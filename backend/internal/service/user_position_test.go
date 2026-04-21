package service

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/jonathanhu237/rota/backend/internal/audit"
	"github.com/jonathanhu237/rota/backend/internal/audit/audittest"
	"github.com/jonathanhu237/rota/backend/internal/model"
	"github.com/jonathanhu237/rota/backend/internal/repository"
)

type userPositionRepositoryStatefulMock struct {
	positionsByUser map[int64][]int64
	positions       map[int64]*model.Position
	users           map[int64]struct{}
}

func (m *userPositionRepositoryStatefulMock) ListPositionsByUserID(ctx context.Context, userID int64) ([]*model.Position, error) {
	if _, ok := m.users[userID]; !ok {
		return nil, repository.ErrUserNotFound
	}

	positionIDs := append([]int64(nil), m.positionsByUser[userID]...)
	sort.Slice(positionIDs, func(i, j int) bool {
		return positionIDs[i] < positionIDs[j]
	})

	positions := make([]*model.Position, 0, len(positionIDs))
	for _, positionID := range positionIDs {
		positions = append(positions, m.positions[positionID])
	}

	return positions, nil
}

func (m *userPositionRepositoryStatefulMock) ReplacePositionsByUserID(ctx context.Context, userID int64, positionIDs []int64) error {
	if _, ok := m.users[userID]; !ok {
		return repository.ErrUserNotFound
	}

	nextPositionIDs := make([]int64, 0, len(positionIDs))
	seen := make(map[int64]struct{}, len(positionIDs))
	for _, positionID := range positionIDs {
		if _, ok := m.positions[positionID]; !ok {
			return repository.ErrPositionNotFound
		}
		if _, ok := seen[positionID]; ok {
			continue
		}
		seen[positionID] = struct{}{}
		nextPositionIDs = append(nextPositionIDs, positionID)
	}

	sort.Slice(nextPositionIDs, func(i, j int) bool {
		return nextPositionIDs[i] < nextPositionIDs[j]
	})
	m.positionsByUser[userID] = nextPositionIDs

	return nil
}

func TestUserPositionServiceListUserPositions(t *testing.T) {
	t.Run("returns empty list when user has no qualifications", func(t *testing.T) {
		t.Parallel()

		service := NewUserPositionService(&userPositionRepositoryStatefulMock{
			users:           map[int64]struct{}{1: {}},
			positions:       make(map[int64]*model.Position),
			positionsByUser: make(map[int64][]int64),
		})

		positions, err := service.ListUserPositions(context.Background(), 1)
		if err != nil {
			t.Fatalf("ListUserPositions returned error: %v", err)
		}
		if len(positions) != 0 {
			t.Fatalf("expected no positions, got %d", len(positions))
		}
	})

	t.Run("returns qualified positions", func(t *testing.T) {
		t.Parallel()

		repo := newUserPositionRepositoryStatefulMock()
		repo.positionsByUser[1] = []int64{2, 1}
		service := NewUserPositionService(repo)

		positions, err := service.ListUserPositions(context.Background(), 1)
		if err != nil {
			t.Fatalf("ListUserPositions returned error: %v", err)
		}
		if len(positions) != 2 {
			t.Fatalf("expected 2 positions, got %d", len(positions))
		}
		if positions[0].ID != 1 || positions[1].ID != 2 {
			t.Fatalf("expected positions ordered by ID, got %+v", positions)
		}
	})

	t.Run("returns ErrUserNotFound for nonexistent user", func(t *testing.T) {
		t.Parallel()

		service := NewUserPositionService(newUserPositionRepositoryStatefulMock())

		_, err := service.ListUserPositions(context.Background(), 999)
		if !errors.Is(err, ErrUserNotFound) {
			t.Fatalf("expected ErrUserNotFound, got %v", err)
		}
	})
}

