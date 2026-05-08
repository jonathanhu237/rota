package service

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"strconv"
	"strings"
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
	row, err := m.GetWithRequestByID(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	return row.Leave, row.Request, nil
}

func (m *leaveRepositoryStatefulMock) GetWithRequestByID(
	ctx context.Context,
	id int64,
) (*repository.LeaveWithRequest, error) {
	m.mu.Lock()
	leave, ok := m.leaves[id]
	m.mu.Unlock()
	if !ok {
		return nil, repository.ErrLeaveNotFound
	}
	return m.rowForLeave(cloneLeave(leave))
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

func (m *leaveRepositoryStatefulMock) ListPool(
	ctx context.Context,
	params repository.ListLeavePoolParams,
) ([]*repository.LeaveWithRequest, int, error) {
	rows := m.list(func(leave *model.Leave) bool {
		req, err := m.request(leave.ShiftChangeRequestID)
		if err != nil {
			return false
		}
		if !params.ViewerIsAdmin &&
			req.Type != model.ShiftChangeTypeGivePool &&
			leave.UserID != params.ViewerUserID &&
			(req.CounterpartUserID == nil || *req.CounterpartUserID != params.ViewerUserID) {
			return false
		}
		state := model.LeaveStateFromSCRT(req.State)
		return params.State == model.LeavePoolStateAll || string(state) == string(params.State)
	})
	sort.Slice(rows, func(i, j int) bool {
		leftState := model.LeaveStateFromSCRT(rows[i].Request.State)
		rightState := model.LeaveStateFromSCRT(rows[j].Request.State)
		if params.State == model.LeavePoolStatePending || params.State == model.LeavePoolStateAll {
			if leftState == model.LeaveStatePending && rightState == model.LeaveStatePending &&
				!rows[i].Request.ExpiresAt.Equal(rows[j].Request.ExpiresAt) {
				return rows[i].Request.ExpiresAt.Before(rows[j].Request.ExpiresAt)
			}
			if params.State == model.LeavePoolStateAll && leftState != rightState {
				return leftState == model.LeaveStatePending
			}
		}
		if !rows[i].Leave.CreatedAt.Equal(rows[j].Leave.CreatedAt) {
			return rows[i].Leave.CreatedAt.After(rows[j].Leave.CreatedAt)
		}
		return rows[i].Leave.ID > rows[j].Leave.ID
	})
	total := len(rows)
	start := params.Offset
	if start > total {
		start = total
	}
	end := start + params.Limit
	if end > total {
		end = total
	}
	return rows[start:end], total, nil
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
		row, _ := m.rowForLeave(leave)
		out = append(out, row)
	}
	return out
}

