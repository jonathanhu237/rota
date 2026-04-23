package service

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jonathanhu237/rota/backend/internal/audit"
	"github.com/jonathanhu237/rota/backend/internal/audit/audittest"
	"github.com/jonathanhu237/rota/backend/internal/email"
	"github.com/jonathanhu237/rota/backend/internal/model"
	"github.com/jonathanhu237/rota/backend/internal/repository"
)

// GetAssignment completes the shiftChangeDeps interface on the existing
// publication mock by looking up an assignment by its ID.
func (m *publicationRepositoryStatefulMock) GetAssignment(
	ctx context.Context,
	id int64,
) (*model.Assignment, error) {
	for _, a := range m.assignments {
		if a.ID == id {
			return cloneAssignment(a), nil
		}
	}
	return nil, repository.ErrAssignmentNotFound
}

// -------- shift change repository stateful mock --------

type shiftChangeRepositoryStatefulMock struct {
	mu                                 sync.Mutex
	requests                           map[int64]*model.ShiftChangeRequest
	nextID                             int64
	invalidateRequestsForAssignmentErr error

	// pub is the paired publication repo used for assignment mutations in
	// ApplySwap / ApplyGive. Keyed like the real code uses for the assignments
	// map (pubID:userID:shiftID).
	pub *publicationRepositoryStatefulMock
}

func newShiftChangeRepositoryStatefulMock(pub *publicationRepositoryStatefulMock) *shiftChangeRepositoryStatefulMock {
	return &shiftChangeRepositoryStatefulMock{
		requests: make(map[int64]*model.ShiftChangeRequest),
		nextID:   1,
		pub:      pub,
	}
}

func cloneShiftChangeRequest(r *model.ShiftChangeRequest) *model.ShiftChangeRequest {
	if r == nil {
		return nil
	}
	cloned := *r
	if r.CounterpartUserID != nil {
		v := *r.CounterpartUserID
		cloned.CounterpartUserID = &v
	}
	if r.CounterpartAssignmentID != nil {
		v := *r.CounterpartAssignmentID
		cloned.CounterpartAssignmentID = &v
	}
	if r.DecidedByUserID != nil {
		v := *r.DecidedByUserID
		cloned.DecidedByUserID = &v
	}
	if r.DecidedAt != nil {
		v := *r.DecidedAt
		cloned.DecidedAt = &v
	}
	return &cloned
}

func (m *shiftChangeRepositoryStatefulMock) Create(
	ctx context.Context,
	params repository.CreateShiftChangeRequestParams,
) (*model.ShiftChangeRequest, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	req := &model.ShiftChangeRequest{
		ID:                      m.nextID,
		PublicationID:           params.PublicationID,
		Type:                    params.Type,
		RequesterUserID:         params.RequesterUserID,
		RequesterAssignmentID:   params.RequesterAssignmentID,
		CounterpartUserID:       params.CounterpartUserID,
		CounterpartAssignmentID: params.CounterpartAssignmentID,
		State:                   model.ShiftChangeStatePending,
		CreatedAt:               params.CreatedAt,
		ExpiresAt:               params.ExpiresAt,
	}
	m.nextID++
	m.requests[req.ID] = cloneShiftChangeRequest(req)
	return cloneShiftChangeRequest(req), nil
}

func (m *shiftChangeRepositoryStatefulMock) GetByID(
	ctx context.Context,
	id int64,
) (*model.ShiftChangeRequest, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	req, ok := m.requests[id]
	if !ok {
		return nil, repository.ErrShiftChangeNotFound
	}
	return cloneShiftChangeRequest(req), nil
}

func (m *shiftChangeRepositoryStatefulMock) ListForPublication(
	ctx context.Context,
	params repository.ListForPublicationParams,
) ([]*model.ShiftChangeRequest, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	out := make([]*model.ShiftChangeRequest, 0)
	for _, r := range m.requests {
		if r.PublicationID != params.PublicationID {
			continue
		}
		if !params.Audience.Admin {
			viewer := params.Audience.ViewerUserID
			allowed := r.RequesterUserID == viewer
			if r.CounterpartUserID != nil && *r.CounterpartUserID == viewer {
				allowed = true
			}
			if r.Type == model.ShiftChangeTypeGivePool {
				allowed = true
			}
			if !allowed {
				continue
			}
		}
		out = append(out, cloneShiftChangeRequest(r))
	}
	return out, nil
}

func (m *shiftChangeRepositoryStatefulMock) CountPendingForCounterpart(
	ctx context.Context,
	userID int64,
	now time.Time,
) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	for _, r := range m.requests {
		if r.CounterpartUserID == nil || *r.CounterpartUserID != userID {
			continue
		}
		if r.State != model.ShiftChangeStatePending {
			continue
		}
		if !r.ExpiresAt.After(now) {
			continue
		}
		count++
	}
	return count, nil
}

func (m *shiftChangeRepositoryStatefulMock) UpdateState(
	ctx context.Context,
	params repository.UpdateStateParams,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	req, ok := m.requests[params.ID]
	if !ok {
		return repository.ErrShiftChangeNotFound
	}
	if req.State != params.CurrentState {
		return repository.ErrShiftChangeNotPending
	}
	req.State = params.NextState
	if params.DecidedByUserID != nil {
		v := *params.DecidedByUserID
		req.DecidedByUserID = &v
	}
	t := params.Now
	req.DecidedAt = &t
	return nil
}

func (m *shiftChangeRepositoryStatefulMock) ApplySwap(
	ctx context.Context,
	params repository.ApplySwapParams,
) (*repository.ApproveResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	req, ok := m.requests[params.RequestID]
	if !ok {
		return nil, repository.ErrShiftChangeNotFound
	}
	if req.State != model.ShiftChangeStatePending {
		return nil, repository.ErrShiftChangeNotPending
	}

	requesterA := findAssignmentByID(m.pub, params.RequesterAssignmentID)
	counterpartA := findAssignmentByID(m.pub, params.CounterpartAssignmentID)
	if requesterA == nil || requesterA.UserID != params.RequesterUserID {
		return nil, repository.ErrShiftChangeAssignmentMiss
	}
	if counterpartA == nil || counterpartA.UserID != params.CounterpartUserID {
		return nil, repository.ErrShiftChangeAssignmentMiss
	}

	// Mutate the paired publication mock's assignments. Because the map is
	// keyed by pub:user:shift, a swap changes two keys.
	swapAssignmentUser(m.pub, requesterA.ID, params.CounterpartUserID)
	swapAssignmentUser(m.pub, counterpartA.ID, params.RequesterUserID)

	req.State = model.ShiftChangeStateApproved
	decidedBy := params.DecidedByUserID
	req.DecidedByUserID = &decidedBy
	t := params.Now
	req.DecidedAt = &t

	return &repository.ApproveResult{
		RequesterAssignment:   findAssignmentByID(m.pub, params.RequesterAssignmentID),
		CounterpartAssignment: findAssignmentByID(m.pub, params.CounterpartAssignmentID),
	}, nil
}

