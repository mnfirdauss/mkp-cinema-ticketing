package auth

import (
	"testing"
	"time"
)

func TestLoginLimiter(t *testing.T) {
	l := NewLoginLimiter(3, time.Minute)
	for i := 0; i < 3; i++ {
		if ok, _ := l.Allowed("k"); !ok {
			t.Fatalf("attempt %d should be allowed", i+1)
		}
		l.Fail("k")
	}
	if ok, wait := l.Allowed("k"); ok || wait <= 0 {
		t.Fatal("4th attempt should be blocked")
	}
	if ok, _ := l.Allowed("other"); !ok {
		t.Fatal("other key must not be affected")
	}
	l.Reset("k")
	if ok, _ := l.Allowed("k"); !ok {
		t.Fatal("reset should unblock")
	}
}
