package service

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/jonathanhu237/rota/backend/internal/audit"
	"github.com/jonathanhu237/rota/backend/internal/audit/audittest"
	"github.com/jonathanhu237/rota/backend/internal/model"
	"github.com/jonathanhu237/rota/backend/internal/repository"
)

type leaveRepositoryStatefulMock struct {
	mu      sync.Mutex
	nextID  int64
	leaves  map[int64]*model.Leave
	request func(id int64) (*model.ShiftChangeRequest, error)
}

func newLeaveRepositoryStatefulMock(request func(id int64) (*model.ShiftChangeRequest, error)) *leaveRepositoryStatefulMock {
	return &leaveRepositoryStatefulMock{
		nextID:  1,
		leaves:  make(map[int64]*model.Leave),
		request: request,
	}
}

func (m *leaveRepositoryStatefulMock) WithTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	return fn(nil)
}

func (m *leaveRepositoryStatefulMock) Insert(
	ctx context.Context,
	tx *sql.Tx,
	params repository.InsertLeaveParams,
) (*model.Leave, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	leave := &model.Leave{
		ID:                   m.nextID,
		UserID:               params.UserID,
		PublicationID:        params.PublicationID,
		ShiftChangeRequestID: params.ShiftChangeRequestID,
		Category:             params.Category,
		Reason:               params.Reason,
		CreatedAt:            params.CreatedAt,
		UpdatedAt:            params.UpdatedAt,
	}
	m.nextID++
	m.leaves[leave.ID] = cloneLeave(leave)
	return cloneLeave(leave), nil
}

func (m *leaveRepositoryStatefulMock) GetByID(
	ctx context.Context,
	id int64,
) (*model.Leave, *model.ShiftChangeRequest, error) {
	m.mu.Lock()
	leave, ok := m.leaves[id]
	m.mu.Unlock()
	if !ok {
		return nil, nil, repository.ErrLeaveNotFound
	}
	req, err := m.request(leave.ShiftChangeRequestID)
	if err != nil {
		return nil, nil, err
	}
	return cloneLeave(leave), req, nil
}

func (m *leaveRepositoryStatefulMock) ListForUser(
	ctx context.Context,
	userID int64,
	page int,
	pageSize int,
) ([]*repository.LeaveWithRequest, error) {
	return m.list(func(leave *model.Leave) bool { return leave.UserID == userID }), nil
}

func (m *leaveRepositoryStatefulMock) ListForPublication(
	ctx context.Context,
	publicationID int64,
	page int,
	pageSize int,
) ([]*repository.LeaveWithRequest, error) {
	return m.list(func(leave *model.Leave) bool { return leave.PublicationID == publicationID }), nil
}

func (m *leaveRepositoryStatefulMock) list(keep func(*model.Leave) bool) []*repository.LeaveWithRequest {
	m.mu.Lock()
	leaves := make([]*model.Leave, 0, len(m.leaves))
	for _, leave := range m.leaves {
		if keep(leave) {
			leaves = append(leaves, cloneLeave(leave))
		}
	}
	m.mu.Unlock()

	sort.Slice(leaves, func(i, j int) bool {
		if !leaves[i].CreatedAt.Equal(leaves[j].CreatedAt) {
			return leaves[i].CreatedAt.After(leaves[j].CreatedAt)
		}
		return leaves[i].ID > leaves[j].ID
	})

	out := make([]*repository.LeaveWithRequest, 0, len(leaves))
	for _, leave := range leaves {
		req, _ := m.request(leave.ShiftChangeRequestID)
		out = append(out, &repository.LeaveWithRequest{Leave: leave, Request: req})
	}
	return out
}

func cloneLeave(leave *model.Leave) *model.Leave {
	if leave == nil {
		return nil
	}
	cloned := *leave
	return &cloned
}