func (m *shiftChangeRepositoryStatefulMock) ApplyGive(
	ctx context.Context,
	params repository.ApplyGiveParams,
) (*repository.ApproveResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	req, ok := m.requests[params.RequestID]
	if !ok {
		return nil, repository.ErrShiftChangeNotFound
	}
	if req.State != model.ShiftChangeStatePending {
		return nil, repository.ErrShiftChangeNotPending
	}

	requesterA := findAssignmentByID(m.pub, params.RequesterAssignmentID)
	if requesterA == nil || requesterA.UserID != params.RequesterUserID {
		return nil, repository.ErrShiftChangeAssignmentMiss
	}

	swapAssignmentUser(m.pub, requesterA.ID, params.ReceiverUserID)

	req.State = model.ShiftChangeStateApproved
	decidedBy := params.DecidedByUserID
	req.DecidedByUserID = &decidedBy
	t := params.Now
	req.DecidedAt = &t

	return &repository.ApproveResult{
		RequesterAssignment: findAssignmentByID(m.pub, params.RequesterAssignmentID),
	}, nil
}

func (m *shiftChangeRepositoryStatefulMock) MarkInvalidated(
	ctx context.Context,
	id int64,
	now time.Time,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	req, ok := m.requests[id]
	if !ok {
		return repository.ErrShiftChangeNotFound
	}
	if req.State != model.ShiftChangeStatePending {
		return nil
	}
	req.State = model.ShiftChangeStateInvalidated
	t := now
	req.DecidedAt = &t
	return nil
}

func (m *shiftChangeRepositoryStatefulMock) MarkExpired(
	ctx context.Context,
	id int64,
	now time.Time,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	req, ok := m.requests[id]
	if !ok {
		return repository.ErrShiftChangeNotFound
	}
	if req.State != model.ShiftChangeStatePending {
		return nil
	}
	req.State = model.ShiftChangeStateExpired
	t := now
	req.DecidedAt = &t
	return nil
}

func (m *shiftChangeRepositoryStatefulMock) InvalidateRequestsForAssignment(
	ctx context.Context,
	assignmentID int64,
	now time.Time,
) ([]int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.invalidateRequestsForAssignmentErr != nil {
		return nil, m.invalidateRequestsForAssignmentErr
	}

	ids := make([]int64, 0)
	for id, req := range m.requests {
		if req.State != model.ShiftChangeStatePending {
			continue
		}
		if req.RequesterAssignmentID != assignmentID &&
			(req.CounterpartAssignmentID == nil || *req.CounterpartAssignmentID != assignmentID) {
			continue
		}

		req.State = model.ShiftChangeStateInvalidated
		t := now
		req.DecidedAt = &t
		ids = append(ids, id)
	}

	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids, nil
}

// findAssignmentByID returns a pointer to the assignment stored in the
// publication mock's map (NOT a clone) — callers may mutate.
func findAssignmentByID(pub *publicationRepositoryStatefulMock, id int64) *model.Assignment {
	for _, a := range pub.assignments {
		if a.ID == id {
			return a
		}
	}
	return nil
}

// swapAssignmentUser replaces the user_id on the assignment with the given
// ID, re-keying the entry so the composite pub:user:slot key stays correct.
func swapAssignmentUser(pub *publicationRepositoryStatefulMock, assignmentID int64, newUserID int64) {
	for key, a := range pub.assignments {
		if a.ID != assignmentID {
			continue
		}
		delete(pub.assignments, key)
		a.UserID = newUserID
		pub.assignments[assignmentKey(a.PublicationID, a.UserID, a.SlotID)] = a
		return
	}
}

// -------- email stub --------

type emailStub struct {
	mu   sync.Mutex
	sent []email.Message
}

func (s *emailStub) Send(_ context.Context, m email.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sent = append(s.sent, m)
	return nil
}

func (s *emailStub) messages() []email.Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	copied := make([]email.Message, len(s.sent))
	copy(copied, s.sent)
	return copied
}

// -------- helpers --------

// buildShiftChangeFixture sets up the publication + shift change repos with
// a published publication and a pair of assignments for the common test
// scenario: user 7 assigned to shift 11 (position 101), user 8 assigned to
// shift 12 (position 102). Both users are mutually qualified for both
// positions.
func buildShiftChangeFixture(now time.Time) (*publicationRepositoryStatefulMock, *shiftChangeRepositoryStatefulMock) {
	pub := newPublicationRepositoryStatefulMock()
	pub.publications[1] = publishedPublication(now)

	// Qualifications: both users qualified for both positions, giving us
	// enough flexibility for every scenario.
	pub.qualifiedByUser[7] = map[int64]struct{}{101: {}, 102: {}}
	pub.qualifiedByUser[8] = map[int64]struct{}{101: {}, 102: {}}

	// Assignments: user 7 on slot 21 (Monday 09:00-12:00, position 101),
	// user 8 on slot 22 (Wednesday 13:00-17:00, position 102).
	pub.assignments[assignmentKey(1, 7, 21)] = &model.Assignment{
		ID:            100,
		PublicationID: 1,
		UserID:        7,
		SlotID:        21,
		PositionID:    101,
		CreatedAt:     now.Add(-24 * time.Hour),
	}
	pub.assignments[assignmentKey(1, 8, 22)] = &model.Assignment{
		ID:            101,
		PublicationID: 1,
		UserID:        8,
		SlotID:        22,
		PositionID:    102,
		CreatedAt:     now.Add(-24 * time.Hour),
	}
	pub.nextAssignmentID = 200

	sc := newShiftChangeRepositoryStatefulMock(pub)
	return pub, sc
}

func newTestShiftChangeService(
	pub *publicationRepositoryStatefulMock,
	sc *shiftChangeRepositoryStatefulMock,
	emailer email.Emailer,
	now time.Time,
) *ShiftChangeService {
	return NewShiftChangeService(sc, pub, emailer, "https://rota.example.com", fixedClock{now: now}, nil)
}

// assertNoMetadataLeak JSON-marshals each event's metadata and verifies it
// contains none of the forbidden substrings.
func assertNoMetadataLeak(t *testing.T, events []audit.RecordedEvent) {
	t.Helper()
	for _, e := range events {
		if e.Metadata == nil {
			continue
		}
		raw, err := json.Marshal(e.Metadata)
		if err != nil {
			t.Fatalf("marshal metadata for %q: %v", e.Action, err)
		}
		lower := strings.ToLower(string(raw))
		for _, forbidden := range []string{"password", "token", "session"} {
			if strings.Contains(lower, forbidden) {
				t.Fatalf("event %q metadata leaked %q: %s", e.Action, forbidden, raw)
			}
		}
	}
}

// -------- CreateShiftChangeRequest --------

