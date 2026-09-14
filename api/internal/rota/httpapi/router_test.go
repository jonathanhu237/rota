package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jonathanhu237/rota/api/internal/auth/domain"
	"github.com/jonathanhu237/rota/api/internal/rota/model"
	"github.com/jonathanhu237/rota/api/internal/rota/service"
)

const routerTestUserID = "019535d9-3df7-79fb-b466-fa907fa17f9e"

type routerPrincipalAuth struct {
	principals map[string]domain.Principal
}

func (a routerPrincipalAuth) CurrentPrincipal(_ context.Context, session string) (domain.Principal, error) {
	principal, ok := a.principals[session]
	if !ok {
		return domain.Principal{}, context.Canceled
	}
	return principal, nil
}

type routerPositionService struct {
	listCalls   int
	createCalls int
}

type routerPublicationService struct {
	publicationService
	currentCalls     int
	submissionUserID string
}

func (s *routerPublicationService) CreateAvailabilitySubmission(_ context.Context, input service.CreateAvailabilitySubmissionInput) (*model.AvailabilitySubmission, error) {
	s.submissionUserID = input.UserID
	return &model.AvailabilitySubmission{ID: 1, UserID: input.UserID}, nil
}

func (s *routerPublicationService) GetCurrentPublication(context.Context) (*model.Publication, error) {
	s.currentCalls++
	return &model.Publication{ID: 7, Name: "Current rota", State: model.PublicationStatePublished}, nil
}

func (s *routerPositionService) ListPositions(context.Context, service.ListPositionsInput) (*service.ListPositionsResult, error) {
	s.listCalls++
	return &service.ListPositionsResult{Positions: []*model.Position{}, Page: 1, PageSize: 10}, nil
}

func (*routerPositionService) GetPositionByID(context.Context, int64) (*model.Position, error) {
	return &model.Position{ID: 1, Name: "Nurse"}, nil
}

func (s *routerPositionService) CreatePosition(context.Context, service.CreatePositionInput) (*model.Position, error) {
	s.createCalls++
	return &model.Position{ID: 1, Name: "Nurse"}, nil
}

func (*routerPositionService) UpdatePosition(context.Context, service.UpdatePositionInput) (*model.Position, error) {
	return &model.Position{ID: 1, Name: "Nurse"}, nil
}

func (*routerPositionService) DeletePosition(context.Context, int64) error { return nil }

func routerTestPrincipal(permissions ...domain.PermissionKey) domain.Principal {
	return domain.Principal{
		User:        domain.User{ID: routerTestUserID, Email: "rota@example.com", Name: "Rota user"},
		Permissions: permissions,
	}
}

func routerRequest(method, path, session, origin string) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(`{"name":"Nurse"}`))
	req.AddCookie(&http.Cookie{Name: "session", Value: session})
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	return req
}

func TestRotaUserOnlyTreatsManagementGrantsAsAdmin(t *testing.T) {
	reader := rotaUser(routerTestPrincipal(domain.PermissionRotaRead), false)
	if reader.IsAdmin {
		t.Fatal("read-only principal was marked as admin")
	}

	manager := rotaUser(routerTestPrincipal(domain.PermissionRotaManage), false)
	if !manager.IsAdmin {
		t.Fatal("management principal was not marked as admin")
	}

	managed := rotaUser(routerTestPrincipal(domain.PermissionRotaRead), true)
	if !managed.IsAdmin {
		t.Fatal("managed request was not marked as admin")
	}
}

func TestRouterAllowsEmployeeSelfServiceButDeniesManagementReads(t *testing.T) {
	positionService := &routerPositionService{}
	publicationService := &routerPublicationService{}
	handler := NewRouter(
		routerPrincipalAuth{principals: map[string]domain.Principal{
			"employee": routerTestPrincipal(domain.PermissionRotaSelf),
		}},
		"session",
		nil,
		RotaHandlers{
			Positions:    NewPositionHandler(positionService),
			Publications: NewPublicationHandler(publicationService),
		},
	)

	submission := httptest.NewRequest(http.MethodPost, "/publications/7/submissions", strings.NewReader(`{"slot_id":1,"weekday":1,"user_id":"019535d9-3df7-79fb-b466-fa907fa17f92"}`))
	submission.AddCookie(&http.Cookie{Name: "session", Value: "employee"})
	created := httptest.NewRecorder()
	handler.ServeHTTP(created, submission)
	if created.Code != http.StatusCreated || publicationService.submissionUserID != routerTestUserID {
		t.Fatalf("submission status/owner = %d/%s, want 201/authenticated employee", created.Code, publicationService.submissionUserID)
	}

	selfService := httptest.NewRecorder()
	handler.ServeHTTP(selfService, routerRequest(http.MethodGet, "/publications/current", "employee", ""))
	if selfService.Code != http.StatusOK || publicationService.currentCalls != 1 {
		t.Fatalf("employee self-service status/calls = %d/%d, want 200/1", selfService.Code, publicationService.currentCalls)
	}

	managementRead := httptest.NewRecorder()
	handler.ServeHTTP(managementRead, routerRequest(http.MethodGet, "/positions", "employee", ""))
	if managementRead.Code != http.StatusForbidden {
		t.Fatalf("employee management read status = %d, want 403", managementRead.Code)
	}
	if positionService.listCalls != 0 {
		t.Fatal("no-grant employee reached management read handler")
	}
}

