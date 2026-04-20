package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"golang.org/x/time/rate"
)

type fakeRateLimitClock struct {
	nowTime time.Time
}

func newFakeRateLimitClock() *fakeRateLimitClock {
	return &fakeRateLimitClock{nowTime: time.Unix(1_700_000_000, 0)}
}

func (c *fakeRateLimitClock) now() time.Time {
	return c.nowTime
}

func (c *fakeRateLimitClock) advance(d time.Duration) {
	c.nowTime = c.nowTime.Add(d)
}

func TestRateLimitMiddlewareAllowsBurstThenRejects(t *testing.T) {
	t.Parallel()

	clock := newFakeRateLimitClock()
	var allowed int
	handler := newRateLimitMiddleware(clientIPRateLimitKey, rate.Every(time.Minute/5), 5, clock.now)(
		func(w http.ResponseWriter, r *http.Request) {
			allowed++
			w.WriteHeader(http.StatusNoContent)
		},
	)

	for i := 0; i < 5; i++ {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
		req.RemoteAddr = "10.0.0.1:1234"

		handler(recorder, req)

		if recorder.Code != http.StatusNoContent {
			t.Fatalf("request %d: expected status 204, got %d", i+1, recorder.Code)
		}
	}

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
	req.RemoteAddr = "10.0.0.1:1234"

	handler(recorder, req)

	if allowed != 5 {
		t.Fatalf("expected 5 allowed requests, got %d", allowed)
	}
	assertErrorResponse(t, recorder, http.StatusTooManyRequests, "TOO_MANY_REQUESTS")
	if retryAfter := recorder.Header().Get("Retry-After"); retryAfter != "12" {
		t.Fatalf("expected Retry-After 12, got %q", retryAfter)
	}
}

func TestRateLimitMiddlewareRecoversAfterWindowPasses(t *testing.T) {
	t.Parallel()

	clock := newFakeRateLimitClock()
	handler := newRateLimitMiddleware(clientIPRateLimitKey, rate.Every(time.Minute/2), 2, clock.now)(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		},
	)

	for i := 0; i < 2; i++ {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		handler(recorder, req)
	}

	deniedRecorder := httptest.NewRecorder()
	deniedRequest := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
	deniedRequest.RemoteAddr = "10.0.0.1:1234"
	handler(deniedRecorder, deniedRequest)
	assertErrorResponse(t, deniedRecorder, http.StatusTooManyRequests, "TOO_MANY_REQUESTS")

	clock.advance(time.Minute + time.Second)

	allowedRecorder := httptest.NewRecorder()
	allowedRequest := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
	allowedRequest.RemoteAddr = "10.0.0.1:1234"
	handler(allowedRecorder, allowedRequest)

	if allowedRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected status 204 after refill, got %d", allowedRecorder.Code)
	}
}

func TestRateLimitMiddlewareIsolatesKeys(t *testing.T) {
	t.Parallel()

	clock := newFakeRateLimitClock()
	handler := newRateLimitMiddleware(clientIPRateLimitKey, rate.Every(time.Minute/2), 2, clock.now)(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		},
	)

	for i := 0; i < 2; i++ {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		handler(recorder, req)
	}

	deniedRecorder := httptest.NewRecorder()
	deniedRequest := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
	deniedRequest.RemoteAddr = "10.0.0.1:1234"
	handler(deniedRecorder, deniedRequest)
	assertErrorResponse(t, deniedRecorder, http.StatusTooManyRequests, "TOO_MANY_REQUESTS")

	allowedRecorder := httptest.NewRecorder()
	allowedRequest := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
	allowedRequest.RemoteAddr = "10.0.0.2:9999"
	handler(allowedRecorder, allowedRequest)

	if allowedRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected second IP to remain allowed, got %d", allowedRecorder.Code)
	}
}

func TestRateLimitMiddlewareCombinesIPAndEmailLimits(t *testing.T) {
	t.Parallel()

	clock := newFakeRateLimitClock()
	var allowed int
	handler := newRateLimitMiddleware(loginEmailRateLimitKey, rate.Every(15*time.Minute/2), 2, clock.now)(
		newRateLimitMiddleware(clientIPRateLimitKey, rate.Every(time.Minute/5), 5, clock.now)(
			func(w http.ResponseWriter, r *http.Request) {
				allowed++
				w.WriteHeader(http.StatusNoContent)
			},
		),
	)

	for _, remoteAddr := range []string{"10.0.0.1:1000", "10.0.0.2:1000"} {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"email":"worker@example.com","password":"pa55word"}`))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = remoteAddr
		handler(recorder, req)

		if recorder.Code != http.StatusNoContent {
			t.Fatalf("expected status 204 for %s, got %d", remoteAddr, recorder.Code)
		}
	}

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"email":"worker@example.com","password":"pa55word"}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "10.0.0.3:1000"
	handler(recorder, req)

	if allowed != 2 {
		t.Fatalf("expected only first two requests to be allowed, got %d", allowed)
	}
	assertErrorResponse(t, recorder, http.StatusTooManyRequests, "TOO_MANY_REQUESTS")
}

func TestRateLimitMiddlewarePreservesRequestBodyForDownstreamHandler(t *testing.T) {
	t.Parallel()

	clock := newFakeRateLimitClock()
	handler := newRateLimitMiddleware(loginEmailRateLimitKey, rate.Every(time.Minute), 1, clock.now)(
		func(w http.ResponseWriter, r *http.Request) {
			var payload struct {
				Email string `json:"email"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode downstream body: %v", err)
			}
			if payload.Email != "worker@example.com" {
				t.Fatalf("expected email to be preserved, got %q", payload.Email)
			}
			w.WriteHeader(http.StatusNoContent)
		},
	)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"email":"worker@example.com","password":"pa55word"}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "10.0.0.1:1234"

	handler(recorder, req)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", recorder.Code)
	}
}

func TestClientIPRateLimitKey(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		remoteAddr string
		forwarded  string
		want       string
	}{
		{
			name:       "falls back to remote addr",
			remoteAddr: "10.0.0.9:8080",
			want:       "10.0.0.9",
		},
		{
			name:       "uses single forwarded IP",
			remoteAddr: "10.0.0.9:8080",
			forwarded:  "1.2.3.4",
			want:       "1.2.3.4",
		},
		{
			name:       "uses leftmost forwarded IP",
			remoteAddr: "10.0.0.9:8080",
			forwarded:  "1.2.3.4, 5.6.7.8",
			want:       "1.2.3.4",
		},
		{
			name:       "falls back on malformed forwarded IP",
			remoteAddr: "10.0.0.9:8080",
			forwarded:  "garbage",
			want:       "10.0.0.9",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
			req.RemoteAddr = tc.remoteAddr
			if tc.forwarded != "" {
				req.Header.Set("X-Forwarded-For", tc.forwarded)
			}

			got := clientIPRateLimitKey(req)
			if got != tc.want {
				t.Fatalf("expected key %q, got %q", tc.want, got)
			}
		})
	}
}