func TestShiftChangeServiceCreate(t *testing.T) {
	t.Run("swap valid persists request with audit and email", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		pub, sc := buildShiftChangeFixture(now)
		emailer := &emailStub{}
		svc := newTestShiftChangeService(pub, sc, emailer, now)

		stub := audittest.New()
		ctx := stub.ContextWith(context.Background())

		counterpartUserID := int64(8)
		counterpartAssignmentID := int64(101)
		req, err := svc.CreateShiftChangeRequest(ctx, CreateShiftChangeInput{
			PublicationID:           1,
			RequesterUserID:         7,
			Type:                    model.ShiftChangeTypeSwap,
			RequesterAssignmentID:   100,
			CounterpartUserID:       &counterpartUserID,
			CounterpartAssignmentID: &counterpartAssignmentID,
		})
		if err != nil {
			t.Fatalf("CreateShiftChangeRequest returned error: %v", err)
		}
		if req == nil || req.ID == 0 {
			t.Fatalf("expected persisted request, got %+v", req)
		}
		if _, ok := sc.requests[req.ID]; !ok {
			t.Fatalf("expected request %d to be persisted", req.ID)
		}

		event := stub.FindByAction(audit.ActionShiftChangeCreate)
		if event == nil {
			t.Fatalf("expected shift_change.create audit, got %v", stub.Actions())
		}
		if event.TargetType != audit.TargetTypeShiftChangeRequest {
			t.Fatalf("unexpected target type: %q", event.TargetType)
		}
		if event.TargetID == nil || *event.TargetID != req.ID {
			t.Fatalf("unexpected target id: %v", event.TargetID)
		}

		msgs := emailer.messages()
		if len(msgs) != 1 {
			t.Fatalf("expected 1 email sent, got %d", len(msgs))
		}
		if msgs[0].To != "bob@example.com" {
			t.Fatalf("expected counterpart email, got %q", msgs[0].To)
		}

		assertNoMetadataLeak(t, stub.Events())
	})

	t.Run("swap counterpart not qualified for requester position", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		pub, sc := buildShiftChangeFixture(now)
		// Counterpart (user 8) loses qualification for position 101.
		pub.qualifiedByUser[8] = map[int64]struct{}{102: {}}
		emailer := &emailStub{}
		svc := newTestShiftChangeService(pub, sc, emailer, now)

		stub := audittest.New()
		ctx := stub.ContextWith(context.Background())

		counterpartUserID := int64(8)
		counterpartAssignmentID := int64(101)
		_, err := svc.CreateShiftChangeRequest(ctx, CreateShiftChangeInput{
			PublicationID:           1,
			RequesterUserID:         7,
			Type:                    model.ShiftChangeTypeSwap,
			RequesterAssignmentID:   100,
			CounterpartUserID:       &counterpartUserID,
			CounterpartAssignmentID: &counterpartAssignmentID,
		})
		if !errors.Is(err, ErrShiftChangeNotQualified) {
			t.Fatalf("expected ErrShiftChangeNotQualified, got %v", err)
		}
		if len(sc.requests) != 0 {
			t.Fatalf("expected no persisted request, got %d", len(sc.requests))
		}
		if len(stub.Events()) != 0 {
			t.Fatalf("expected no audit events, got %+v", stub.Events())
		}
		if len(emailer.messages()) != 0 {
			t.Fatalf("expected no email sent, got %d", len(emailer.messages()))
		}
	})

	t.Run("swap requester not qualified for counterpart position", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		pub, sc := buildShiftChangeFixture(now)
		// Requester (user 7) loses qualification for position 102.
		pub.qualifiedByUser[7] = map[int64]struct{}{101: {}}
		svc := newTestShiftChangeService(pub, sc, &emailStub{}, now)

		counterpartUserID := int64(8)
		counterpartAssignmentID := int64(101)
		_, err := svc.CreateShiftChangeRequest(context.Background(), CreateShiftChangeInput{
			PublicationID:           1,
			RequesterUserID:         7,
			Type:                    model.ShiftChangeTypeSwap,
			RequesterAssignmentID:   100,
			CounterpartUserID:       &counterpartUserID,
			CounterpartAssignmentID: &counterpartAssignmentID,
		})
		if !errors.Is(err, ErrShiftChangeNotQualified) {
			t.Fatalf("expected ErrShiftChangeNotQualified, got %v", err)
		}
	})

	t.Run("swap with counterpart equal to requester is self error", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		pub, sc := buildShiftChangeFixture(now)
		svc := newTestShiftChangeService(pub, sc, &emailStub{}, now)

		counterpartUserID := int64(7) // same as requester
		counterpartAssignmentID := int64(100)
		_, err := svc.CreateShiftChangeRequest(context.Background(), CreateShiftChangeInput{
			PublicationID:           1,
			RequesterUserID:         7,
			Type:                    model.ShiftChangeTypeSwap,
			RequesterAssignmentID:   100,
			CounterpartUserID:       &counterpartUserID,
			CounterpartAssignmentID: &counterpartAssignmentID,
		})
		if !errors.Is(err, ErrShiftChangeSelf) {
			t.Fatalf("expected ErrShiftChangeSelf, got %v", err)
		}
	})

	t.Run("give_direct qualified target succeeds with email", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		pub, sc := buildShiftChangeFixture(now)
		emailer := &emailStub{}
		svc := newTestShiftChangeService(pub, sc, emailer, now)

		counterpartUserID := int64(8)
		req, err := svc.CreateShiftChangeRequest(context.Background(), CreateShiftChangeInput{
			PublicationID:         1,
			RequesterUserID:       7,
			Type:                  model.ShiftChangeTypeGiveDirect,
			RequesterAssignmentID: 100,
			CounterpartUserID:     &counterpartUserID,
		})
		if err != nil {
			t.Fatalf("CreateShiftChangeRequest returned error: %v", err)
		}
		if req == nil {
			t.Fatal("expected request to be persisted")
		}
		if len(emailer.messages()) != 1 {
			t.Fatalf("expected 1 email sent, got %d", len(emailer.messages()))
		}
	})

	t.Run("give_direct unqualified target errors", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		pub, sc := buildShiftChangeFixture(now)
		pub.qualifiedByUser[8] = map[int64]struct{}{102: {}}
		svc := newTestShiftChangeService(pub, sc, &emailStub{}, now)

		counterpartUserID := int64(8)
		_, err := svc.CreateShiftChangeRequest(context.Background(), CreateShiftChangeInput{
			PublicationID:         1,
			RequesterUserID:       7,
			Type:                  model.ShiftChangeTypeGiveDirect,
			RequesterAssignmentID: 100,
			CounterpartUserID:     &counterpartUserID,
		})
		if !errors.Is(err, ErrShiftChangeNotQualified) {
			t.Fatalf("expected ErrShiftChangeNotQualified, got %v", err)
		}
	})

	t.Run("give_pool always succeeds when published and sends no email", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		pub, sc := buildShiftChangeFixture(now)
		emailer := &emailStub{}
		svc := newTestShiftChangeService(pub, sc, emailer, now)

		req, err := svc.CreateShiftChangeRequest(context.Background(), CreateShiftChangeInput{
			PublicationID:         1,
			RequesterUserID:       7,
			Type:                  model.ShiftChangeTypeGivePool,
			RequesterAssignmentID: 100,
		})
		if err != nil {
			t.Fatalf("CreateShiftChangeRequest returned error: %v", err)
		}
		if req == nil {
			t.Fatal("expected give_pool request to be persisted")
		}
		if len(emailer.messages()) != 0 {
			t.Fatalf("expected no email for give_pool, got %d", len(emailer.messages()))
		}
	})

	t.Run("publication not published returns domain error", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		pub, sc := buildShiftChangeFixture(now)
		pub.publications[1].State = model.PublicationStateActive
		svc := newTestShiftChangeService(pub, sc, &emailStub{}, now)

		counterpartUserID := int64(8)
		counterpartAssignmentID := int64(101)
		_, err := svc.CreateShiftChangeRequest(context.Background(), CreateShiftChangeInput{
			PublicationID:           1,
			RequesterUserID:         7,
			Type:                    model.ShiftChangeTypeSwap,
			RequesterAssignmentID:   100,
			CounterpartUserID:       &counterpartUserID,
			CounterpartAssignmentID: &counterpartAssignmentID,
		})
		if !errors.Is(err, ErrPublicationNotPublished) {
			t.Fatalf("expected ErrPublicationNotPublished, got %v", err)
		}
	})

	t.Run("requester does not own requester_assignment_id", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		pub, sc := buildShiftChangeFixture(now)
		svc := newTestShiftChangeService(pub, sc, &emailStub{}, now)

		counterpartUserID := int64(8)
		counterpartAssignmentID := int64(101)
		// Pass assignment 101 (user 8's) as requester's assignment; user 7
		// doesn't own it.
		_, err := svc.CreateShiftChangeRequest(context.Background(), CreateShiftChangeInput{
			PublicationID:           1,
			RequesterUserID:         7,
			Type:                    model.ShiftChangeTypeSwap,
			RequesterAssignmentID:   101,
			CounterpartUserID:       &counterpartUserID,
			CounterpartAssignmentID: &counterpartAssignmentID,
		})
		if !errors.Is(err, ErrShiftChangeNotOwner) {
			t.Fatalf("expected ErrShiftChangeNotOwner, got %v", err)
		}
	})

	t.Run("swap counterpart does not own counterpart_assignment_id", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		pub, sc := buildShiftChangeFixture(now)
		svc := newTestShiftChangeService(pub, sc, &emailStub{}, now)

		counterpartUserID := int64(8)
		counterpartAssignmentID := int64(100) // This belongs to user 7.
		_, err := svc.CreateShiftChangeRequest(context.Background(), CreateShiftChangeInput{
			PublicationID:           1,
			RequesterUserID:         7,
			Type:                    model.ShiftChangeTypeSwap,
			RequesterAssignmentID:   100,
			CounterpartUserID:       &counterpartUserID,
			CounterpartAssignmentID: &counterpartAssignmentID,
		})
		if !errors.Is(err, ErrShiftChangeNotFound) {
			t.Fatalf("expected ErrShiftChangeNotFound, got %v", err)
		}
	})
}

