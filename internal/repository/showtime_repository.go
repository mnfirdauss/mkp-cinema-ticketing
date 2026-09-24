package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mnfirdauss/mkp-cinema-ticketing/internal/model"
)

// CleaningBufferMinutes is added after the movie ends so the studio can be
// cleaned before the next showtime.
const CleaningBufferMinutes = 15

type ShowtimeRepository struct {
	db *pgxpool.Pool
}

func NewShowtimeRepository(db *pgxpool.Pool) *ShowtimeRepository {
	return &ShowtimeRepository{db: db}
}

const showtimeSelect = `
SELECT s.id, s.movie_id, m.title, s.studio_id, st.name, c.id, c.name, ci.name,
       s.start_time, s.end_time, s.price::float8, s.status::text, s.cancel_reason,
       COALESCE(a.total, 0), COALESCE(a.available, 0), COALESCE(a.sold, 0),
       s.created_at, s.updated_at
FROM showtimes s
JOIN movies  m  ON m.id  = s.movie_id
JOIN studios st ON st.id = s.studio_id
JOIN cinemas c  ON c.id  = st.cinema_id
JOIN cities  ci ON ci.id = c.city_id
LEFT JOIN LATERAL (
    SELECT COUNT(*) AS total,
           COUNT(*) FILTER (WHERE ss.status = 'available') AS available,
           COUNT(*) FILTER (WHERE ss.status = 'sold')      AS sold
    FROM showtime_seats ss WHERE ss.showtime_id = s.id
) a ON TRUE
WHERE s.deleted_at IS NULL`

func scanShowtime(row pgx.Row) (*model.Showtime, error) {
	var s model.Showtime
	err := row.Scan(
		&s.ID, &s.MovieID, &s.MovieTitle, &s.StudioID, &s.StudioName, &s.CinemaID, &s.CinemaName, &s.CityName,
		&s.StartTime, &s.EndTime, &s.Price, &s.Status, &s.CancelReason,
		&s.TotalSeats, &s.Available, &s.Sold, &s.CreatedAt, &s.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &s, err
}

func (r *ShowtimeRepository) List(ctx context.Context, f model.ShowtimeFilter) ([]model.Showtime, int, error) {
	var (
		where []string
		args  []any
	)
	add := func(cond string, v any) {
		args = append(args, v)
		where = append(where, fmt.Sprintf(cond, len(args)))
	}
	if f.MovieID > 0 {
		add("s.movie_id = $%d", f.MovieID)
	}
	if f.CinemaID > 0 {
		add("c.id = $%d", f.CinemaID)
	}
	if f.CityID > 0 {
		add("ci.id = $%d", f.CityID)
	}
	if f.Status != "" {
		add("s.status::text = $%d", f.Status)
	}
	if f.Date != nil {
		// compare against the cinema's local date (WIB/WITA/WIT)
		add("(s.start_time AT TIME ZONE ci.timezone)::date = $%d::date", f.Date.Format("2006-01-02"))
	}

	query := showtimeSelect
	if len(where) > 0 {
		query += " AND " + strings.Join(where, " AND ")
	}

	var total int
	if err := r.db.QueryRow(ctx, "SELECT COUNT(*) FROM ("+query+") q", args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, f.Limit, (f.Page-1)*f.Limit)
	query += fmt.Sprintf(" ORDER BY s.start_time, s.id LIMIT $%d OFFSET $%d", len(args)-1, len(args))

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]model.Showtime, 0, f.Limit)
	for rows.Next() {
		s, err := scanShowtime(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, *s)
	}
	return items, total, rows.Err()
}

func (r *ShowtimeRepository) GetByID(ctx context.Context, id int64) (*model.Showtime, error) {
	return scanShowtime(r.db.QueryRow(ctx, showtimeSelect+" AND s.id = $1", id))
}

// StudioCinemaID returns the cinema that owns the studio (used for authorization).
func (r *ShowtimeRepository) StudioCinemaID(ctx context.Context, studioID int64) (int64, error) {
	var cinemaID int64
	err := r.db.QueryRow(ctx, `SELECT cinema_id FROM studios WHERE id = $1 AND is_active`, studioID).Scan(&cinemaID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrInvalidReference
	}
	return cinemaID, err
}

