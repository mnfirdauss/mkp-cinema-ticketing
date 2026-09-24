package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/mnfirdauss/mkp-cinema-ticketing/internal/auth"
	"github.com/mnfirdauss/mkp-cinema-ticketing/internal/httpx"
	"github.com/mnfirdauss/mkp-cinema-ticketing/internal/middleware"
	"github.com/mnfirdauss/mkp-cinema-ticketing/internal/model"
	"github.com/mnfirdauss/mkp-cinema-ticketing/internal/repository"
)

// Statuses an admin may set through CRUD. "cancelled" is intentionally
// excluded: cancelling a showtime must go through the refund flow.
var editableStatuses = []string{"scheduled", "open", "closed", "finished"}

const (
	maxPrice = 10_000_000 // well within NUMERIC(12,2)
	maxPage  = 100_000
)

type ShowtimeHandler struct {
	repo *repository.ShowtimeRepository
}

func NewShowtimeHandler(repo *repository.ShowtimeRepository) *ShowtimeHandler {
	return &ShowtimeHandler{repo: repo}
}

type showtimeRequest struct {
	MovieID   int64   `json:"movie_id"`
	StudioID  int64   `json:"studio_id"`
	StartTime string  `json:"start_time"` // RFC3339, e.g. 2025-10-01T19:00:00+07:00
	Price     float64 `json:"price"`
	Status    string  `json:"status"`
}