func (m *leaveRepositoryStatefulMock) rowForLeave(leave *model.Leave) (*repository.LeaveWithRequest, error) {
	req, err := m.request(leave.ShiftChangeRequestID)
	if err != nil {
		return nil, err
	}
	var counterpartName *string
	if req.CounterpartUserID != nil {
		name := "Bob"
		counterpartName = &name
	}
	var substituteName *string
	if req.DecidedByUserID != nil && req.State == model.ShiftChangeStateApproved {
		name := "Bob"
		substituteName = &name
	}
	return &repository.LeaveWithRequest{
		Leave:           leave,
		Request:         req,
		RequesterName:   "Alice",
		CounterpartName: counterpartName,
		SubstituteName:  substituteName,
		Shift: &repository.LeaveShiftContext{
			AssignmentID:    req.RequesterAssignmentID,
			SlotID:          21,
			Weekday:         1,
			StartTime:       "09:00",
			EndTime:         "12:00",
			PositionID:      101,
			PositionName:    "Front Desk",
			OccurrenceStart: req.ExpiresAt,
			OccurrenceEnd:   req.ExpiresAt.Add(3 * time.Hour),
		},
	}, nil
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
		_, err := svc.GetByID(context.Background(), 99, 7, false)
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

func TestLeavePoolServiceActions(t *testing.T) {
	t.Run("qualified employee can claim public leave", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		svc, _, _, _ := buildLeaveFixture(now)
		if _, err := svc.Create(context.Background(), CreateLeaveInput{
			UserID:         7,
			AssignmentID:   100,
			OccurrenceDate: mondayOccurrence(now),
			Type:           model.ShiftChangeTypeGivePool,
			Category:       model.LeaveCategoryPersonal,
		}); err != nil {
			t.Fatalf("create leave: %v", err)
		}

		result, err := svc.ListPool(context.Background(), 8, false, ListLeavePoolInput{})
		if err != nil {
			t.Fatalf("ListPool returned error: %v", err)
		}
		if result.Page != 1 || result.PageSize != 20 || result.TotalCount != 1 {
			t.Fatalf("unexpected pagination: %+v", result)
		}
		if len(result.Leaves) != 1 || !result.Leaves[0].Actions.CanClaim {
			t.Fatalf("expected claim action, got %+v", result.Leaves)
		}
	})

	t.Run("unqualified public row is visible but disabled", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		svc, pub, _, _ := buildLeaveFixture(now)
		delete(pub.qualifiedByUser, 8)
		if _, err := svc.Create(context.Background(), CreateLeaveInput{
			UserID:         7,
			AssignmentID:   100,
			OccurrenceDate: mondayOccurrence(now),
			Type:           model.ShiftChangeTypeGivePool,
			Category:       model.LeaveCategoryPersonal,
		}); err != nil {
			t.Fatalf("create leave: %v", err)
		}

		result, err := svc.ListPool(context.Background(), 8, false, ListLeavePoolInput{})
		if err != nil {
			t.Fatalf("ListPool returned error: %v", err)
		}
		if len(result.Leaves) != 1 ||
			result.Leaves[0].Actions.CanClaim ||
			result.Leaves[0].Actions.DisabledReason != model.LeaveActionDisabledNotQualified {
			t.Fatalf("expected not-qualified disabled row, got %+v", result.Leaves)
		}
	})

	t.Run("requester counterpart admin and completed substitute actions", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		svc, _, sc, _ := buildLeaveFixture(now)
		counterpartID := int64(8)
		detail, err := svc.Create(context.Background(), CreateLeaveInput{
			UserID:            7,
			AssignmentID:      100,
			OccurrenceDate:    mondayOccurrence(now),
			Type:              model.ShiftChangeTypeGiveDirect,
			CounterpartUserID: &counterpartID,
			Category:          model.LeaveCategoryPersonal,
		})
		if err != nil {
			t.Fatalf("create leave: %v", err)
		}

		own, err := svc.ListPool(context.Background(), 7, false, ListLeavePoolInput{})
		if err != nil {
			t.Fatalf("ListPool requester: %v", err)
		}
		if len(own.Leaves) != 1 || !own.Leaves[0].Actions.CanCancel {
			t.Fatalf("expected requester cancel action, got %+v", own.Leaves)
		}

		counterpart, err := svc.ListPool(context.Background(), 8, false, ListLeavePoolInput{})
		if err != nil {
			t.Fatalf("ListPool counterpart: %v", err)
		}
		if len(counterpart.Leaves) != 1 ||
			!counterpart.Leaves[0].Actions.CanApprove ||
			!counterpart.Leaves[0].Actions.CanReject {
			t.Fatalf("expected counterpart approve/reject, got %+v", counterpart.Leaves)
		}

		admin, err := svc.ListPool(context.Background(), 99, true, ListLeavePoolInput{})
		if err != nil {
			t.Fatalf("ListPool admin: %v", err)
		}
		if len(admin.Leaves) != 1 ||
			admin.Leaves[0].Actions.CanApprove ||
			admin.Leaves[0].Actions.DisabledReason != model.LeaveActionDisabledAdminViewOnly {
			t.Fatalf("expected admin view-only row, got %+v", admin.Leaves)
		}

		decider := int64(8)
		sc.requests[detail.Request.ID].State = model.ShiftChangeStateApproved
		sc.requests[detail.Request.ID].DecidedByUserID = &decider
		completed, err := svc.ListPool(context.Background(), 7, false, ListLeavePoolInput{State: "completed"})
		if err != nil {
			t.Fatalf("ListPool completed: %v", err)
		}
		if len(completed.Leaves) != 1 || completed.Leaves[0].SubstituteName == nil || *completed.Leaves[0].SubstituteName != "Bob" {
			t.Fatalf("expected completed substitute display, got %+v", completed.Leaves)
		}
	})

	t.Run("rejects invalid state and pagination", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		svc, _, _, _ := buildLeaveFixture(now)
		if _, err := svc.ListPool(context.Background(), 8, false, ListLeavePoolInput{State: "unknown"}); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("expected invalid state error, got %v", err)
		}
		if _, err := svc.ListPool(context.Background(), 8, false, ListLeavePoolInput{Page: -1}); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("expected invalid pagination error, got %v", err)
		}
	})
}

