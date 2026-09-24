package auth

import (
	"testing"
	"time"
)

func TestTokenRoundTrip(t *testing.T) {
	svc := NewTokenService("secret", time.Hour)
	cinemaID := int64(7)

	token, _, err := svc.Generate("user-1", RoleCinemaAdmin, &cinemaID)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := svc.Parse(token)
	if err != nil {
		t.Fatal(err)
	}
	if claims.UserID() != "user-1" || claims.Role != RoleCinemaAdmin || *claims.CinemaID != 7 {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}

func TestParseRejectsWrongSecretAndExpired(t *testing.T) {
	token, _, _ := NewTokenService("secret", time.Hour).Generate("u", RoleCustomer, nil)
	if _, err := NewTokenService("other", time.Hour).Parse(token); err == nil {
		t.Fatal("expected error for token signed with a different secret")
	}

	expired, _, _ := NewTokenService("secret", -time.Minute).Generate("u", RoleCustomer, nil)
	if _, err := NewTokenService("secret", time.Hour).Parse(expired); err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestPassword(t *testing.T) {
	hash, err := HashPassword("password123")
	if err != nil {
		t.Fatal(err)
	}
	if !CheckPassword(hash, "password123") || CheckPassword(hash, "wrong") {
		t.Fatal("password check mismatch")
	}
}
