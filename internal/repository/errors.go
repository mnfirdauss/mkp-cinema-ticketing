package repository

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrNotFound         = errors.New("resource not found")
	ErrOverlap          = errors.New("studio already has another showtime in that time range")
	ErrInvalidReference = errors.New("referenced movie or studio does not exist")
	ErrHasBookings      = errors.New("showtime already has held/sold seats")
)

// translate maps PostgreSQL constraint violations to domain errors.
func translate(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23P01": // exclusion_violation (no_overlapping_showtime)
			return ErrOverlap
		case "23503": // foreign_key_violation
			return ErrInvalidReference
		}
	}
	return err
}