func TestLeavePreviewDirectCandidates(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
	svc, _, _, _ := buildLeaveFixture(now)
	rows, err := svc.PreviewOccurrences(context.Background(), 7, now, now.AddDate(0, 0, 14))
	if err != nil {
		t.Fatalf("PreviewOccurrences returned error: %v", err)
	}
	if len(rows) == 0 {
		t.Fatalf("expected preview occurrence")
	}
	if len(rows[0].DirectCandidates) != 1 ||
		rows[0].DirectCandidates[0].UserID != 8 ||
		rows[0].DirectCandidates[0].Name != "Bob" {
		t.Fatalf("expected Bob as direct candidate only, got %+v", rows[0].DirectCandidates)
	}
}

func TestLeaveServiceCreationEmails(t *testing.T) {
	t.Run("direct leave sends counterpart email with leave link", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		pub, sc := buildShiftChangeFixture(now)
		pub.publications[1] = activePublication(now)
		emailer := &emailStub{}
		shiftService := newTestShiftChangeService(pub, sc, emailer, now)
		leaveRepo := newLeaveRepositoryStatefulMock(func(id int64) (*model.ShiftChangeRequest, error) {
			return sc.GetByID(context.Background(), id)
		})
		svc := NewLeaveService(leaveRepo, sc, shiftService, pub, fixedClock{now: now})
		counterpartID := int64(8)
		detail, err := svc.Create(context.Background(), CreateLeaveInput{
			UserID:            7,
			AssignmentID:      100,
			OccurrenceDate:    mondayOccurrence(now),
			Type:              model.ShiftChangeTypeGiveDirect,
			CounterpartUserID: &counterpartID,
			Category:          model.LeaveCategoryPersonal,
		})
		if err != nil {
			t.Fatalf("Create returned error: %v", err)
		}
		msgs := emailer.messages()
		if len(msgs) != 1 {
			t.Fatalf("expected direct leave creation email, got %d", len(msgs))
		}
		if !strings.Contains(msgs[0].Body, "/leaves/"+strconv.FormatInt(detail.Leave.ID, 10)) {
			t.Fatalf("expected leave detail link in email body, got %q", msgs[0].Body)
		}
	})

	t.Run("public leave sends no creation email", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
		pub, sc := buildShiftChangeFixture(now)
		pub.publications[1] = activePublication(now)
		emailer := &emailStub{}
		shiftService := newTestShiftChangeService(pub, sc, emailer, now)
		leaveRepo := newLeaveRepositoryStatefulMock(func(id int64) (*model.ShiftChangeRequest, error) {
			return sc.GetByID(context.Background(), id)
		})
		svc := NewLeaveService(leaveRepo, sc, shiftService, pub, fixedClock{now: now})
		if _, err := svc.Create(context.Background(), CreateLeaveInput{
			UserID:         7,
			AssignmentID:   100,
			OccurrenceDate: mondayOccurrence(now),
			Type:           model.ShiftChangeTypeGivePool,
			Category:       model.LeaveCategoryPersonal,
		}); err != nil {
			t.Fatalf("Create returned error: %v", err)
		}
		if len(emailer.messages()) != 0 {
			t.Fatalf("expected no public leave creation email, got %+v", emailer.messages())
		}
	})
}
