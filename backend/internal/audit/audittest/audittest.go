// Package audittest provides test helpers for code that emits audit events.
package audittest

import (
	"context"
	"sync"

	"github.com/jonathanhu237/rota/backend/internal/audit"
)

// Stub is an in-memory Recorder that captures every event it receives.
// Safe for concurrent use across parallel subtests.
type Stub struct {
	mu     sync.Mutex
	events []audit.RecordedEvent
}

// New returns a fresh Stub.
func New() *Stub {
	return &Stub{}
}

// Record implements audit.Recorder.
func (s *Stub) Record(_ context.Context, event audit.RecordedEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, event)
}

// Events returns a snapshot of captured events in order.
func (s *Stub) Events() []audit.RecordedEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	copied := make([]audit.RecordedEvent, len(s.events))
	copy(copied, s.events)
	return copied
}

// Actions returns the action names of captured events in order.
func (s *Stub) Actions() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	actions := make([]string, len(s.events))
	for i, e := range s.events {
		actions[i] = e.Action
	}
	return actions
}

// FindByAction returns the first event with the given action, or nil if
// none was recorded.
func (s *Stub) FindByAction(action string) *audit.RecordedEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.events {
		if s.events[i].Action == action {
			return &s.events[i]
		}
	}
	return nil
}

// Reset clears captured events.
func (s *Stub) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = nil
}

// ContextWith returns a context wired with this stub as the audit recorder.
// Convenience helper for tests that need to call services directly.
func (s *Stub) ContextWith(ctx context.Context) context.Context {
	return audit.WithRecorder(ctx, s)
}
