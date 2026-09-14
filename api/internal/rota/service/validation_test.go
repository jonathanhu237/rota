package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jonathanhu237/rota/api/internal/rota/model"
	"github.com/jonathanhu237/rota/api/internal/rota/repository"
)

type positionServiceFakeRepo struct {
	positions   []*model.Position
	listTotal   int
	getErr      error
	createErr   error
	updateErr   error
	deleteErr   error
	lastCreate  repository.CreatePositionParams
	lastUpdate  repository.UpdatePositionParams
	deleteCalls []int64
}

func (f *positionServiceFakeRepo) ListPaginated(context.Context, repository.ListPositionsParams) ([]*model.Position, int, error) {
	return f.positions, f.listTotal, nil
}

func (f *positionServiceFakeRepo) GetByID(_ context.Context, id int64) (*model.Position, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	for _, position := range f.positions {
		if position.ID == id {
			return position, nil
		}
	}
	return nil, repository.ErrPositionNotFound
}

func (f *positionServiceFakeRepo) Create(_ context.Context, params repository.CreatePositionParams) (*model.Position, error) {
	f.lastCreate = params
	if f.createErr != nil {
		return nil, f.createErr
	}
	return &model.Position{ID: 2, Name: params.Name, Description: params.Description}, nil
}

func (f *positionServiceFakeRepo) Update(_ context.Context, params repository.UpdatePositionParams) (*model.Position, error) {
	f.lastUpdate = params
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	return &model.Position{ID: params.ID, Name: params.Name, Description: params.Description}, nil
}

func (f *positionServiceFakeRepo) Delete(_ context.Context, id int64) error {
	f.deleteCalls = append(f.deleteCalls, id)
	return f.deleteErr
}

func positionForTest(id int64) *model.Position {
	return &model.Position{
		ID:          id,
		Name:        "Nurse",
		Description: "Ward",
		CreatedAt:   time.Unix(1, 0).UTC(),
		UpdatedAt:   time.Unix(1, 0).UTC(),
	}
}

func TestPositionServiceCRUDAndNormalization(t *testing.T) {
	repo := &positionServiceFakeRepo{positions: []*model.Position{positionForTest(1)}, listTotal: 11}
	service := NewPositionService(repo)

	list, err := service.ListPositions(context.Background(), ListPositionsInput{})
	if err != nil || list.Page != 1 || list.PageSize != 10 || list.TotalPages != 2 {
		t.Fatalf("ListPositions() = %#v, %v", list, err)
	}

	got, err := service.GetPositionByID(context.Background(), 1)
	if err != nil || got.ID != 1 {
		t.Fatalf("GetPositionByID() = %#v, %v", got, err)
	}

	created, err := service.CreatePosition(context.Background(), CreatePositionInput{Name: "  Doctor ", Description: "  On call  "})
	if err != nil || created.Name != "Doctor" || created.Description != "On call" {
		t.Fatalf("CreatePosition() = %#v, %v", created, err)
	}
	if repo.lastCreate.Name != "Doctor" || repo.lastCreate.Description != "On call" {
		t.Fatalf("create params = %#v", repo.lastCreate)
	}

	updated, err := service.UpdatePosition(context.Background(), UpdatePositionInput{ID: 1, Name: "  Senior Nurse ", Description: "  ICU "})
	if err != nil || updated.Name != "Senior Nurse" || updated.Description != "ICU" {
		t.Fatalf("UpdatePosition() = %#v, %v", updated, err)
	}
	if err := service.DeletePosition(context.Background(), 1); err != nil || len(repo.deleteCalls) != 1 || repo.deleteCalls[0] != 1 {
		t.Fatalf("DeletePosition() = %v, calls=%v", err, repo.deleteCalls)
	}
}

func TestPositionServiceRejectsInvalidAndMapsRepositoryErrors(t *testing.T) {
	service := NewPositionService(&positionServiceFakeRepo{positions: []*model.Position{positionForTest(1)}})
	for _, test := range []struct {
		name string
		call func() error
	}{
		{name: "blank create name", call: func() error {
			_, err := service.CreatePosition(context.Background(), CreatePositionInput{Name: "  "})
			return err
		}},
		{name: "invalid get id", call: func() error {
			_, err := service.GetPositionByID(context.Background(), 0)
			return err
		}},
		{name: "invalid delete id", call: func() error { return service.DeletePosition(context.Background(), 0) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := test.call()
			if !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("error = %v, want ErrInvalidInput", err)
			}
		})
	}

	notFound := NewPositionService(&positionServiceFakeRepo{getErr: repository.ErrPositionNotFound})
	if _, err := notFound.GetPositionByID(context.Background(), 1); !errors.Is(err, ErrPositionNotFound) {
		t.Fatalf("mapped get error = %v, want ErrPositionNotFound", err)
	}
	inUse := NewPositionService(&positionServiceFakeRepo{
		positions: []*model.Position{positionForTest(1)},
		deleteErr: repository.ErrPositionInUse,
	})
	if err := inUse.DeletePosition(context.Background(), 1); !errors.Is(err, ErrPositionInUse) {
		t.Fatalf("mapped delete error = %v, want ErrPositionInUse", err)
	}
}

