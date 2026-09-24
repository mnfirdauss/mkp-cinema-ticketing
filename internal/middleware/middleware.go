package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/mnfirdauss/mkp-cinema-ticketing/internal/auth"
	"github.com/mnfirdauss/mkp-cinema-ticketing/internal/httpx"
)

type ctxKey struct{}

// ClaimsFrom returns the JWT claims stored by Authenticate.
func ClaimsFrom(ctx context.Context) *auth.Claims {
	c, _ := ctx.Value(ctxKey{}).(*auth.Claims)
	return c
}

// UserLookup returns the current role, cinema and active flag of a user, so
// that deactivating a user or changing their role takes effect immediately
// instead of waiting for the JWT to expire. It returns an error if the user
// does not exist.
type UserLookup func(ctx context.Context, userID string) (role string, cinemaID *int64, active bool, err error)

// Authenticate validates the "Authorization: Bearer <token>" header. When
// lookup is non-nil the user is re-checked against the database.
func Authenticate(tokens *auth.TokenService, lookup UserLookup) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			token, ok := strings.CutPrefix(header, "Bearer ")
			if !ok || token == "" {
				httpx.Error(w, http.StatusUnauthorized, "missing or invalid Authorization header")
				return
			}
			claims, err := tokens.Parse(token)
			if err != nil {
				httpx.Error(w, http.StatusUnauthorized, "invalid or expired token")
				return
			}
			if lookup != nil {
				role, cinemaID, active, err := lookup(r.Context(), claims.UserID())
				if err != nil || !active {
					httpx.Error(w, http.StatusUnauthorized, "invalid or expired token")
					return
				}
				claims.Role, claims.CinemaID = role, cinemaID
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, claims)))
		})
	}
}

// RequireRole must be used after Authenticate.
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := ClaimsFrom(r.Context())
			if claims == nil || !slices.Contains(roles, claims.Role) {
				httpx.Error(w, http.StatusForbidden, "you do not have permission to access this resource")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		slog.Info("request", "method", r.Method, "path", r.URL.Path, "status", rec.status, "duration", time.Since(start))
	})
}

func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				slog.Error("panic recovered", "error", err, "path", r.URL.Path)
				httpx.Error(w, http.StatusInternalServerError, "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func Chain(h http.Handler, mws ...func(http.Handler) http.Handler) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}