// -------- ApproveShiftChangeRequest --------

func TestShiftChangeServiceApprove(t *testing.T) {
	t.Run("swap happy path swaps assignments and emits audit + email", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		pub, sc := buildShiftChangeFixture(now)
		emailer := &emailStub{}
		svc := newTestShiftChangeService(pub, sc, emailer, now)

		counterpartUserID := int64(8)
		counterpartAssignmentID := int64(101)
		created, err := svc.CreateShiftChangeRequest(context.Background(), CreateShiftChangeInput{
			PublicationID:           1,
			RequesterUserID:         7,
			Type:                    model.ShiftChangeTypeSwap,
			RequesterAssignmentID:   100,
			CounterpartUserID:       &counterpartUserID,
			CounterpartAssignmentID: &counterpartAssignmentID,
		})
		if err != nil {
			t.Fatalf("create setup failed: %v", err)
		}

		stub := audittest.New()
		ctx := stub.ContextWith(context.Background())

		if err := svc.ApproveShiftChangeRequest(ctx, created.ID, 8); err != nil {
			t.Fatalf("ApproveShiftChangeRequest returned error: %v", err)
		}

		// Assignments should have swapped user_ids.
		a100 := findAssignmentByID(pub, 100)
		a101 := findAssignmentByID(pub, 101)
		if a100 == nil || a100.UserID != 8 {
			t.Fatalf("expected assignment 100 to move to user 8, got %+v", a100)
		}
		if a101 == nil || a101.UserID != 7 {
			t.Fatalf("expected assignment 101 to move to user 7, got %+v", a101)
		}

		if sc.requests[created.ID].State != model.ShiftChangeStateApproved {
			t.Fatalf("expected approved, got %s", sc.requests[created.ID].State)
		}

		if stub.FindByAction(audit.ActionShiftChangeApprove) == nil {
			t.Fatalf("expected approve audit event, got %v", stub.Actions())
		}

		msgs := emailer.messages()
		// Approve sends one email to the requester. (Create also sent one, but
		// that happened under a different context so this emailer captures both.)
		if len(msgs) < 2 {
			t.Fatalf("expected at least 2 emails (create + resolve), got %d", len(msgs))
		}
		last := msgs[len(msgs)-1]
		if last.To != "alice@example.com" {
			t.Fatalf("expected resolved email to requester alice, got %q", last.To)
		}
		if !strings.Contains(strings.ToLower(last.Body), "approved") {
			t.Fatalf("expected outcome=approved in body, got %q", last.Body)
		}

		assertNoMetadataLeak(t, stub.Events())
	})

	t.Run("give_direct happy path transfers assignment", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		pub, sc := buildShiftChangeFixture(now)
		svc := newTestShiftChangeService(pub, sc, &emailStub{}, now)

		counterpartUserID := int64(8)
		created, err := svc.CreateShiftChangeRequest(context.Background(), CreateShiftChangeInput{
			PublicationID:         1,
			RequesterUserID:       7,
			Type:                  model.ShiftChangeTypeGiveDirect,
			RequesterAssignmentID: 100,
			CounterpartUserID:     &counterpartUserID,
		})
		if err != nil {
			t.Fatalf("create setup failed: %v", err)
		}

		if err := svc.ApproveShiftChangeRequest(context.Background(), created.ID, 8); err != nil {
			t.Fatalf("ApproveShiftChangeRequest returned error: %v", err)
		}

		a100 := findAssignmentByID(pub, 100)
		if a100 == nil || a100.UserID != 8 {
			t.Fatalf("expected assignment 100 transferred to user 8, got %+v", a100)
		}
		if sc.requests[created.ID].State != model.ShiftChangeStateApproved {
			t.Fatalf("expected approved state, got %s", sc.requests[created.ID].State)
		}
	})

	t.Run("give_pool claim transfers assignment with outcome=claimed", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		pub, sc := buildShiftChangeFixture(now)
		emailer := &emailStub{}
		svc := newTestShiftChangeService(pub, sc, emailer, now)

		created, err := svc.CreateShiftChangeRequest(context.Background(), CreateShiftChangeInput{
			PublicationID:         1,
			RequesterUserID:       7,
			Type:                  model.ShiftChangeTypeGivePool,
			RequesterAssignmentID: 100,
		})
		if err != nil {
			t.Fatalf("create setup failed: %v", err)
		}

		if err := svc.ApproveShiftChangeRequest(context.Background(), created.ID, 8); err != nil {
			t.Fatalf("ApproveShiftChangeRequest returned error: %v", err)
		}

		a100 := findAssignmentByID(pub, 100)
		if a100 == nil || a100.UserID != 8 {
			t.Fatalf("expected assignment 100 transferred to user 8, got %+v", a100)
		}
		if sc.requests[created.ID].State != model.ShiftChangeStateApproved {
			t.Fatalf("expected approved state, got %s", sc.requests[created.ID].State)
		}

		msgs := emailer.messages()
		if len(msgs) != 1 {
			t.Fatalf("expected one email (the resolve notification), got %d", len(msgs))
		}
		if !strings.Contains(strings.ToLower(msgs[0].Body), "claimed") {
			t.Fatalf("expected outcome=claimed, got %q", msgs[0].Body)
		}
	})

	t.Run("approve when assignment changed underneath invalidates request", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		pub, sc := buildShiftChangeFixture(now)
		svc := newTestShiftChangeService(pub, sc, &emailStub{}, now)

		counterpartUserID := int64(8)
		counterpartAssignmentID := int64(101)
		created, err := svc.CreateShiftChangeRequest(context.Background(), CreateShiftChangeInput{
			PublicationID:           1,
			RequesterUserID:         7,
			Type:                    model.ShiftChangeTypeSwap,
			RequesterAssignmentID:   100,
			CounterpartUserID:       &counterpartUserID,
			CounterpartAssignmentID: &counterpartAssignmentID,
		})
		if err != nil {
			t.Fatalf("create setup failed: %v", err)
		}

		// Simulate the assignment moving out from under the request: reassign
		// assignment 100 to some other user.
		swapAssignmentUser(pub, 100, 9)

		err = svc.ApproveShiftChangeRequest(context.Background(), created.ID, 8)
		if !errors.Is(err, ErrShiftChangeInvalidated) {
			t.Fatalf("expected ErrShiftChangeInvalidated, got %v", err)
		}
		if sc.requests[created.ID].State != model.ShiftChangeStateInvalidated {
			t.Fatalf("expected invalidated state, got %s", sc.requests[created.ID].State)
		}
	})

	t.Run("approve swap with time conflict", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		pub, sc := buildShiftChangeFixture(now)
		// Add a third shift that overlaps with shift 12 on the same weekday
		// (Wednesday). User 7 already holds this third shift, so receiving
		// shift 12 via the swap would create a time conflict.
		pub.templateSlots[23] = &model.TemplateSlot{
			ID:         23,
			TemplateID: 1,
			Weekday:    3,
			StartTime:  "14:00",
			EndTime:    "16:00",
		}
		pub.slotPositions[23] = []*model.TemplateSlotPosition{{
			ID:                13,
			SlotID:            23,
			PositionID:        101,
			RequiredHeadcount: 1,
		}}
		pub.assignments[assignmentKey(1, 7, 23)] = &model.Assignment{
			ID:            102,
			PublicationID: 1,
			UserID:        7,
			SlotID:        23,
			PositionID:    101,
			CreatedAt:     now.Add(-24 * time.Hour),
		}
		svc := newTestShiftChangeService(pub, sc, &emailStub{}, now)

		counterpartUserID := int64(8)
		counterpartAssignmentID := int64(101)
		created, err := svc.CreateShiftChangeRequest(context.Background(), CreateShiftChangeInput{
			PublicationID:           1,
			RequesterUserID:         7,
			Type:                    model.ShiftChangeTypeSwap,
			RequesterAssignmentID:   100,
			CounterpartUserID:       &counterpartUserID,
			CounterpartAssignmentID: &counterpartAssignmentID,
		})
		if err != nil {
			t.Fatalf("create setup failed: %v", err)
		}

		err = svc.ApproveShiftChangeRequest(context.Background(), created.ID, 8)
		if !errors.Is(err, ErrShiftChangeTimeConflict) {
			t.Fatalf("expected ErrShiftChangeTimeConflict, got %v", err)
		}
	})

	t.Run("approve by non-counterpart on swap is not owner", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		pub, sc := buildShiftChangeFixture(now)
		svc := newTestShiftChangeService(pub, sc, &emailStub{}, now)

		counterpartUserID := int64(8)
		counterpartAssignmentID := int64(101)
		created, err := svc.CreateShiftChangeRequest(context.Background(), CreateShiftChangeInput{
			PublicationID:           1,
			RequesterUserID:         7,
			Type:                    model.ShiftChangeTypeSwap,
			RequesterAssignmentID:   100,
			CounterpartUserID:       &counterpartUserID,
			CounterpartAssignmentID: &counterpartAssignmentID,
		})
		if err != nil {
			t.Fatalf("create setup failed: %v", err)
		}

		// User 9 is neither requester nor counterpart.
		err = svc.ApproveShiftChangeRequest(context.Background(), created.ID, 9)
		if !errors.Is(err, ErrShiftChangeNotOwner) {
			t.Fatalf("expected ErrShiftChangeNotOwner, got %v", err)
		}
	})

	t.Run("pool claim by requester is self error", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		pub, sc := buildShiftChangeFixture(now)
		svc := newTestShiftChangeService(pub, sc, &emailStub{}, now)

		created, err := svc.CreateShiftChangeRequest(context.Background(), CreateShiftChangeInput{
			PublicationID:         1,
			RequesterUserID:       7,
			Type:                  model.ShiftChangeTypeGivePool,
			RequesterAssignmentID: 100,
		})
		if err != nil {
			t.Fatalf("create setup failed: %v", err)
		}

		err = svc.ApproveShiftChangeRequest(context.Background(), created.ID, 7)
		if !errors.Is(err, ErrShiftChangeSelf) {
			t.Fatalf("expected ErrShiftChangeSelf, got %v", err)
		}
	})

	t.Run("pool claim by unqualified user is rejected", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		pub, sc := buildShiftChangeFixture(now)
		// User 8 no longer qualified for position 101 (the shift being given).
		pub.qualifiedByUser[8] = map[int64]struct{}{102: {}}
		svc := newTestShiftChangeService(pub, sc, &emailStub{}, now)

		created, err := svc.CreateShiftChangeRequest(context.Background(), CreateShiftChangeInput{
			PublicationID:         1,
			RequesterUserID:       7,
			Type:                  model.ShiftChangeTypeGivePool,
			RequesterAssignmentID: 100,
		})
		if err != nil {
			t.Fatalf("create setup failed: %v", err)
		}

		err = svc.ApproveShiftChangeRequest(context.Background(), created.ID, 8)
		if !errors.Is(err, ErrShiftChangeNotQualified) {
			t.Fatalf("expected ErrShiftChangeNotQualified, got %v", err)
		}
	})

	t.Run("approve when request already decided is not pending", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		pub, sc := buildShiftChangeFixture(now)
		svc := newTestShiftChangeService(pub, sc, &emailStub{}, now)

		counterpartUserID := int64(8)
		counterpartAssignmentID := int64(101)
		created, err := svc.CreateShiftChangeRequest(context.Background(), CreateShiftChangeInput{
			PublicationID:           1,
			RequesterUserID:         7,
			Type:                    model.ShiftChangeTypeSwap,
			RequesterAssignmentID:   100,
			CounterpartUserID:       &counterpartUserID,
			CounterpartAssignmentID: &counterpartAssignmentID,
		})
		if err != nil {
			t.Fatalf("create setup failed: %v", err)
		}

		// Mark rejected first.
		sc.requests[created.ID].State = model.ShiftChangeStateRejected
		err = svc.ApproveShiftChangeRequest(context.Background(), created.ID, 8)
		if !errors.Is(err, ErrShiftChangeNotPending) {
			t.Fatalf("expected ErrShiftChangeNotPending, got %v", err)
		}
	})

	t.Run("approve when request expired marks expired", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		pub, sc := buildShiftChangeFixture(now)
		// Make the publication expire-at in the past by setting a planned
		// active from that is already past. Create with create-time clock,
		// then approve with a much-later clock so the request is expired.
		pub.publications[1].PlannedActiveFrom = now.Add(1 * time.Minute)
		svcCreate := newTestShiftChangeService(pub, sc, &emailStub{}, now)

		counterpartUserID := int64(8)
		counterpartAssignmentID := int64(101)
		created, err := svcCreate.CreateShiftChangeRequest(context.Background(), CreateShiftChangeInput{
			PublicationID:           1,
			RequesterUserID:         7,
			Type:                    model.ShiftChangeTypeSwap,
			RequesterAssignmentID:   100,
			CounterpartUserID:       &counterpartUserID,
			CounterpartAssignmentID: &counterpartAssignmentID,
		})
		if err != nil {
			t.Fatalf("create setup failed: %v", err)
		}

		later := now.Add(1 * time.Hour)
		svcApprove := newTestShiftChangeService(pub, sc, &emailStub{}, later)
		err = svcApprove.ApproveShiftChangeRequest(context.Background(), created.ID, 8)
		if !errors.Is(err, ErrShiftChangeExpired) {
			t.Fatalf("expected ErrShiftChangeExpired, got %v", err)
		}
		if sc.requests[created.ID].State != model.ShiftChangeStateExpired {
			t.Fatalf("expected expired state, got %s", sc.requests[created.ID].State)
		}
	})

	t.Run("approve when publication no longer published", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		pub, sc := buildShiftChangeFixture(now)
		svc := newTestShiftChangeService(pub, sc, &emailStub{}, now)

		counterpartUserID := int64(8)
		counterpartAssignmentID := int64(101)
		created, err := svc.CreateShiftChangeRequest(context.Background(), CreateShiftChangeInput{
			PublicationID:           1,
			RequesterUserID:         7,
			Type:                    model.ShiftChangeTypeSwap,
			RequesterAssignmentID:   100,
			CounterpartUserID:       &counterpartUserID,
			CounterpartAssignmentID: &counterpartAssignmentID,
		})
		if err != nil {
			t.Fatalf("create setup failed: %v", err)
		}

		// Flip publication out of PUBLISHED state.
		pub.publications[1].State = model.PublicationStateActive

		err = svc.ApproveShiftChangeRequest(context.Background(), created.ID, 8)
		if !errors.Is(err, ErrPublicationNotPublished) {
			t.Fatalf("expected ErrPublicationNotPublished, got %v", err)
		}
	})
}