func TestUserPositionServiceReplaceUserPositions(t *testing.T) {
	t.Run("replaces empty set with non-empty set", func(t *testing.T) {
		t.Parallel()

		repo := newUserPositionRepositoryStatefulMock()
		service := NewUserPositionService(repo)

		stub := audittest.New()
		ctx := stub.ContextWith(context.Background())
		err := service.ReplaceUserPositions(ctx, ReplaceUserPositionsInput{
			UserID:      1,
			PositionIDs: []int64{1, 2},
		})
		if err != nil {
			t.Fatalf("ReplaceUserPositions returned error: %v", err)
		}

		assertUserPositionIDs(t, repo.positionsByUser[1], []int64{1, 2})
		assertQualificationsAudit(t, stub, 1, []int64{1, 2})
	})

	t.Run("replaces non-empty set with smaller set", func(t *testing.T) {
		t.Parallel()

		repo := newUserPositionRepositoryStatefulMock()
		repo.positionsByUser[1] = []int64{1, 2, 3}
		service := NewUserPositionService(repo)

		stub := audittest.New()
		ctx := stub.ContextWith(context.Background())
		err := service.ReplaceUserPositions(ctx, ReplaceUserPositionsInput{
			UserID:      1,
			PositionIDs: []int64{2},
		})
		if err != nil {
			t.Fatalf("ReplaceUserPositions returned error: %v", err)
		}

		assertUserPositionIDs(t, repo.positionsByUser[1], []int64{2})
		assertQualificationsAudit(t, stub, 1, []int64{2})
	})

	t.Run("replaces non-empty set with larger overlapping set", func(t *testing.T) {
		t.Parallel()

		repo := newUserPositionRepositoryStatefulMock()
		repo.positionsByUser[1] = []int64{1, 2}
		service := NewUserPositionService(repo)

		stub := audittest.New()
		ctx := stub.ContextWith(context.Background())
		err := service.ReplaceUserPositions(ctx, ReplaceUserPositionsInput{
			UserID:      1,
			PositionIDs: []int64{2, 3, 4},
		})
		if err != nil {
			t.Fatalf("ReplaceUserPositions returned error: %v", err)
		}

		assertUserPositionIDs(t, repo.positionsByUser[1], []int64{2, 3, 4})
		assertQualificationsAudit(t, stub, 1, []int64{2, 3, 4})
	})

	t.Run("replaces non-empty set with empty set", func(t *testing.T) {
		t.Parallel()

		repo := newUserPositionRepositoryStatefulMock()
		repo.positionsByUser[1] = []int64{1, 2}
		service := NewUserPositionService(repo)

		stub := audittest.New()
		ctx := stub.ContextWith(context.Background())
		err := service.ReplaceUserPositions(ctx, ReplaceUserPositionsInput{
			UserID:      1,
			PositionIDs: []int64{},
		})
		if err != nil {
			t.Fatalf("ReplaceUserPositions returned error: %v", err)
		}

		assertUserPositionIDs(t, repo.positionsByUser[1], []int64{})
		assertQualificationsAudit(t, stub, 1, []int64{})
	})

	t.Run("dedupes duplicate position IDs", func(t *testing.T) {
		t.Parallel()

		repo := newUserPositionRepositoryStatefulMock()
		service := NewUserPositionService(repo)

		stub := audittest.New()
		ctx := stub.ContextWith(context.Background())
		err := service.ReplaceUserPositions(ctx, ReplaceUserPositionsInput{
			UserID:      1,
			PositionIDs: []int64{2, 1, 2, 1},
		})
		if err != nil {
			t.Fatalf("ReplaceUserPositions returned error: %v", err)
		}

		assertUserPositionIDs(t, repo.positionsByUser[1], []int64{1, 2})
		assertQualificationsAudit(t, stub, 1, []int64{1, 2})
	})

	t.Run("returns ErrPositionNotFound without partial write when any position is missing", func(t *testing.T) {
		t.Parallel()

		repo := newUserPositionRepositoryStatefulMock()
		repo.positionsByUser[1] = []int64{1, 2}
		service := NewUserPositionService(repo)

		stub := audittest.New()
		ctx := stub.ContextWith(context.Background())
		err := service.ReplaceUserPositions(ctx, ReplaceUserPositionsInput{
			UserID:      1,
			PositionIDs: []int64{2, 999},
		})
		if !errors.Is(err, ErrPositionNotFound) {
			t.Fatalf("expected ErrPositionNotFound, got %v", err)
		}

		assertUserPositionIDs(t, repo.positionsByUser[1], []int64{1, 2})
		if len(stub.Events()) != 0 {
			t.Fatalf("expected no audit events on error, got %v", stub.Actions())
		}
	})

	t.Run("returns ErrUserNotFound for nonexistent user", func(t *testing.T) {
		t.Parallel()

		service := NewUserPositionService(newUserPositionRepositoryStatefulMock())

		stub := audittest.New()
		ctx := stub.ContextWith(context.Background())
		err := service.ReplaceUserPositions(ctx, ReplaceUserPositionsInput{
			UserID:      999,
			PositionIDs: []int64{1},
		})
		if !errors.Is(err, ErrUserNotFound) {
			t.Fatalf("expected ErrUserNotFound, got %v", err)
		}
		if len(stub.Events()) != 0 {
			t.Fatalf("expected no audit events on error, got %v", stub.Actions())
		}
	})
}

