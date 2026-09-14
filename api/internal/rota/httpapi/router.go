package httpapi

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"

	"github.com/jonathanhu237/rota/api/internal/auth/application"
	"github.com/jonathanhu237/rota/api/internal/auth/domain"
	"github.com/jonathanhu237/rota/api/internal/rota/audit"
	"github.com/jonathanhu237/rota/api/internal/rota/model"
)

// PrincipalAuthenticator is the small common-auth seam needed by the Rota
// extension. The generated authentication service remains the authority for
// session validation, account lifecycle, and RBAC.
type PrincipalAuthenticator interface {
	CurrentPrincipal(context.Context, string) (domain.Principal, error)
}

// RotaHandlers contains the business handlers mounted below /api/rota.
type RotaHandlers struct {
	Positions     *PositionHandler
	Templates     *TemplateHandler
	Publications  *PublicationHandler
	Attendance    *AttendanceHandler
	UserPositions *UserPositionHandler
	ShiftChanges  *ShiftChangeHandler
	Leaves        *LeaveHandler
}

// RouterOption configures request-boundary behavior for the Rota subrouter.
type RouterOption func(*routerConfig)

type routerConfig struct {
	origin            string
	trustedProxyCIDRs []string
}

// WithOrigin enables same-origin protection for every Rota write. The empty
// origin is intentionally left permissive for unit-test and embedded callers;
// the production configuration always supplies APP_PUBLIC_URL.
func WithOrigin(origin string) RouterOption {
	return func(config *routerConfig) { config.origin = origin }
}

// WithTrustedProxyCIDRs controls source-IP attribution for Rota audit events.
// Forwarding headers are ignored unless the immediate peer is in one of these
// networks.
func WithTrustedProxyCIDRs(cidrs []string) RouterOption {
	return func(config *routerConfig) { config.trustedProxyCIDRs = append([]string(nil), cidrs...) }
}

