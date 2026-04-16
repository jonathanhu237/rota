package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jonathanhu237/rota/backend/internal/model"
)

func requestWithUser(r *http.Request, user *model.User) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), currentUserContextKey{}, user))
}

func requestWithPathValues(r *http.Request, values map[string]string) *http.Request {
	for key, value := range values {
		r.SetPathValue(key, value)
	}
	return r
}

func jsonRequest(t testing.TB, method, target string, payload any) *http.Request {
	t.Helper()

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}

	req := httptest.NewRequest(method, target, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func decodeJSONResponse[T any](t testing.TB, recorder *httptest.ResponseRecorder) T {
	t.Helper()

	var response T
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	return response
}

func assertErrorResponse(t testing.TB, recorder *httptest.ResponseRecorder, status int, code string) {
	t.Helper()

	if recorder.Code != status {
		t.Fatalf("expected status %d, got %d", status, recorder.Code)
	}

	response := decodeJSONResponse[errorResponse](t, recorder)
	if response.Error.Code != code {
		t.Fatalf("expected error code %q, got %q", code, response.Error.Code)
	}
}