func TestRouterSeparatesReadAndManagePermissions(t *testing.T) {
	positionService := &routerPositionService{}
	handler := NewRouter(
		routerPrincipalAuth{principals: map[string]domain.Principal{
			"reader":  routerTestPrincipal(domain.PermissionRotaRead),
			"manager": routerTestPrincipal(domain.PermissionRotaManage),
		}},
		"session",
		nil,
		RotaHandlers{Positions: NewPositionHandler(positionService)},
		WithOrigin("https://rota.example"),
	)

	reader := httptest.NewRecorder()
	handler.ServeHTTP(reader, routerRequest(http.MethodGet, "/positions", "reader", ""))
	if reader.Code != http.StatusOK {
		t.Fatalf("reader GET status = %d, want 200", reader.Code)
	}
	if positionService.listCalls != 1 {
		t.Fatalf("reader GET list calls = %d, want 1", positionService.listCalls)
	}

	readerWrite := httptest.NewRecorder()
	handler.ServeHTTP(readerWrite, routerRequest(http.MethodPost, "/positions", "reader", "https://rota.example"))
	if readerWrite.Code != http.StatusForbidden {
		t.Fatalf("reader POST status = %d, want 403", readerWrite.Code)
	}
	if positionService.createCalls != 0 {
		t.Fatal("reader POST reached the management handler")
	}

	managerWrite := httptest.NewRecorder()
	handler.ServeHTTP(managerWrite, routerRequest(http.MethodPost, "/positions", "manager", "https://rota.example"))
	if managerWrite.Code != http.StatusCreated {
		t.Fatalf("manager POST status = %d, want 201; body=%s", managerWrite.Code, managerWrite.Body.String())
	}
	if positionService.createCalls != 1 {
		t.Fatalf("manager POST create calls = %d, want 1", positionService.createCalls)
	}
}

func TestRouterRequiresOriginForWritesBeforeAuthentication(t *testing.T) {
	positionService := &routerPositionService{}
	handler := NewRouter(
		routerPrincipalAuth{principals: map[string]domain.Principal{
			"manager": routerTestPrincipal(domain.PermissionRotaManage),
		}},
		"session",
		nil,
		RotaHandlers{Positions: NewPositionHandler(positionService)},
		WithOrigin("https://rota.example"),
	)

	for _, test := range []struct {
		name   string
		origin string
	}{
		{name: "missing", origin: ""},
		{name: "evil", origin: "https://evil.example"},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, routerRequest(http.MethodPost, "/positions", "manager", test.origin))
			if response.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want 403", response.Code)
			}
		})
	}
	if positionService.createCalls != 0 {
		t.Fatal("origin-rejected request reached the management handler")
	}
}

func TestRouterReadAndManageRejectUnauthenticatedRequests(t *testing.T) {
	positionService := &routerPositionService{}
	handler := NewRouter(
		routerPrincipalAuth{principals: map[string]domain.Principal{}},
		"session",
		nil,
		RotaHandlers{Positions: NewPositionHandler(positionService)},
	)

	for _, method := range []string{http.MethodGet, http.MethodPost} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, routerRequest(method, "/positions", "missing", ""))
		if response.Code != http.StatusUnauthorized {
			t.Errorf("%s status = %d, want 401", method, response.Code)
		}
	}
}

func TestRequestIPTrustsForwardedHeadersOnlyFromConfiguredProxy(t *testing.T) {
	direct := httptest.NewRequest(http.MethodGet, "/", nil)
	direct.RemoteAddr = "198.51.100.10:1234"
	direct.Header.Set("X-Forwarded-For", "203.0.113.8")
	if got := requestIP(direct, []string{"10.0.0.0/8"}); got != "198.51.100.10" {
		t.Fatalf("untrusted proxy attribution = %q, want peer address", got)
	}

	trusted := httptest.NewRequest(http.MethodGet, "/", nil)
	trusted.RemoteAddr = "10.1.2.3:1234"
	trusted.Header.Set("X-Forwarded-For", "203.0.113.8, 10.1.2.3")
	if got := requestIP(trusted, []string{"10.0.0.0/8"}); got != "203.0.113.8" {
		t.Fatalf("trusted proxy attribution = %q, want client address", got)
	}

	malformed := httptest.NewRequest(http.MethodGet, "/", nil)
	malformed.RemoteAddr = "10.1.2.3:1234"
	malformed.Header.Set("X-Forwarded-For", "not-an-ip")
	if got := requestIP(malformed, []string{"10.0.0.0/8"}); got != "10.1.2.3" {
		t.Fatalf("malformed forwarded attribution = %q, want peer address", got)
	}
}