// NewRouter mounts the complete Rota business API. It intentionally returns a
// subrouter whose paths are relative to /api/rota; the application handler owns
// the common /api namespace. Optional router options preserve the original
// constructor seam for embedders and tests.
func NewRouter(auth PrincipalAuthenticator, cookieName string, recorder audit.Recorder, handlers RotaHandlers, options ...RouterOption) http.Handler {
	config := routerConfig{}
	for _, option := range options {
		if option != nil {
			option(&config)
		}
	}
	mux := http.NewServeMux()
	if auth == nil {
		return mux
	}

	authenticated := func(next http.Handler) http.Handler {
		return withPrincipal(auth, cookieName, recorder, next, accessAuthenticated, config)
	}
	readOnly := func(next http.Handler) http.Handler {
		return withPrincipal(auth, cookieName, recorder, next, accessRead, config)
	}
	managed := func(next http.Handler) http.Handler {
		return withPrincipal(auth, cookieName, recorder, next, accessManage, config)
	}
	h := func(fn http.HandlerFunc) http.Handler { return authenticated(fn) }
	r := func(fn http.HandlerFunc) http.Handler { return readOnly(fn) }
	m := func(fn http.HandlerFunc) http.Handler { return managed(fn) }

	if handlers.Positions != nil {
		mux.Handle("GET /positions", r(handlers.Positions.List))
		mux.Handle("POST /positions", m(handlers.Positions.Create))
		mux.Handle("GET /positions/{id}", r(handlers.Positions.GetByID))
		mux.Handle("PUT /positions/{id}", m(handlers.Positions.Update))
		mux.Handle("DELETE /positions/{id}", m(handlers.Positions.Delete))
	}
	if handlers.UserPositions != nil {
		mux.Handle("GET /users/{id}/positions", r(handlers.UserPositions.List))
		mux.Handle("PUT /users/{id}/positions", m(handlers.UserPositions.Replace))
	}
	if handlers.Templates != nil {
		mux.Handle("GET /templates", r(handlers.Templates.List))
		mux.Handle("POST /templates", m(handlers.Templates.Create))
		mux.Handle("GET /templates/{id}", r(handlers.Templates.GetByID))
		mux.Handle("PUT /templates/{id}", m(handlers.Templates.Update))
		mux.Handle("DELETE /templates/{id}", m(handlers.Templates.Delete))
		mux.Handle("POST /templates/{id}/clone", m(handlers.Templates.Clone))
		mux.Handle("POST /templates/{id}/slots", m(handlers.Templates.CreateSlot))
		mux.Handle("PATCH /templates/{id}/slots/{slot_id}", m(handlers.Templates.UpdateSlot))
		mux.Handle("DELETE /templates/{id}/slots/{slot_id}", m(handlers.Templates.DeleteSlot))
		mux.Handle("POST /templates/{id}/slots/{slot_id}/positions", m(handlers.Templates.CreateSlotPosition))
		mux.Handle("PATCH /templates/{id}/slots/{slot_id}/positions/{position_entry_id}", m(handlers.Templates.UpdateSlotPosition))
		mux.Handle("DELETE /templates/{id}/slots/{slot_id}/positions/{position_entry_id}", m(handlers.Templates.DeleteSlotPosition))
	}
	if handlers.Publications != nil {
		mux.Handle("GET /publications", r(handlers.Publications.List))
		mux.Handle("POST /publications", m(handlers.Publications.Create))
		mux.Handle("GET /publications/{id}", r(handlers.Publications.GetByID))
		mux.Handle("PATCH /publications/{id}", m(handlers.Publications.Update))
		mux.Handle("DELETE /publications/{id}", m(handlers.Publications.Delete))
		mux.Handle("GET /publications/{id}/assignment-board", r(handlers.Publications.GetAssignmentBoard))
		mux.Handle("GET /publications/{id}/availability-board", r(handlers.Publications.ListAdminAvailability))
		mux.Handle("GET /publications/{id}/availability-submissions/{user_id}", r(handlers.Publications.GetAdminAvailabilityDetail))
		mux.Handle("PUT /publications/{id}/availability-submissions/{user_id}", m(handlers.Publications.ReplaceAdminAvailability))
		mux.Handle("POST /publications/{id}/auto-assign", m(handlers.Publications.AutoAssign))
		mux.Handle("POST /publications/{id}/assignments", m(handlers.Publications.CreateAssignment))
		mux.Handle("DELETE /publications/{id}/assignments/{assignment_id}", m(handlers.Publications.DeleteAssignment))
		mux.Handle("POST /publications/{id}/publish", m(handlers.Publications.Publish))
		mux.Handle("POST /publications/{id}/activate", m(handlers.Publications.Activate))
		mux.Handle("POST /publications/{id}/end", m(handlers.Publications.End))
		mux.Handle("GET /publications/current", h(handlers.Publications.GetCurrent))
		mux.Handle("GET /roster/current", h(handlers.Publications.GetCurrentRoster))
		mux.Handle("GET /publications/{id}/schedule.xlsx", h(handlers.Publications.ExportScheduleXLSX))
		mux.Handle("GET /publications/{id}/roster", h(handlers.Publications.GetRoster))
		mux.Handle("GET /publications/{id}/submissions/me", h(handlers.Publications.ListMySubmissionSlots))
		mux.Handle("POST /publications/{id}/submissions", h(handlers.Publications.CreateSubmission))
		mux.Handle("DELETE /publications/{id}/submissions/{slot_id}/{weekday}", h(handlers.Publications.DeleteSubmission))
		mux.Handle("GET /publications/{id}/shifts/me", h(handlers.Publications.ListMyQualifiedShifts))
	}
	if handlers.Attendance != nil {
		mux.Handle("GET /publications/{id}/attendance", r(handlers.Attendance.ListAdmin))
		mux.Handle("GET /publications/{id}/attendance/shifts/{slot_id}/{occurrence_date}", r(handlers.Attendance.GetAdminShift))
		mux.Handle("PUT /publications/{id}/attendance/arrivals", m(handlers.Attendance.AdminUpsertArrival))
		mux.Handle("DELETE /publications/{id}/attendance/arrivals/{record_id}", m(handlers.Attendance.AdminClearArrival))
		mux.Handle("POST /publications/{id}/attendance/overtime", m(handlers.Attendance.AdminCreateOvertime))
		mux.Handle("PATCH /publications/{id}/attendance/overtime/{record_id}", m(handlers.Attendance.AdminUpdateOvertime))
		mux.Handle("DELETE /publications/{id}/attendance/overtime/{record_id}", m(handlers.Attendance.AdminDeleteOvertime))
		mux.Handle("PATCH /publications/{id}/attendance/settings", m(handlers.Attendance.UpdateSettings))
		mux.Handle("GET /attendance/current", h(handlers.Attendance.Current))
		mux.Handle("POST /attendance/arrivals", h(handlers.Attendance.RecordLeaderArrival))
		mux.Handle("POST /attendance/overtime", h(handlers.Attendance.RecordLeaderOvertime))
	}
	if handlers.ShiftChanges != nil {
		mux.Handle("POST /publications/{id}/shift-changes", h(handlers.ShiftChanges.Create))
		mux.Handle("GET /publications/{id}/shift-changes", h(handlers.ShiftChanges.List))
		mux.Handle("GET /publications/{id}/shift-changes/{request_id}", h(handlers.ShiftChanges.GetByID))
		mux.Handle("POST /publications/{id}/shift-changes/{request_id}/approve", h(handlers.ShiftChanges.Approve))
		mux.Handle("POST /publications/{id}/shift-changes/{request_id}/reject", h(handlers.ShiftChanges.Reject))
		mux.Handle("POST /publications/{id}/shift-changes/{request_id}/cancel", h(handlers.ShiftChanges.Cancel))
		mux.Handle("GET /publications/{id}/members", h(handlers.ShiftChanges.ListMembers))
		mux.Handle("GET /users/me/notifications/unread-count", h(handlers.ShiftChanges.UnreadCount))
	}
	if handlers.Leaves != nil {
		mux.Handle("GET /publications/{id}/leaves", r(handlers.Leaves.ListForPublication))
		mux.Handle("POST /leaves", h(handlers.Leaves.Create))
		mux.Handle("GET /leaves/pool", h(handlers.Leaves.ListPool))
		mux.Handle("GET /leaves/{id}", h(handlers.Leaves.GetByID))
		mux.Handle("POST /leaves/{id}/cancel", h(handlers.Leaves.Cancel))
		mux.Handle("GET /users/me/leaves", h(handlers.Leaves.ListMine))
		mux.Handle("GET /users/me/leaves/preview", h(handlers.Leaves.PreviewMine))
	}

	return mux
}