func buildLeaveFixture(now time.Time) (*LeaveService, *publicationRepositoryStatefulMock, *shiftChangeRepositoryStatefulMock, *leaveRepositoryStatefulMock) {
	pub, sc := buildShiftChangeFixture(now)
	pub.publications[1] = activePublication(now)
	shiftService := newTestShiftChangeService(pub, sc, nil, now)
	leaveRepo := newLeaveRepositoryStatefulMock(func(id int64) (*model.ShiftChangeRequest, error) {
		return sc.GetByID(context.Background(), id)
	})
	leaveService := NewLeaveService(leaveRepo, sc, shiftService, pub, fixedClock{now: now})
	return leaveService, pub, sc, leaveRepo
}

func TestLeaveServiceCreate(t *testing.T) {
	t.Run("give_pool creates leave and linked request", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		svc, _, sc, _ := buildLeaveFixture(now)
		stub := audittest.New()
		ctx := stub.ContextWith(context.Background())

		detail, err := svc.Create(ctx, CreateLeaveInput{
			UserID:         7,
			AssignmentID:   100,
			OccurrenceDate: mondayOccurrence(now),
			Type:           model.ShiftChangeTypeGivePool,
			Category:       model.LeaveCategoryPersonal,
			Reason:         "exam",
		})
		if err != nil {
			t.Fatalf("Create returned error: %v", err)
		}
		if detail.Leave.ID == 0 || detail.Request.LeaveID == nil || *detail.Request.LeaveID != detail.Leave.ID {
			t.Fatalf("expected linked leave/request, got %+v", detail)
		}
		if sc.requests[detail.Request.ID].LeaveID == nil || *sc.requests[detail.Request.ID].LeaveID != detail.Leave.ID {
			t.Fatalf("expected persisted request leave id, got %+v", sc.requests[detail.Request.ID])
		}
		if stub.FindByAction(audit.ActionLeaveCreate) == nil {
			t.Fatalf("expected leave.create audit, got %v", stub.Actions())
		}
		if stub.FindByAction(audit.ActionShiftChangeCreate) == nil {
			t.Fatalf("expected shift_change.create audit, got %v", stub.Actions())
		}
	})

	t.Run("swap is rejected", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		svc, _, _, _ := buildLeaveFixture(now)
		_, err := svc.Create(context.Background(), CreateLeaveInput{
			UserID:         7,
			AssignmentID:   100,
			OccurrenceDate: mondayOccurrence(now),
			Type:           model.ShiftChangeTypeSwap,
			Category:       model.LeaveCategorySick,
		})
		if !errors.Is(err, ErrShiftChangeInvalidType) {
			t.Fatalf("expected ErrShiftChangeInvalidType, got %v", err)
		}
	})

	t.Run("requires active publication", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		svc, pub, _, _ := buildLeaveFixture(now)
		pub.publications[1] = publishedPublication(now)
		_, err := svc.Create(context.Background(), CreateLeaveInput{
			UserID:         7,
			AssignmentID:   100,
			OccurrenceDate: mondayOccurrence(now),
			Type:           model.ShiftChangeTypeGivePool,
			Category:       model.LeaveCategorySick,
		})
		if !errors.Is(err, ErrPublicationNotActive) {
			t.Fatalf("expected ErrPublicationNotActive, got %v", err)
		}
	})
}

