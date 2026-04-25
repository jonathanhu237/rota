package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/jonathanhu237/rota/backend/internal/config"
)

func TestRunRefusesProduction(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer
	err := run(context.Background(), &config.Config{AppEnv: "production"}, nil, &bytes.Buffer{}, &stderr)
	if err == nil {
		t.Fatal("expected production guard error")
	}
	if got := err.Error(); !strings.Contains(got, "production") || !strings.Contains(got, "refusing") {
		t.Fatalf("expected production refusal, got %q", got)
	}
}
