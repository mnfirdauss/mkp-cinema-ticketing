package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDecodeShowtime(t *testing.T) {
	future := time.Now().Add(48 * time.Hour).Format(time.RFC3339)
	past := time.Now().Add(-48 * time.Hour).Format(time.RFC3339)

	tests := []struct {
		name          string
		body          string
		requireFuture bool
		wantOK        bool
		wantStatus    int
	}{
		{"valid, default status", `{"movie_id":1,"studio_id":1,"start_time":"` + future + `","price":50000}`, true, true, 0},
		{"past start rejected on create", `{"movie_id":1,"studio_id":1,"start_time":"` + past + `","price":1}`, true, false, http.StatusUnprocessableEntity},
		{"past start allowed on running showtime", `{"movie_id":1,"studio_id":1,"start_time":"` + past + `","price":1}`, false, true, 0},
		{"cancelled not allowed via CRUD", `{"movie_id":1,"studio_id":1,"start_time":"` + future + `","price":1,"status":"cancelled"}`, true, false, http.StatusUnprocessableEntity},
		{"bad time format", `{"movie_id":1,"studio_id":1,"start_time":"2025-10-01 19:00","price":1}`, true, false, http.StatusUnprocessableEntity},
		{"unknown field", `{"movie_id":1,"foo":1}`, true, false, http.StatusBadRequest},
		{"empty body", ``, true, false, http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tt.body))
			rec := httptest.NewRecorder()
			in, ok := decodeShowtime(rec, req, tt.requireFuture)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v (body: %s)", ok, tt.wantOK, rec.Body.String())
			}
			if !ok && rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if ok && in.Status != "scheduled" {
				t.Fatalf("default status = %q, want scheduled", in.Status)
			}
		})
	}
}