func TestLeaveServiceCancel(t *testing.T) {
	t.Run("owner cancels pending leave", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		svc, _, sc, _ := buildLeaveFixture(now)
		detail, err := svc.Create(context.Background(), CreateLeaveInput{
			UserID:         7,
			AssignmentID:   100,
			OccurrenceDate: mondayOccurrence(now),
			Type:           model.ShiftChangeTypeGivePool,
			Category:       model.LeaveCategoryPersonal,
		})
		if err != nil {
			t.Fatalf("create leave: %v", err)
		}
		stub := audittest.New()
		if err := svc.Cancel(stub.ContextWith(context.Background()), detail.Leave.ID, 7); err != nil {
			t.Fatalf("Cancel returned error: %v", err)
		}
		if sc.requests[detail.Request.ID].State != model.ShiftChangeStateCancelled {
			t.Fatalf("expected request cancelled, got %+v", sc.requests[detail.Request.ID])
		}
		if stub.FindByAction(audit.ActionLeaveCancel) == nil {
			t.Fatalf("expected leave.cancel audit, got %v", stub.Actions())
		}
	})

	t.Run("non-owner rejected", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		svc, _, _, _ := buildLeaveFixture(now)
		detail, err := svc.Create(context.Background(), CreateLeaveInput{
			UserID:         7,
			AssignmentID:   100,
			OccurrenceDate: mondayOccurrence(now),
			Type:           model.ShiftChangeTypeGivePool,
			Category:       model.LeaveCategoryPersonal,
		})
		if err != nil {
			t.Fatalf("create leave: %v", err)
		}
		err = svc.Cancel(context.Background(), detail.Leave.ID, 8)
		if !errors.Is(err, ErrLeaveNotOwner) {
			t.Fatalf("expected ErrLeaveNotOwner, got %v", err)
		}
	})

	t.Run("terminal request is no-op", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		svc, _, sc, _ := buildLeaveFixture(now)
		detail, err := svc.Create(context.Background(), CreateLeaveInput{
			UserID:         7,
			AssignmentID:   100,
			OccurrenceDate: mondayOccurrence(now),
			Type:           model.ShiftChangeTypeGivePool,
			Category:       model.LeaveCategoryPersonal,
		})
		if err != nil {
			t.Fatalf("create leave: %v", err)
		}
		sc.requests[detail.Request.ID].State = model.ShiftChangeStateApproved
		stub := audittest.New()
		if err := svc.Cancel(stub.ContextWith(context.Background()), detail.Leave.ID, 7); err != nil {
			t.Fatalf("Cancel returned error: %v", err)
		}
		if stub.FindByAction(audit.ActionLeaveCancel) != nil {
			t.Fatalf("expected no leave.cancel audit for no-op, got %v", stub.Actions())
		}
	})
}

func TestLeaveServiceReadListAndPreview(t *testing.T) {
	t.Run("get missing maps to leave not found", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		svc, _, _, _ := buildLeaveFixture(now)
		_, err := svc.GetByID(context.Background(), 99)
		if !errors.Is(err, ErrLeaveNotFound) {
			t.Fatalf("expected ErrLeaveNotFound, got %v", err)
		}
	})

	t.Run("list for user returns own leaves", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		svc, _, _, _ := buildLeaveFixture(now)
		own, err := svc.Create(context.Background(), CreateLeaveInput{
			UserID:         7,
			AssignmentID:   100,
			OccurrenceDate: mondayOccurrence(now),
			Type:           model.ShiftChangeTypeGivePool,
			Category:       model.LeaveCategoryPersonal,
		})
		if err != nil {
			t.Fatalf("create own leave: %v", err)
		}
		rows, err := svc.ListForUser(context.Background(), 7, ListLeavesInput{})
		if err != nil {
			t.Fatalf("ListForUser returned error: %v", err)
		}
		if len(rows) != 1 || rows[0].Leave.ID != own.Leave.ID {
			t.Fatalf("expected own leave only, got %+v", rows)
		}
	})

	t.Run("preview returns future active occurrences", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		svc, _, _, _ := buildLeaveFixture(now)
		rows, err := svc.PreviewOccurrences(context.Background(), 7, now, now.AddDate(0, 0, 14))
		if err != nil {
			t.Fatalf("PreviewOccurrences returned error: %v", err)
		}
		if len(rows) == 0 || rows[0].AssignmentID != 100 {
			t.Fatalf("expected preview occurrence for assignment 100, got %+v", rows)
		}
	})

	t.Run("preview without active publication returns empty", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		svc, pub, _, _ := buildLeaveFixture(now)
		pub.publications[1] = publishedPublication(now)
		rows, err := svc.PreviewOccurrences(context.Background(), 7, now, now.AddDate(0, 0, 14))
		if err != nil {
			t.Fatalf("PreviewOccurrences returned error: %v", err)
		}
		if len(rows) != 0 {
			t.Fatalf("expected empty preview, got %+v", rows)
		}
	})

	t.Run("preview rejects inverted range", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		svc, _, _, _ := buildLeaveFixture(now)
		_, err := svc.PreviewOccurrences(context.Background(), 7, now.AddDate(0, 0, 2), now)
		if !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("expected ErrInvalidInput, got %v", err)
		}
	})
}