// -------- RejectShiftChangeRequest --------

func TestShiftChangeServiceReject(t *testing.T) {
	t.Run("counterpart rejects swap sends resolved email", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		pub, sc := buildShiftChangeFixture(now)
		emailer := &emailStub{}
		svc := newTestShiftChangeService(pub, sc, emailer, now)

		counterpartUserID := int64(8)
		counterpartAssignmentID := int64(101)
		created, err := svc.CreateShiftChangeRequest(context.Background(), CreateShiftChangeInput{
			PublicationID:           1,
			RequesterUserID:         7,
			Type:                    model.ShiftChangeTypeSwap,
			RequesterAssignmentID:   100,
			CounterpartUserID:       &counterpartUserID,
			CounterpartAssignmentID: &counterpartAssignmentID,
		})
		if err != nil {
			t.Fatalf("create setup failed: %v", err)
		}

		before := len(emailer.messages())
		if err := svc.RejectShiftChangeRequest(context.Background(), created.ID, 8); err != nil {
			t.Fatalf("RejectShiftChangeRequest returned error: %v", err)
		}
		if sc.requests[created.ID].State != model.ShiftChangeStateRejected {
			t.Fatalf("expected rejected state, got %s", sc.requests[created.ID].State)
		}

		msgs := emailer.messages()
		if len(msgs) <= before {
			t.Fatalf("expected a reject email to be sent")
		}
		last := msgs[len(msgs)-1]
		if !strings.Contains(strings.ToLower(last.Body), "rejected") {
			t.Fatalf("expected outcome=rejected body, got %q", last.Body)
		}
	})

	t.Run("counterpart rejects give_direct", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		pub, sc := buildShiftChangeFixture(now)
		emailer := &emailStub{}
		svc := newTestShiftChangeService(pub, sc, emailer, now)

		counterpartUserID := int64(8)
		created, err := svc.CreateShiftChangeRequest(context.Background(), CreateShiftChangeInput{
			PublicationID:         1,
			RequesterUserID:       7,
			Type:                  model.ShiftChangeTypeGiveDirect,
			RequesterAssignmentID: 100,
			CounterpartUserID:     &counterpartUserID,
		})
		if err != nil {
			t.Fatalf("create setup failed: %v", err)
		}

		if err := svc.RejectShiftChangeRequest(context.Background(), created.ID, 8); err != nil {
			t.Fatalf("RejectShiftChangeRequest returned error: %v", err)
		}
		if sc.requests[created.ID].State != model.ShiftChangeStateRejected {
			t.Fatalf("expected rejected, got %s", sc.requests[created.ID].State)
		}
	})

	t.Run("non-counterpart rejecting is not owner", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		pub, sc := buildShiftChangeFixture(now)
		svc := newTestShiftChangeService(pub, sc, &emailStub{}, now)

		counterpartUserID := int64(8)
		counterpartAssignmentID := int64(101)
		created, err := svc.CreateShiftChangeRequest(context.Background(), CreateShiftChangeInput{
			PublicationID:           1,
			RequesterUserID:         7,
			Type:                    model.ShiftChangeTypeSwap,
			RequesterAssignmentID:   100,
			CounterpartUserID:       &counterpartUserID,
			CounterpartAssignmentID: &counterpartAssignmentID,
		})
		if err != nil {
			t.Fatalf("create setup failed: %v", err)
		}

		err = svc.RejectShiftChangeRequest(context.Background(), created.ID, 9)
		if !errors.Is(err, ErrShiftChangeNotOwner) {
			t.Fatalf("expected ErrShiftChangeNotOwner, got %v", err)
		}
	})

	t.Run("rejecting give_pool is invalid type", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		pub, sc := buildShiftChangeFixture(now)
		svc := newTestShiftChangeService(pub, sc, &emailStub{}, now)

		created, err := svc.CreateShiftChangeRequest(context.Background(), CreateShiftChangeInput{
			PublicationID:         1,
			RequesterUserID:       7,
			Type:                  model.ShiftChangeTypeGivePool,
			RequesterAssignmentID: 100,
		})
		if err != nil {
			t.Fatalf("create setup failed: %v", err)
		}

		err = svc.RejectShiftChangeRequest(context.Background(), created.ID, 8)
		if !errors.Is(err, ErrShiftChangeInvalidType) {
			t.Fatalf("expected ErrShiftChangeInvalidType, got %v", err)
		}
	})

	t.Run("rejecting already decided is not pending", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		pub, sc := buildShiftChangeFixture(now)
		svc := newTestShiftChangeService(pub, sc, &emailStub{}, now)

		counterpartUserID := int64(8)
		counterpartAssignmentID := int64(101)
		created, err := svc.CreateShiftChangeRequest(context.Background(), CreateShiftChangeInput{
			PublicationID:           1,
			RequesterUserID:         7,
			Type:                    model.ShiftChangeTypeSwap,
			RequesterAssignmentID:   100,
			CounterpartUserID:       &counterpartUserID,
			CounterpartAssignmentID: &counterpartAssignmentID,
		})
		if err != nil {
			t.Fatalf("create setup failed: %v", err)
		}

		sc.requests[created.ID].State = model.ShiftChangeStateCancelled
		err = svc.RejectShiftChangeRequest(context.Background(), created.ID, 8)
		if !errors.Is(err, ErrShiftChangeNotPending) {
			t.Fatalf("expected ErrShiftChangeNotPending, got %v", err)
		}
	})

	t.Run("rejecting expired marks expired", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		pub, sc := buildShiftChangeFixture(now)
		pub.publications[1].PlannedActiveFrom = now.Add(1 * time.Minute)
		svcCreate := newTestShiftChangeService(pub, sc, &emailStub{}, now)

		counterpartUserID := int64(8)
		counterpartAssignmentID := int64(101)
		created, err := svcCreate.CreateShiftChangeRequest(context.Background(), CreateShiftChangeInput{
			PublicationID:           1,
			RequesterUserID:         7,
			Type:                    model.ShiftChangeTypeSwap,
			RequesterAssignmentID:   100,
			CounterpartUserID:       &counterpartUserID,
			CounterpartAssignmentID: &counterpartAssignmentID,
		})
		if err != nil {
			t.Fatalf("create setup failed: %v", err)
		}

		later := now.Add(2 * time.Hour)
		svcLater := newTestShiftChangeService(pub, sc, &emailStub{}, later)
		err = svcLater.RejectShiftChangeRequest(context.Background(), created.ID, 8)
		if !errors.Is(err, ErrShiftChangeExpired) {
			t.Fatalf("expected ErrShiftChangeExpired, got %v", err)
		}
	})
}

