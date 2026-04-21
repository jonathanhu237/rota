package audit_test

import (
	"context"
	"testing"

	"github.com/jonathanhu237/rota/backend/internal/audit"
	"github.com/jonathanhu237/rota/backend/internal/audit/audittest"
)

func TestRecordPropagatesActorAndIPFromContext(t *testing.T) {
	t.Parallel()

	stub := audittest.New()
	ctx := audit.WithRecorder(context.Background(), stub)
	ctx = audit.WithActor(ctx, 42)
	ctx = audit.WithActorIP(ctx, "10.0.0.1")

	targetID := int64(7)
	audit.Record(ctx, audit.Event{
		Action:     audit.ActionUserCreate,
		TargetType: audit.TargetTypeUser,
		TargetID:   &targetID,
		Metadata:   map[string]any{"email": "new@example.com"},
	})

	events := stub.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	e := events[0]
	if e.Action != audit.ActionUserCreate {
		t.Fatalf("unexpected action %q", e.Action)
	}
	if e.ActorID == nil || *e.ActorID != 42 {
		t.Fatalf("unexpected actor id: %v", e.ActorID)
	}
	if e.ActorIP != "10.0.0.1" {
		t.Fatalf("unexpected actor ip: %q", e.ActorIP)
	}
	if e.TargetType != audit.TargetTypeUser {
		t.Fatalf("unexpected target type: %q", e.TargetType)
	}
	if e.TargetID == nil || *e.TargetID != 7 {
		t.Fatalf("unexpected target id: %v", e.TargetID)
	}
	if e.Metadata["email"] != "new@example.com" {
		t.Fatalf("unexpected metadata: %+v", e.Metadata)
	}
}

func TestRecordWithoutActorLeavesActorNil(t *testing.T) {
	t.Parallel()

	stub := audittest.New()
	ctx := audit.WithRecorder(context.Background(), stub)
	ctx = audit.WithActorIP(ctx, "1.2.3.4")

	audit.Record(ctx, audit.Event{Action: audit.ActionAuthLoginFailure})

	events := stub.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].ActorID != nil {
		t.Fatalf("expected nil actor id, got %v", events[0].ActorID)
	}
	if events[0].ActorIP != "1.2.3.4" {
		t.Fatalf("unexpected actor ip: %q", events[0].ActorIP)
	}
}

func TestRecordWithNoRecorderIsSafe(t *testing.T) {
	t.Parallel()

	// No recorder in context. Record should swallow the event, not panic.
	audit.Record(context.Background(), audit.Event{Action: audit.ActionUserCreate})
}

func TestRecordEmptyActionIgnored(t *testing.T) {
	t.Parallel()

	stub := audittest.New()
	ctx := audit.WithRecorder(context.Background(), stub)

	audit.Record(ctx, audit.Event{Action: ""})

	if len(stub.Events()) != 0 {
		t.Fatalf("expected event to be dropped, got %d", len(stub.Events()))
	}
}
