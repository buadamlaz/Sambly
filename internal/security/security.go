package security

import (
	"net"
	"net/http"
	"regexp"
	"sync"
	"time"
)

var (
	reUsername  = regexp.MustCompile(`^[a-zA-Z0-9_\-\.]{1,32}$`)
	reGroupName = regexp.MustCompile(`^[a-zA-Z0-9_\-]{1,32}$`)
	reShareName = regexp.MustCompile(`^[a-zA-Z0-9_\- ]{1,64}$`)
	reAbsPath   = regexp.MustCompile(`^/[a-zA-Z0-9_\-\./]{1,255}$`)
)

func ValidateUsername(s string) bool  { return reUsername.MatchString(s) }
func ValidateGroupName(s string) bool { return reGroupName.MatchString(s) }
func ValidateShareName(s string) bool { return reShareName.MatchString(s) }

func ValidatePath(s string) bool {
	if !reAbsPath.MatchString(s) {
		return false
	}
	// Reject path traversal
	for i := 0; i < len(s)-1; i++ {
		if s[i] == '.' && s[i+1] == '.' {
			return false
		}
	}
	return true
}

func RealIP(r *http.Request) string {
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		return ip
	}
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	return host
}

func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval' unpkg.com cdn.jsdelivr.net; style-src 'self' 'unsafe-inline'; img-src 'self' data:")
		next.ServeHTTP(w, r)
	})
}

// --- Rate limiter ---

type attempt struct {
	count     int
	blockedAt time.Time
}

type RateLimiter struct {
	mu       sync.Mutex
	attempts map[string]*attempt
	max      int
	window   time.Duration
	block    time.Duration
}

func NewRateLimiter(maxAttempts int, window, blockDuration time.Duration) *RateLimiter {
	return &RateLimiter{
		attempts: make(map[string]*attempt),
		max:      maxAttempts,
		window:   window,
		block:    blockDuration,
	}
}

func (rl *RateLimiter) Allow(ip string) (allowed bool, remaining time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	a, ok := rl.attempts[ip]
	if !ok {
		rl.attempts[ip] = &attempt{count: 1}
		return true, 0
	}

	// Currently blocked
	if !a.blockedAt.IsZero() {
		if time.Since(a.blockedAt) < rl.block {
			return false, rl.block - time.Since(a.blockedAt)
		}
		// Block expired, reset
		delete(rl.attempts, ip)
		rl.attempts[ip] = &attempt{count: 1}
		return true, 0
	}

	a.count++
	if a.count > rl.max {
		a.blockedAt = time.Now()
		return false, rl.block
	}
	return true, 0
}

func (rl *RateLimiter) Reset(ip string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.attempts, ip)
}
