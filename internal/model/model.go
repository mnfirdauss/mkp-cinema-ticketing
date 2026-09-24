package model

import "time"

type User struct {
	ID           string     `json:"id"`
	FullName     string     `json:"full_name"`
	Email        string     `json:"email"`
	Phone        *string    `json:"phone,omitempty"`
	PasswordHash string     `json:"-"`
	Role         string     `json:"role"`
	CinemaID     *int64     `json:"cinema_id,omitempty"`
	IsActive     bool       `json:"is_active"`
	LastLoginAt  *time.Time `json:"last_login_at,omitempty"`
}

type Showtime struct {
	ID           int64     `json:"id"`
	MovieID      int64     `json:"movie_id"`
	MovieTitle   string    `json:"movie_title"`
	StudioID     int64     `json:"studio_id"`
	StudioName   string    `json:"studio_name"`
	CinemaID     int64     `json:"cinema_id"`
	CinemaName   string    `json:"cinema_name"`
	CityName     string    `json:"city_name"`
	StartTime    time.Time `json:"start_time"`
	EndTime      time.Time `json:"end_time"`
	Price        float64   `json:"price"`
	Status       string    `json:"status"`
	CancelReason *string   `json:"cancel_reason,omitempty"`
	TotalSeats   int       `json:"total_seats"`
	Available    int       `json:"available_seats"`
	Sold         int       `json:"sold_seats"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type ShowtimeFilter struct {
	MovieID  int64
	CinemaID int64
	CityID   int64
	Date     *time.Time // local date (Asia/Jakarta)
	Status   string
	Page     int
	Limit    int
}

// ShowtimeInput is the validated payload for create/update.
type ShowtimeInput struct {
	MovieID   int64
	StudioID  int64
	StartTime time.Time
	Price     float64
	Status    string
}