type rotaAccess uint8

const (
	accessAuthenticated rotaAccess = iota
	accessRead
	accessManage
)

func withPrincipal(auth PrincipalAuthenticator, cookieName string, recorder audit.Recorder, next http.Handler, access rotaAccess, config routerConfig) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead && !validOrigin(r, config.origin) {
			writeError(w, http.StatusForbidden, "FORBIDDEN", "Forbidden")
			return
		}
		cookie, err := r.Cookie(cookieName)
		if err != nil || strings.TrimSpace(cookie.Value) == "" {
			writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required")
			return
		}
		principal, err := auth.CurrentPrincipal(r.Context(), cookie.Value)
		if err != nil {
			if errors.Is(err, application.ErrForbidden) {
				writeError(w, http.StatusForbidden, "FORBIDDEN", "Forbidden")
			} else {
				writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required")
			}
			return
		}
		switch access {
		case accessRead:
			if !principal.SuperAdmin && !principal.Has(domain.PermissionRotaRead) && !principal.Has(domain.PermissionRotaManage) {
				writeError(w, http.StatusForbidden, "FORBIDDEN", "Forbidden")
				return
			}
		case accessManage:
			if !principal.SuperAdmin && !principal.Has(domain.PermissionRotaManage) {
				writeError(w, http.StatusForbidden, "FORBIDDEN", "Forbidden")
				return
			}
		}
		user := rotaUser(principal, access == accessManage)
		ctx := WithCurrentUser(r.Context(), user)
		ctx = context.WithValue(ctx, rotaPrincipalContextKey{}, principal)
		if recorder != nil {
			ctx = audit.WithRecorder(ctx, recorder)
		}
		ctx = audit.WithActor(ctx, user.ID)
		ctx = audit.WithActorIP(ctx, requestIP(r, config.trustedProxyCIDRs))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func validOrigin(r *http.Request, expected string) bool {
	if expected == "" {
		return true
	}
	values := r.Header.Values("Origin")
	if len(values) != 1 {
		return false
	}
	return strings.TrimSpace(values[0]) == expected
}