func TestUserIDValidationRequiresCanonicalUUID(t *testing.T) {
	valid := "019535d9-3df7-79fb-b466-fa907fa17f9e"
	for _, id := range []string{"", "1", "019535D9-3df7-79fb-b466-fa907fa17f9e", valid + " ", "019535d9-3df7-79fb-b466-fa907fa17f9"} {
		if validUserID(id) {
			t.Errorf("validUserID(%q) = true, want false", id)
		}
	}
	if !validUserID(valid) {
		t.Fatal("valid canonical UUID rejected")
	}
}

func TestNormalizePaginationRejectsNegativeAndCapsPageSize(t *testing.T) {
	if _, _, err := normalizePagination(-1, 10); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("negative page error = %v, want ErrInvalidInput", err)
	}
	page, pageSize, err := normalizePagination(2, 1000)
	if err != nil || page != 2 || pageSize != 100 {
		t.Fatalf("normalizePagination() = (%d, %d, %v), want (2, 100, nil)", page, pageSize, err)
	}
}

type userPositionServiceFakeRepo struct {
	positions       []*model.Position
	listErr         error
	replaceErr      error
	lastPositionIDs []int64
}

func (f *userPositionServiceFakeRepo) ListPositionsByUserID(context.Context, string) ([]*model.Position, error) {
	return f.positions, f.listErr
}

func (f *userPositionServiceFakeRepo) ReplacePositionsByUserID(_ context.Context, _ string, ids []int64) error {
	f.lastPositionIDs = append([]int64(nil), ids...)
	return f.replaceErr
}

func TestUserPositionServiceListsAndReplacesNormalizedQualifications(t *testing.T) {
	position := positionForTest(2)
	repo := &userPositionServiceFakeRepo{positions: []*model.Position{position}}
	service := NewUserPositionService(repo)
	const userID = "019535d9-3df7-79fb-b466-fa907fa17f9e"

	positions, err := service.ListUserPositions(context.Background(), userID)
	if err != nil || len(positions) != 1 || positions[0].ID != 2 {
		t.Fatalf("ListUserPositions() = %#v, %v", positions, err)
	}
	if err := service.ReplaceUserPositions(context.Background(), ReplaceUserPositionsInput{UserID: userID, PositionIDs: []int64{3, 1, 3}}); err != nil {
		t.Fatalf("ReplaceUserPositions() error = %v", err)
	}
	if !equalInt64s(repo.lastPositionIDs, []int64{1, 3}) {
		t.Fatalf("normalized position IDs = %#v, want [1 3]", repo.lastPositionIDs)
	}
}

func TestUserPositionServiceRejectsInvalidAndMapsQualificationErrors(t *testing.T) {
	service := NewUserPositionService(&userPositionServiceFakeRepo{})
	if _, err := service.ListUserPositions(context.Background(), "not-a-uuid"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid user list error = %v, want ErrInvalidInput", err)
	}
	if err := service.ReplaceUserPositions(context.Background(), ReplaceUserPositionsInput{
		UserID: "019535d9-3df7-79fb-b466-fa907fa17f9e", PositionIDs: []int64{0},
	}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid position replacement error = %v, want ErrInvalidInput", err)
	}

	notFound := NewUserPositionService(&userPositionServiceFakeRepo{listErr: repository.ErrUserNotFound})
	if _, err := notFound.ListUserPositions(context.Background(), "019535d9-3df7-79fb-b466-fa907fa17f9e"); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("mapped user error = %v, want ErrUserNotFound", err)
	}
	missingPosition := NewUserPositionService(&userPositionServiceFakeRepo{replaceErr: repository.ErrPositionNotFound})
	if err := missingPosition.ReplaceUserPositions(context.Background(), ReplaceUserPositionsInput{
		UserID: "019535d9-3df7-79fb-b466-fa907fa17f9e", PositionIDs: []int64{1},
	}); !errors.Is(err, ErrPositionNotFound) {
		t.Fatalf("mapped position error = %v, want ErrPositionNotFound", err)
	}
}

func equalInt64s(left, right []int64) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
