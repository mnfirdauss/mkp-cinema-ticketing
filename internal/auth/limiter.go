package auth

import (
	"sync"
	"time"
)

// LoginLimiter blocks a key (client IP + email) after too many failed login
// attempts within a window, to slow down password brute forcing.
// It is in-memory; with multiple API replicas it should move to Redis.
type LoginLimiter struct {
	mu       sync.Mutex
	max      int
	window   time.Duration
	failures map[string]*attempts
}

type attempts struct {
	count int
	reset time.Time
}

func NewLoginLimiter(max int, window time.Duration) *LoginLimiter {
	return &LoginLimiter{max: max, window: window, failures: map[string]*attempts{}}
}

// Allowed reports whether key may try to log in, and if not, how long to wait.
func (l *LoginLimiter) Allowed(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	a, ok := l.failures[key]
	if !ok || time.Now().After(a.reset) {
		return true, 0
	}
	if a.count >= l.max {
		return false, time.Until(a.reset)
	}
	return true, 0
}

func (l *LoginLimiter) Fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	if len(l.failures) > 10000 { // drop expired entries so memory stays bounded
		for k, a := range l.failures {
			if now.After(a.reset) {
				delete(l.failures, k)
			}
		}
	}
	a, ok := l.failures[key]
	if !ok || now.After(a.reset) {
		a = &attempts{reset: now.Add(l.window)}
		l.failures[key] = a
	}
	a.count++
}

func (l *LoginLimiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.failures, key)
}