type pageMeta struct {
	Page       int `json:"page"`
	Limit      int `json:"limit"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

// List godoc: GET /api/v1/showtimes?movie_id=&cinema_id=&city_id=&date=YYYY-MM-DD&status=&page=&limit=
func (h *ShowtimeHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	errs := map[string]string{}
	f := model.ShowtimeFilter{
		MovieID:  parseInt(q.Get("movie_id"), "movie_id", errs),
		CinemaID: parseInt(q.Get("cinema_id"), "cinema_id", errs),
		CityID:   parseInt(q.Get("city_id"), "city_id", errs),
		Status:   q.Get("status"),
		Page:     int(parseInt(q.Get("page"), "page", errs)),
		Limit:    int(parseInt(q.Get("limit"), "limit", errs)),
	}
	if d := q.Get("date"); d != "" {
		t, err := time.Parse("2006-01-02", d)
		if err != nil {
			errs["date"] = "must be in YYYY-MM-DD format"
		}
		f.Date = &t
	}
	if len(errs) > 0 {
		httpx.ValidationError(w, errs)
		return
	}
	if f.Page < 1 {
		f.Page = 1
	}
	if f.Page > maxPage {
		f.Page = maxPage
	}
	if f.Limit < 1 || f.Limit > 100 {
		f.Limit = 20
	}

	items, total, err := h.repo.List(r.Context(), f)
	if err != nil {
		internalError(w, "list showtimes", err)
		return
	}
	httpx.JSON(w, http.StatusOK, httpx.Envelope{
		Success: true,
		Data:    items,
		Meta:    pageMeta{Page: f.Page, Limit: f.Limit, Total: total, TotalPages: (total + f.Limit - 1) / f.Limit},
	})
}

// Get godoc: GET /api/v1/showtimes/{id}
func (h *ShowtimeHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	s, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		h.writeRepoError(w, "get showtime", err)
		return
	}
	httpx.OK(w, http.StatusOK, "", s)
}

// Create godoc: POST /api/v1/showtimes
func (h *ShowtimeHandler) Create(w http.ResponseWriter, r *http.Request) {
	in, ok := decodeShowtime(w, r, true)
	if !ok {
		return
	}
	if !h.authorizeStudio(w, r, in.StudioID) {
		return
	}

	s, err := h.repo.Create(r.Context(), in, middleware.ClaimsFrom(r.Context()).UserID())
	if err != nil {
		h.writeRepoError(w, "create showtime", err)
		return
	}
	httpx.OK(w, http.StatusCreated, "showtime created", s)
}

// Update godoc: PUT /api/v1/showtimes/{id}
func (h *ShowtimeHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	current, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		h.writeRepoError(w, "get showtime", err)
		return
	}
	if !canManageCinema(r, current.CinemaID) {
		httpx.Error(w, http.StatusForbidden, "you can only manage showtimes of your own cinema")
		return
	}

	in, ok := decodeShowtime(w, r, current.StartTime.After(time.Now()))
	if !ok {
		return
	}
	if in.StudioID != current.StudioID && !h.authorizeStudio(w, r, in.StudioID) {
		return
	}

	s, err := h.repo.Update(r.Context(), id, in)
	if err != nil {
		h.writeRepoError(w, "update showtime", err)
		return
	}
	httpx.OK(w, http.StatusOK, "showtime updated", s)
}

// Delete godoc: DELETE /api/v1/showtimes/{id}
func (h *ShowtimeHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	current, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		h.writeRepoError(w, "get showtime", err)
		return
	}
	if !canManageCinema(r, current.CinemaID) {
		httpx.Error(w, http.StatusForbidden, "you can only manage showtimes of your own cinema")
		return
	}
	if err := h.repo.Delete(r.Context(), id); err != nil {
		h.writeRepoError(w, "delete showtime", err)
		return
	}
	httpx.OK(w, http.StatusOK, "showtime deleted", nil)
}

// authorizeStudio checks the studio exists and belongs to the admin's cinema.
func (h *ShowtimeHandler) authorizeStudio(w http.ResponseWriter, r *http.Request, studioID int64) bool {
	cinemaID, err := h.repo.StudioCinemaID(r.Context(), studioID)
	if err != nil {
		h.writeRepoError(w, "get studio", err)
		return false
	}
	if !canManageCinema(r, cinemaID) {
		httpx.Error(w, http.StatusForbidden, "you can only manage showtimes of your own cinema")
		return false
	}
	return true
}

func (h *ShowtimeHandler) writeRepoError(w http.ResponseWriter, op string, err error) {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		httpx.Error(w, http.StatusNotFound, "showtime not found")
	case errors.Is(err, repository.ErrInvalidReference):
		httpx.ValidationError(w, map[string]string{"movie_id/studio_id": err.Error()})
	case errors.Is(err, repository.ErrOverlap):
		httpx.Error(w, http.StatusConflict, err.Error())
	case errors.Is(err, repository.ErrHasBookings):
		httpx.Error(w, http.StatusConflict, err.Error()+"; movie/studio/start_time cannot be changed and the showtime cannot be deleted, use the cancellation & refund flow instead")
	default:
		internalError(w, op, err)
	}
}

func canManageCinema(r *http.Request, cinemaID int64) bool {
	c := middleware.ClaimsFrom(r.Context())
	switch c.Role {
	case auth.RoleSuperAdmin:
		return true
	case auth.RoleCinemaAdmin:
		return c.CinemaID != nil && *c.CinemaID == cinemaID
	}
	return false
}

// decodeShowtime parses and validates the request body. requireFuture is
// false when updating a showtime that already started, so price/status can
// still be adjusted.
func decodeShowtime(w http.ResponseWriter, r *http.Request, requireFuture bool) (model.ShowtimeInput, bool) {
	var req showtimeRequest
	if err := httpx.Decode(w, r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return model.ShowtimeInput{}, false
	}

	errs := map[string]string{}
	if req.MovieID <= 0 {
		errs["movie_id"] = "is required"
	}
	if req.StudioID <= 0 {
		errs["studio_id"] = "is required"
	}
	start, err := time.Parse(time.RFC3339, req.StartTime)
	switch {
	case err != nil:
		errs["start_time"] = "must be RFC3339, e.g. 2025-10-01T19:00:00+07:00"
	case requireFuture && start.Before(time.Now()):
		errs["start_time"] = "must be in the future"
	}
	if req.Price < 0 || req.Price > maxPrice {
		errs["price"] = "must be between 0 and 10000000"
	}
	if req.Status == "" {
		req.Status = "scheduled"
	}
	if !slices.Contains(editableStatuses, req.Status) {
		errs["status"] = "must be one of scheduled, open, closed, finished"
	}
	if len(errs) > 0 {
		httpx.ValidationError(w, errs)
		return model.ShowtimeInput{}, false
	}

	return model.ShowtimeInput{
		MovieID:   req.MovieID,
		StudioID:  req.StudioID,
		StartTime: start,
		Price:     req.Price,
		Status:    req.Status,
	}, true
}

func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		httpx.Error(w, http.StatusBadRequest, "invalid id")
		return 0, false
	}
	return id, true
}

func parseInt(v, field string, errs map[string]string) int64 {
	if v == "" {
		return 0
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		errs[field] = "must be a number"
	}
	return n
}

func internalError(w http.ResponseWriter, op string, err error) {
	slog.Error(op, "error", err)
	httpx.Error(w, http.StatusInternalServerError, "internal server error")
}