// -------- CancelShiftChangeRequest --------

func TestShiftChangeServiceCancel(t *testing.T) {
	t.Run("requester cancels successfully", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		pub, sc := buildShiftChangeFixture(now)
		svc := newTestShiftChangeService(pub, sc, &emailStub{}, now)

		counterpartUserID := int64(8)
		counterpartAssignmentID := int64(101)
		created, err := svc.CreateShiftChangeRequest(context.Background(), CreateShiftChangeInput{
			PublicationID:           1,
			RequesterUserID:         7,
			Type:                    model.ShiftChangeTypeSwap,
			RequesterAssignmentID:   100,
			CounterpartUserID:       &counterpartUserID,
			CounterpartAssignmentID: &counterpartAssignmentID,
		})
		if err != nil {
			t.Fatalf("create setup failed: %v", err)
		}

		if err := svc.CancelShiftChangeRequest(context.Background(), created.ID, 7); err != nil {
			t.Fatalf("CancelShiftChangeRequest returned error: %v", err)
		}
		if sc.requests[created.ID].State != model.ShiftChangeStateCancelled {
			t.Fatalf("expected cancelled, got %s", sc.requests[created.ID].State)
		}
	})

	t.Run("non-requester cancelling is not owner", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		pub, sc := buildShiftChangeFixture(now)
		svc := newTestShiftChangeService(pub, sc, &emailStub{}, now)

		counterpartUserID := int64(8)
		counterpartAssignmentID := int64(101)
		created, err := svc.CreateShiftChangeRequest(context.Background(), CreateShiftChangeInput{
			PublicationID:           1,
			RequesterUserID:         7,
			Type:                    model.ShiftChangeTypeSwap,
			RequesterAssignmentID:   100,
			CounterpartUserID:       &counterpartUserID,
			CounterpartAssignmentID: &counterpartAssignmentID,
		})
		if err != nil {
			t.Fatalf("create setup failed: %v", err)
		}

		err = svc.CancelShiftChangeRequest(context.Background(), created.ID, 8)
		if !errors.Is(err, ErrShiftChangeNotOwner) {
			t.Fatalf("expected ErrShiftChangeNotOwner, got %v", err)
		}
	})
}