// Create inserts the showtime and generates its per-seat inventory
// (showtime_seats) in a single transaction.
func (r *ShowtimeRepository) Create(ctx context.Context, in model.ShowtimeInput, createdBy string) (*model.Showtime, error) {
	var id int64
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `
			INSERT INTO showtimes (movie_id, studio_id, start_time, end_time, price, status, created_by)
			SELECT m.id, $2, $3::timestamptz, $3::timestamptz + make_interval(mins => m.duration_minutes + $6), $4, $5::showtime_status, $7
			FROM movies m WHERE m.id = $1 AND m.is_active
			RETURNING id`,
			in.MovieID, in.StudioID, in.StartTime, in.Price, in.Status, CleaningBufferMinutes, createdBy,
		).Scan(&id)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrInvalidReference
		}
		if err != nil {
			return translate(err)
		}
		return generateSeats(ctx, tx, id, in.StudioID)
	})
	if err != nil {
		return nil, err
	}
	return r.GetByID(ctx, id)
}

// Update changes a showtime. Movie, studio and start time are locked once a
// seat has been held or sold, because customers already paid for them; the
// cinema must use the cancellation/refund flow instead.
func (r *ShowtimeRepository) Update(ctx context.Context, id int64, in model.ShowtimeInput) (*model.Showtime, error) {
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		cur, booked, err := lockShowtime(ctx, tx, id)
		if err != nil {
			return err
		}
		scheduleChanged := cur.MovieID != in.MovieID || cur.StudioID != in.StudioID || !cur.StartTime.Equal(in.StartTime)
		if booked > 0 && scheduleChanged {
			return ErrHasBookings
		}

		tag, err := tx.Exec(ctx, `
			UPDATE showtimes s
			SET movie_id = m.id, studio_id = $3, start_time = $4::timestamptz,
			    end_time = $4::timestamptz + make_interval(mins => m.duration_minutes + $7),
			    price = $5, status = $6::showtime_status, updated_at = now()
			FROM movies m
			WHERE s.id = $1 AND m.id = $2 AND m.is_active`,
			id, in.MovieID, in.StudioID, in.StartTime, in.Price, in.Status, CleaningBufferMinutes,
		)
		if err != nil {
			return translate(err)
		}
		if tag.RowsAffected() == 0 {
			return ErrInvalidReference
		}

		if cur.StudioID != in.StudioID {
			if _, err := tx.Exec(ctx, `DELETE FROM showtime_seats WHERE showtime_id = $1`, id); err != nil {
				return err
			}
			return generateSeats(ctx, tx, id, in.StudioID)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return r.GetByID(ctx, id)
}

// Delete soft-deletes a showtime that has no held/sold seats.
func (r *ShowtimeRepository) Delete(ctx context.Context, id int64) error {
	return pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		_, booked, err := lockShowtime(ctx, tx, id)
		if err != nil {
			return err
		}
		if booked > 0 {
			return ErrHasBookings
		}
		_, err = tx.Exec(ctx, `UPDATE showtimes SET deleted_at = now(), updated_at = now() WHERE id = $1`, id)
		return err
	})
}

// lockShowtime takes a row lock on the showtime and returns how many seats are
// currently held or sold.
func lockShowtime(ctx context.Context, tx pgx.Tx, id int64) (*model.Showtime, int, error) {
	var s model.Showtime
	err := tx.QueryRow(ctx, `
		SELECT id, movie_id, studio_id, start_time FROM showtimes
		WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`, id,
	).Scan(&s.ID, &s.MovieID, &s.StudioID, &s.StartTime)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, 0, ErrNotFound
	}
	if err != nil {
		return nil, 0, err
	}

	var booked int
	err = tx.QueryRow(ctx, `
		SELECT COUNT(*) FROM showtime_seats
		WHERE showtime_id = $1 AND status IN ('held', 'sold')`, id,
	).Scan(&booked)
	return &s, booked, err
}

func generateSeats(ctx context.Context, tx pgx.Tx, showtimeID, studioID int64) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO showtime_seats (showtime_id, seat_id)
		SELECT $1, id FROM seats WHERE studio_id = $2 AND is_active`,
		showtimeID, studioID,
	)
	return err
}
