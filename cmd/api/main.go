package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mnfirdauss/mkp-cinema-ticketing/internal/auth"
	"github.com/mnfirdauss/mkp-cinema-ticketing/internal/config"
	"github.com/mnfirdauss/mkp-cinema-ticketing/internal/database"
	"github.com/mnfirdauss/mkp-cinema-ticketing/internal/handler"
	"github.com/mnfirdauss/mkp-cinema-ticketing/internal/httpx"
	mw "github.com/mnfirdauss/mkp-cinema-ticketing/internal/middleware"
	"github.com/mnfirdauss/mkp-cinema-ticketing/internal/repository"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("connect database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	tokens := auth.NewTokenService(cfg.JWTSecret, cfg.JWTTTL)
	users := repository.NewUserRepository(db)
	authH := handler.NewAuthHandler(users, tokens, auth.NewLoginLimiter(5, 15*time.Minute))
	showtimeH := handler.NewShowtimeHandler(repository.NewShowtimeRepository(db))

	authenticated := mw.Authenticate(tokens, users.AuthState)
	adminOnly := mw.RequireRole(auth.RoleSuperAdmin, auth.RoleCinemaAdmin)
	protect := func(h http.HandlerFunc, extra ...func(http.Handler) http.Handler) http.Handler {
		return mw.Chain(h, append([]func(http.Handler) http.Handler{authenticated}, extra...)...)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		httpx.OK(w, http.StatusOK, "ok", nil)
	})

	mux.HandleFunc("POST /api/v1/auth/login", authH.Login)
	mux.Handle("GET /api/v1/auth/me", protect(authH.Me))

	mux.Handle("GET /api/v1/showtimes", protect(showtimeH.List))
	mux.Handle("GET /api/v1/showtimes/{id}", protect(showtimeH.Get))
	mux.Handle("POST /api/v1/showtimes", protect(showtimeH.Create, adminOnly))
	mux.Handle("PUT /api/v1/showtimes/{id}", protect(showtimeH.Update, adminOnly))
	mux.Handle("DELETE /api/v1/showtimes/{id}", protect(showtimeH.Delete, adminOnly))

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           mw.Chain(mux, mw.Recover, mw.Logger),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
	}

	go func() {
		slog.Info("server started", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server error", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown", "error", err)
	}
	slog.Info("server stopped")
}