// -------- List / Get / Count --------

func TestShiftChangeServiceListGetCount(t *testing.T) {
	t.Run("admin list returns all requests without touching audit", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		pub, sc := buildShiftChangeFixture(now)
		svc := newTestShiftChangeService(pub, sc, &emailStub{}, now)

		counterpartUserID := int64(8)
		counterpartAssignmentID := int64(101)
		if _, err := svc.CreateShiftChangeRequest(context.Background(), CreateShiftChangeInput{
			PublicationID:           1,
			RequesterUserID:         7,
			Type:                    model.ShiftChangeTypeSwap,
			RequesterAssignmentID:   100,
			CounterpartUserID:       &counterpartUserID,
			CounterpartAssignmentID: &counterpartAssignmentID,
		}); err != nil {
			t.Fatalf("create setup failed: %v", err)
		}
		if _, err := svc.CreateShiftChangeRequest(context.Background(), CreateShiftChangeInput{
			PublicationID:         1,
			RequesterUserID:       7,
			Type:                  model.ShiftChangeTypeGivePool,
			RequesterAssignmentID: 100,
		}); err != nil {
			t.Fatalf("create setup failed: %v", err)
		}

		stub := audittest.New()
		ctx := stub.ContextWith(context.Background())

		out, err := svc.ListShiftChangeRequests(ctx, 1, 999, true)
		if err != nil {
			t.Fatalf("ListShiftChangeRequests returned error: %v", err)
		}
		if len(out) != 2 {
			t.Fatalf("expected admin to see 2 requests, got %d", len(out))
		}
		if len(stub.Events()) != 0 {
			t.Fatalf("expected no audit events on list, got %+v", stub.Events())
		}
	})

	t.Run("employee list filters to sent, received, and pool", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		pub, sc := buildShiftChangeFixture(now)
		// Add user 9 who is also mutually qualified.
		pub.qualifiedByUser[9] = map[int64]struct{}{101: {}, 102: {}}
		pub.users[9] = &model.User{ID: 9, Name: "Cora", Email: "cora@example.com", Status: model.UserStatusActive}
		svc := newTestShiftChangeService(pub, sc, &emailStub{}, now)

		// Sent by user 7.
		counterpartUserID := int64(8)
		counterpartAssignmentID := int64(101)
		if _, err := svc.CreateShiftChangeRequest(context.Background(), CreateShiftChangeInput{
			PublicationID:           1,
			RequesterUserID:         7,
			Type:                    model.ShiftChangeTypeSwap,
			RequesterAssignmentID:   100,
			CounterpartUserID:       &counterpartUserID,
			CounterpartAssignmentID: &counterpartAssignmentID,
		}); err != nil {
			t.Fatalf("create setup failed: %v", err)
		}
		// Pool post by user 7.
		if _, err := svc.CreateShiftChangeRequest(context.Background(), CreateShiftChangeInput{
			PublicationID:         1,
			RequesterUserID:       7,
			Type:                  model.ShiftChangeTypeGivePool,
			RequesterAssignmentID: 100,
		}); err != nil {
			t.Fatalf("create setup failed: %v", err)
		}

		// Viewer is user 9: should see only the pool request (no sent, no
		// received).
		out, err := svc.ListShiftChangeRequests(context.Background(), 1, 9, false)
		if err != nil {
			t.Fatalf("ListShiftChangeRequests returned error: %v", err)
		}
		if len(out) != 1 || out[0].Type != model.ShiftChangeTypeGivePool {
			t.Fatalf("expected viewer 9 to see only pool, got %+v", out)
		}

		// Viewer is user 8: should see the swap (received) plus the pool.
		out, err = svc.ListShiftChangeRequests(context.Background(), 1, 8, false)
		if err != nil {
			t.Fatalf("ListShiftChangeRequests returned error: %v", err)
		}
		if len(out) != 2 {
			t.Fatalf("expected viewer 8 to see 2 requests, got %d", len(out))
		}
	})

	t.Run("GetShiftChangeRequest enforces viewer access", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		pub, sc := buildShiftChangeFixture(now)
		// Add user 9 who is not a party to this request.
		pub.users[9] = &model.User{ID: 9, Name: "Cora", Email: "cora@example.com", Status: model.UserStatusActive}
		svc := newTestShiftChangeService(pub, sc, &emailStub{}, now)

		counterpartUserID := int64(8)
		created, err := svc.CreateShiftChangeRequest(context.Background(), CreateShiftChangeInput{
			PublicationID:         1,
			RequesterUserID:       7,
			Type:                  model.ShiftChangeTypeGiveDirect,
			RequesterAssignmentID: 100,
			CounterpartUserID:     &counterpartUserID,
		})
		if err != nil {
			t.Fatalf("create setup failed: %v", err)
		}

		// Requester can view.
		if _, err := svc.GetShiftChangeRequest(context.Background(), created.ID, 7, false); err != nil {
			t.Fatalf("requester should view: %v", err)
		}
		// Counterpart can view.
		if _, err := svc.GetShiftChangeRequest(context.Background(), created.ID, 8, false); err != nil {
			t.Fatalf("counterpart should view: %v", err)
		}
		// Unrelated user is hidden via ErrShiftChangeNotFound.
		_, err = svc.GetShiftChangeRequest(context.Background(), created.ID, 9, false)
		if !errors.Is(err, ErrShiftChangeNotFound) {
			t.Fatalf("expected ErrShiftChangeNotFound for outsider, got %v", err)
		}
		// Admin sees everything.
		if _, err := svc.GetShiftChangeRequest(context.Background(), created.ID, 999, true); err != nil {
			t.Fatalf("admin should view: %v", err)
		}
	})

	t.Run("CountPendingForViewer counts only swap/give_direct with matching counterpart", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		pub, sc := buildShiftChangeFixture(now)
		svc := newTestShiftChangeService(pub, sc, &emailStub{}, now)

		counterpartUserID := int64(8)
		counterpartAssignmentID := int64(101)
		if _, err := svc.CreateShiftChangeRequest(context.Background(), CreateShiftChangeInput{
			PublicationID:           1,
			RequesterUserID:         7,
			Type:                    model.ShiftChangeTypeSwap,
			RequesterAssignmentID:   100,
			CounterpartUserID:       &counterpartUserID,
			CounterpartAssignmentID: &counterpartAssignmentID,
		}); err != nil {
			t.Fatalf("create setup failed: %v", err)
		}
		// Pool request should not be counted (no counterpart).
		if _, err := svc.CreateShiftChangeRequest(context.Background(), CreateShiftChangeInput{
			PublicationID:         1,
			RequesterUserID:       7,
			Type:                  model.ShiftChangeTypeGivePool,
			RequesterAssignmentID: 100,
		}); err != nil {
			t.Fatalf("create setup failed: %v", err)
		}

		n, err := svc.CountPendingForViewer(context.Background(), 8)
		if err != nil {
			t.Fatalf("CountPendingForViewer returned error: %v", err)
		}
		if n != 1 {
			t.Fatalf("expected count 1, got %d", n)
		}
	})
}

