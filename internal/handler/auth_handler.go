package handler

import (
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/mail"
	"strconv"
	"strings"
	"time"

	"github.com/mnfirdauss/mkp-cinema-ticketing/internal/auth"
	"github.com/mnfirdauss/mkp-cinema-ticketing/internal/httpx"
	"github.com/mnfirdauss/mkp-cinema-ticketing/internal/middleware"
	"github.com/mnfirdauss/mkp-cinema-ticketing/internal/model"
	"github.com/mnfirdauss/mkp-cinema-ticketing/internal/repository"
)

type AuthHandler struct {
	users   *repository.UserRepository
	tokens  *auth.TokenService
	limiter *auth.LoginLimiter
}

func NewAuthHandler(users *repository.UserRepository, tokens *auth.TokenService, limiter *auth.LoginLimiter) *AuthHandler {
	return &AuthHandler{users: users, tokens: tokens, limiter: limiter}
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	AccessToken string      `json:"access_token"`
	TokenType   string      `json:"token_type"`
	ExpiresAt   time.Time   `json:"expires_at"`
	User        *model.User `json:"user"`
}

// Login godoc: POST /api/v1/auth/login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := httpx.Decode(w, r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}

	req.Email = strings.TrimSpace(req.Email)
	errs := map[string]string{}
	if _, err := mail.ParseAddress(req.Email); err != nil {
		errs["email"] = "must be a valid email address"
	}
	if req.Password == "" {
		errs["password"] = "is required"
	}
	if len(errs) > 0 {
		httpx.ValidationError(w, errs)
		return
	}

	limitKey := clientIP(r) + "|" + strings.ToLower(req.Email)
	if ok, wait := h.limiter.Allowed(limitKey); !ok {
		w.Header().Set("Retry-After", strconv.Itoa(int(wait.Seconds())+1))
		httpx.Error(w, http.StatusTooManyRequests, "too many failed login attempts, please try again later")
		return
	}

	user, err := h.users.FindByEmail(r.Context(), req.Email)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		slog.Error("find user", "error", err)
		httpx.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}
	// Same message and (via the dummy bcrypt compare) same response time for
	// unknown email and wrong password, to avoid user enumeration.
	if user == nil {
		auth.CheckPasswordDummy(req.Password)
	}
	if user == nil || !auth.CheckPassword(user.PasswordHash, req.Password) {
		h.limiter.Fail(limitKey)
		httpx.Error(w, http.StatusUnauthorized, "invalid email or password")
		return
	}
	h.limiter.Reset(limitKey)
	if !user.IsActive {
		httpx.Error(w, http.StatusForbidden, "account is disabled")
		return
	}

	token, exp, err := h.tokens.Generate(user.ID, user.Role, user.CinemaID)
	if err != nil {
		slog.Error("generate token", "error", err)
		httpx.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if err := h.users.TouchLastLogin(r.Context(), user.ID); err != nil {
		slog.Warn("update last login", "error", err)
	}

	httpx.OK(w, http.StatusOK, "login successful", loginResponse{
		AccessToken: token,
		TokenType:   "Bearer",
		ExpiresAt:   exp,
		User:        user,
	})
}

// clientIP uses the TCP peer address. X-Forwarded-For is deliberately not
// trusted because clients can spoof it; behind a trusted proxy, configure the
// proxy to set RemoteAddr (or add explicit trusted-proxy handling).
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// Me godoc: GET /api/v1/auth/me
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFrom(r.Context())
	user, err := h.users.FindByID(r.Context(), claims.UserID())
	if errors.Is(err, repository.ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, "user not found")
		return
	}
	if err != nil {
		slog.Error("find user", "error", err)
		httpx.Error(w, http.StatusInternalServerError, "internal server error")
		return
	}
	httpx.OK(w, http.StatusOK, "", user)
}