// assertQualificationsAudit checks the user.qualifications.replace audit event
// was recorded with the expected target user and final sorted position IDs.
func assertQualificationsAudit(t *testing.T, stub *audittest.Stub, userID int64, wantPositionIDs []int64) {
	t.Helper()

	event := stub.FindByAction(audit.ActionUserQualificationsReplace)
	if event == nil {
		t.Fatalf("expected %q audit event, got %v", audit.ActionUserQualificationsReplace, stub.Actions())
	}
	if event.TargetType != audit.TargetTypeUser {
		t.Fatalf("unexpected target type: %q", event.TargetType)
	}
	if event.TargetID == nil || *event.TargetID != userID {
		t.Fatalf("unexpected target id: %v", event.TargetID)
	}
	got, ok := event.Metadata["position_ids"].([]int64)
	if !ok {
		t.Fatalf("expected position_ids metadata to be []int64, got %T", event.Metadata["position_ids"])
	}
	if len(got) != len(wantPositionIDs) {
		t.Fatalf("expected position_ids %v, got %v", wantPositionIDs, got)
	}
	for i := range wantPositionIDs {
		if got[i] != wantPositionIDs[i] {
			t.Fatalf("expected position_ids %v, got %v", wantPositionIDs, got)
		}
	}
}

func newUserPositionRepositoryStatefulMock() *userPositionRepositoryStatefulMock {
	now := time.Now()

	return &userPositionRepositoryStatefulMock{
		users: map[int64]struct{}{
			1: {},
		},
		positions: map[int64]*model.Position{
			1: {
				ID:          1,
				Name:        "Front Desk",
				Description: "Handles arrivals",
				CreatedAt:   now,
				UpdatedAt:   now,
			},
			2: {
				ID:          2,
				Name:        "Cashier",
				Description: "Handles payments",
				CreatedAt:   now,
				UpdatedAt:   now,
			},
			3: {
				ID:          3,
				Name:        "Warehouse",
				Description: "Moves inventory",
				CreatedAt:   now,
				UpdatedAt:   now,
			},
			4: {
				ID:          4,
				Name:        "Security",
				Description: "Monitors access",
				CreatedAt:   now,
				UpdatedAt:   now,
			},
		},
		positionsByUser: make(map[int64][]int64),
	}
}

func assertUserPositionIDs(t *testing.T, got, want []int64) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected %v, got %v", want, got)
		}
	}
}
