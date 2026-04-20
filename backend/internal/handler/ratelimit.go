package handler

import (
	"bytes"
	"container/list"
	"encoding/json"
	"io"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const (
	defaultRateLimitMaxEntries      = 4096
	defaultRateLimitIdleTTL         = 30 * time.Minute
	defaultRateLimitCleanupInterval = 5 * time.Minute
)

type Middleware func(http.HandlerFunc) http.HandlerFunc

type RequestKeyFunc func(*http.Request) string

type rateLimitEntry struct {
	key      string
	limiter  *rate.Limiter
	lastSeen time.Time
	element  *list.Element
}

type rateLimitStore struct {
	now             func() time.Time
	limit           rate.Limit
	burst           int
	maxEntries      int
	idleTTL         time.Duration
	cleanupInterval time.Duration

	mu          sync.Mutex
	entries     sync.Map
	lru         *list.List
	lastCleanup time.Time
}

func Chain(next http.HandlerFunc, middlewares ...Middleware) http.HandlerFunc {
	for i := len(middlewares) - 1; i >= 0; i-- {
		next = middlewares[i](next)
	}

	return next
}

func NewRateLimitMiddleware(keyFn RequestKeyFunc, limit rate.Limit, burst int) Middleware {
	return newRateLimitMiddleware(keyFn, limit, burst, time.Now)
}

func ClientIPRateLimitKey(r *http.Request) string {
	return clientIPRateLimitKey(r)
}

func LoginEmailRateLimitKey(r *http.Request) string {
	return loginEmailRateLimitKey(r)
}

func newRateLimitMiddleware(
	keyFn RequestKeyFunc,
	limit rate.Limit,
	burst int,
	now func() time.Time,
) Middleware {
	store := newRateLimitStore(limit, burst, now)

	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			key := keyFn(r)
			if key == "" {
				next(w, r)
				return
			}

			retryAfter, allowed := store.allow(key)
			if !allowed {
				writeTooManyRequests(w, retryAfter)
				return
			}

			next(w, r)
		}
	}
}

func newRateLimitStore(limit rate.Limit, burst int, now func() time.Time) *rateLimitStore {
	return &rateLimitStore{
		now:             now,
		limit:           limit,
		burst:           burst,
		maxEntries:      defaultRateLimitMaxEntries,
		idleTTL:         defaultRateLimitIdleTTL,
		cleanupInterval: defaultRateLimitCleanupInterval,
		lru:             list.New(),
	}
}

func (s *rateLimitStore) allow(key string) (time.Duration, bool) {
	now := s.now()

	s.mu.Lock()
	defer s.mu.Unlock()

	s.cleanupLocked(now)

	entry := s.loadOrCreateLocked(key, now)
	entry.lastSeen = now
	s.lru.MoveToFront(entry.element)

	reservation := entry.limiter.ReserveN(now, 1)
	if !reservation.OK() {
		return time.Second, false
	}

	delay := reservation.DelayFrom(now)
	if delay > 0 {
		reservation.CancelAt(now)
		return delay, false
	}

	return 0, true
}

func (s *rateLimitStore) loadOrCreateLocked(key string, now time.Time) *rateLimitEntry {
	if value, ok := s.entries.Load(key); ok {
		return value.(*rateLimitEntry)
	}

	entry := &rateLimitEntry{
		key:      key,
		limiter:  rate.NewLimiter(s.limit, s.burst),
		lastSeen: now,
	}
	entry.element = s.lru.PushFront(entry)
	s.entries.Store(key, entry)
	s.evictOverflowLocked()

	return entry
}

func (s *rateLimitStore) evictOverflowLocked() {
	for s.lru.Len() > s.maxEntries {
		oldest := s.lru.Back()
		if oldest == nil {
			return
		}

		entry := oldest.Value.(*rateLimitEntry)
		s.removeLocked(entry)
	}
}

func (s *rateLimitStore) cleanupLocked(now time.Time) {
	if !s.lastCleanup.IsZero() && now.Sub(s.lastCleanup) < s.cleanupInterval {
		return
	}

	for element := s.lru.Back(); element != nil; {
		prev := element.Prev()
		entry := element.Value.(*rateLimitEntry)
		idle := now.Sub(entry.lastSeen)
		if idle > s.idleTTL && idle >= s.fullRefillDuration() {
			s.removeLocked(entry)
		}
		element = prev
	}

	s.lastCleanup = now
}

func (s *rateLimitStore) removeLocked(entry *rateLimitEntry) {
	if entry.element != nil {
		s.lru.Remove(entry.element)
		entry.element = nil
	}
	s.entries.Delete(entry.key)
}

func (s *rateLimitStore) fullRefillDuration() time.Duration {
	if s.limit <= 0 || s.burst <= 0 {
		return 0
	}

	seconds := float64(s.burst) / float64(s.limit)
	return time.Duration(seconds * float64(time.Second))
}

func clientIPRateLimitKey(r *http.Request) string {
	if forwarded := forwardedClientIP(r.Header.Get("X-Forwarded-For")); forwarded != "" {
		return forwarded
	}

	return remoteAddrIP(r.RemoteAddr)
}

func loginEmailRateLimitKey(r *http.Request) string {
	body, err := readAndRestoreRequestBody(r)
	if err != nil || len(body) == 0 {
		return ""
	}

	var payload struct {
		Email string `json:"email"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return ""
	}

	return strings.ToLower(strings.TrimSpace(payload.Email))
}

func readAndRestoreRequestBody(r *http.Request) ([]byte, error) {
	if r.Body == nil {
		return nil, nil
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, maxJSONBodyBytes+1))
	if err != nil {
		return nil, err
	}
	r.Body = io.NopCloser(bytes.NewReader(body))

	if len(body) > maxJSONBodyBytes {
		return nil, io.ErrUnexpectedEOF
	}

	return body, nil
}

func forwardedClientIP(headerValue string) string {
	if headerValue == "" {
		return ""
	}

	firstHop := strings.TrimSpace(strings.Split(headerValue, ",")[0])
	if net.ParseIP(firstHop) == nil {
		return ""
	}

	return firstHop
}

func remoteAddrIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err == nil {
		return host
	}

	return strings.TrimSpace(remoteAddr)
}

func writeTooManyRequests(w http.ResponseWriter, retryAfter time.Duration) {
	seconds := int(math.Ceil(retryAfter.Seconds()))
	if seconds < 1 {
		seconds = 1
	}

	w.Header().Set("Retry-After", strconv.Itoa(seconds))
	writeError(w, http.StatusTooManyRequests, "TOO_MANY_REQUESTS", "Too many requests. Please try again in a moment.")
}
