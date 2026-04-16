package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestReadJSON(t *testing.T) {
	t.Run("rejects oversized request bodies", func(t *testing.T) {
		t.Parallel()

		payload, err := json.Marshal(map[string]string{
			"value": string(bytes.Repeat([]byte("a"), (1<<20)+1)),
		})
		if err != nil {
			t.Fatalf("marshal payload: %v", err)
		}

		req := httptest.NewRequest(http.MethodPost, "/templates", bytes.NewReader(payload))
		recorder := httptest.NewRecorder()

		var body struct {
			Value string `json:"value"`
		}
		err = readJSON(recorder, req, &body)
		if err == nil {
			t.Fatal("expected oversized body error")
		}

		var maxBytesErr *http.MaxBytesError
		if !errors.As(err, &maxBytesErr) {
			t.Fatalf("expected http.MaxBytesError, got %T (%v)", err, err)
		}
	})
}
