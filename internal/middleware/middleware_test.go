package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mnfirdauss/mkp-cinema-ticketing/internal/auth"
)

func TestAuthenticateAndRequireRole(t *testing.T) {
	tokens := auth.NewTokenService("secret", time.Hour)
	adminToken, _, _ := tokens.Generate("a", auth.RoleSuperAdmin, nil)
	customerToken, _, _ := tokens.Generate("c", auth.RoleCustomer, nil)

	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })
	h := Chain(ok, Authenticate(tokens, nil), RequireRole(auth.RoleSuperAdmin, auth.RoleCinemaAdmin))

	tests := []struct {
		name   string
		header string
		want   int
	}{
		{"no header", "", http.StatusUnauthorized},
		{"not bearer", "Basic abc", http.StatusUnauthorized},
		{"garbage token", "Bearer abc", http.StatusUnauthorized},
		{"customer forbidden", "Bearer " + customerToken, http.StatusForbidden},
		{"admin allowed", "Bearer " + adminToken, http.StatusNoContent},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/showtimes", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tt.want {
				t.Fatalf("got %d, want %d", rec.Code, tt.want)
			}
		})
	}
}
