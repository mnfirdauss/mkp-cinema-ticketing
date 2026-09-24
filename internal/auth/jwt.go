package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	RoleCustomer    = "customer"
	RoleCinemaAdmin = "cinema_admin"
	RoleSuperAdmin  = "super_admin"
)

type Claims struct {
	Role     string `json:"role"`
	CinemaID *int64 `json:"cinema_id,omitempty"`
	jwt.RegisteredClaims
}

// UserID returns the authenticated user's id (JWT "sub").
func (c *Claims) UserID() string { return c.Subject }

type TokenService struct {
	secret []byte
	ttl    time.Duration
}

func NewTokenService(secret string, ttl time.Duration) *TokenService {
	return &TokenService{secret: []byte(secret), ttl: ttl}
}

func (s *TokenService) Generate(userID, role string, cinemaID *int64) (string, time.Time, error) {
	now := time.Now()
	exp := now.Add(s.ttl)
	claims := Claims{
		Role:     role,
		CinemaID: cinemaID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			Issuer:    "mkp-cinema-api",
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
	return token, exp, err
}

func (s *TokenService) Parse(tokenStr string) (*Claims, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		return s.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithIssuer("mkp-cinema-api"))
	if err != nil {
		return nil, err
	}
	if claims.Subject == "" {
		return nil, errors.New("token missing subject")
	}
	return claims, nil
}