// -------- Time-conflict helpers --------

func TestHasOverlapInSet(t *testing.T) {
	t.Parallel()

	mk := func(weekday int, start, end string) *model.PublicationShift {
		return &model.PublicationShift{Weekday: weekday, StartTime: start, EndTime: end}
	}

	tests := []struct {
		name  string
		input []*model.PublicationShift
		want  bool
	}{
		{
			name:  "non-overlapping same weekday",
			input: []*model.PublicationShift{mk(1, "09:00", "10:00"), mk(1, "11:00", "12:00")},
			want:  false,
		},
		{
			name:  "full overlap same weekday",
			input: []*model.PublicationShift{mk(1, "09:00", "12:00"), mk(1, "09:00", "12:00")},
			want:  true,
		},
		{
			name:  "partial overlap same weekday",
			input: []*model.PublicationShift{mk(1, "09:00", "11:00"), mk(1, "10:00", "12:00")},
			want:  true,
		},
		{
			name:  "boundary end equals start is not overlap",
			input: []*model.PublicationShift{mk(1, "09:00", "10:00"), mk(1, "10:00", "11:00")},
			want:  false,
		},
		{
			name:  "overlap window but different weekday",
			input: []*model.PublicationShift{mk(1, "09:00", "12:00"), mk(2, "09:00", "12:00")},
			want:  false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := hasOverlapInSet(tc.input); got != tc.want {
				t.Fatalf("hasOverlapInSet(%s) = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}