func requestIP(r *http.Request, trustedCIDRs []string) string {
	peer, peerIP := parseSourceAddress(r.RemoteAddr)
	if peerIP == nil || !trustedProxy(peerIP, trustedCIDRs) {
		return peer
	}
	if values := r.Header.Values("X-Forwarded-For"); len(values) > 0 {
		client, valid := forwardedClientIP(values, func(ip net.IP) bool { return trustedProxy(ip, trustedCIDRs) })
		if !valid || client == nil {
			return peer
		}
		return canonicalIP(client)
	}
	if values := r.Header.Values("X-Real-IP"); len(values) == 1 {
		candidate := strings.TrimSpace(values[0])
		parsed := net.ParseIP(candidate)
		if candidate != "" && parsed != nil && !trustedProxy(parsed, trustedCIDRs) {
			return canonicalIP(parsed)
		}
	}
	return peer
}

func parseSourceAddress(remoteAddr string) (string, net.IP) {
	host := strings.TrimSpace(remoteAddr)
	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		host = parsedHost
	}
	parsed := net.ParseIP(host)
	if parsed == nil {
		return "unknown", nil
	}
	return canonicalIP(parsed), parsed
}

func canonicalIP(ip net.IP) string {
	if ipv4 := ip.To4(); ipv4 != nil {
		return ipv4.String()
	}
	return ip.String()
}

func trustedProxy(ip net.IP, cidrs []string) bool {
	for _, rawCIDR := range cidrs {
		_, network, err := net.ParseCIDR(strings.TrimSpace(rawCIDR))
		if err == nil && network.Contains(ip) {
			return true
		}
	}
	return false
}

func forwardedClientIP(values []string, trusted func(net.IP) bool) (net.IP, bool) {
	for valueIndex := len(values) - 1; valueIndex >= 0; valueIndex-- {
		parts := strings.Split(values[valueIndex], ",")
		for partIndex := len(parts) - 1; partIndex >= 0; partIndex-- {
			candidate := strings.TrimSpace(parts[partIndex])
			parsed := net.ParseIP(candidate)
			if candidate == "" || parsed == nil {
				return nil, false
			}
			if !trusted(parsed) {
				return parsed, true
			}
		}
	}
	return nil, true
}

type rotaPrincipalContextKey struct{}

func PrincipalFromRequest(r *http.Request) (domain.Principal, bool) {
	principal, ok := r.Context().Value(rotaPrincipalContextKey{}).(domain.Principal)
	return principal, ok
}

func rotaUser(principal domain.Principal, managed bool) *model.User {
	user := &model.User{
		ID:      principal.User.ID,
		Email:   principal.User.Email,
		Name:    principal.User.Name,
		Status:  model.UserStatusActive,
		Version: 1,
		IsAdmin: principal.SuperAdmin || principal.Has(domain.PermissionRotaManage) || managed,
	}
	if principal.User.Locale == domain.LocaleChinese {
		value := model.LanguagePreferenceZH
		user.LanguagePreference = &value
	} else if principal.User.Locale == domain.LocaleEnglish {
		value := model.LanguagePreferenceEN
		user.LanguagePreference = &value
	}
	return user
}
