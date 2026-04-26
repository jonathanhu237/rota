// Package audit provides structured, append-only audit logging.
//
// Services call Record at the successful end of mutating operations and at
// key authentication events. The actor (user ID) and actor IP are pulled
// from the request context so service method signatures stay clean.
package audit

import (
	"context"
	"log/slog"
)

// Action constants list every audit event the system emits. Keep this list
// authoritative so new actions are easy to discover during code review.
const (
	ActionUserCreate                = "user.create"
	ActionUserUpdate                = "user.update"
	ActionUserStatusActivate        = "user.status.activate"
	ActionUserStatusDisable         = "user.status.disable"
	ActionUserInvitationResend      = "user.invitation.resend"
	ActionUserInvitationEmailFailed = "user.invitation.email_failed"
	ActionUserQualificationsReplace = "user.qualifications.replace"

	ActionPositionCreate = "position.create"
	ActionPositionUpdate = "position.update"
	ActionPositionDelete = "position.delete"

	ActionTemplateCreate = "template.create"
	ActionTemplateUpdate = "template.update"
	ActionTemplateDelete = "template.delete"
	ActionTemplateClone  = "template.clone"

	ActionSlotPositionCreate = "template.shift.create"
	ActionSlotPositionUpdate = "template.shift.update"
	ActionSlotPositionDelete = "template.shift.delete"

	ActionPublicationCreate     = "publication.create"
	ActionPublicationUpdate     = "publication.update"
	ActionPublicationDelete     = "publication.delete"
	ActionPublicationPublish    = "publication.publish"
	ActionPublicationActivate   = "publication.activate"
	ActionPublicationEnd        = "publication.end"
	ActionPublicationAutoAssign = "publication.auto_assign"

	ActionShiftChangeCreate            = "shift_change.create"
	ActionShiftChangeApprove           = "shift_change.approve"
	ActionShiftChangeReject            = "shift_change.reject"
	ActionShiftChangeCancel            = "shift_change.cancel"
	ActionShiftChangeInvalidateCascade = "shift_change.invalidate.cascade"
	ActionShiftChangeExpireBulk        = "shift_change.expire.bulk"

	ActionLeaveCreate = "leave.create"
	ActionLeaveCancel = "leave.cancel"

	ActionSubmissionCreate = "submission.create"
	ActionSubmissionDelete = "submission.delete"

	ActionAssignmentCreate = "assignment.create"
	ActionAssignmentDelete = "assignment.delete"

	ActionAuthLoginSuccess         = "auth.login.success"
	ActionAuthLoginFailure         = "auth.login.failure"
	ActionAuthLogout               = "auth.logout"
	ActionAuthPasswordResetRequest = "auth.password_reset.request"
	ActionAuthPasswordSet          = "auth.password.set"
)

// TargetType constants name the entity kinds referenced in audit rows.
const (
	TargetTypeUser                   = "user"
	TargetTypePosition               = "position"
	TargetTypeTemplate               = "template"
	TargetTypeSlotPosition           = "slot_position"
	TargetTypePublication            = "publication"
	TargetTypeAvailabilitySubmission = "availability_submission"
	TargetTypeAssignment             = "assignment"
	TargetTypeShiftChangeRequest     = "shift_change_request"
	TargetTypeLeave                  = "leave"
)

// Event captures a single audit record. Actor and ActorIP are taken from
// context, not the Event, so callers only describe the domain action itself.
type Event struct {
	Action     string
	TargetType string
	TargetID   *int64
	Metadata   map[string]any
}

// Recorder persists an audit event. Implementations should never fail the
// caller: record errors must be logged and swallowed so the primary
// operation is not affected by audit infrastructure problems.
type Recorder interface {
	Record(ctx context.Context, occurred RecordedEvent)
}

// RecordedEvent is the fully resolved event passed to a Recorder, after
// Actor and ActorIP have been extracted from context.
type RecordedEvent struct {
	ActorID    *int64
	ActorIP    string
	Action     string
	TargetType string
	TargetID   *int64
	Metadata   map[string]any
}

type recorderCtxKey struct{}
type actorCtxKey struct{}
type actorIPCtxKey struct{}

// WithRecorder returns a context that carries the given Recorder.
func WithRecorder(ctx context.Context, r Recorder) context.Context {
	if r == nil {
		return ctx
	}
	return context.WithValue(ctx, recorderCtxKey{}, r)
}

// WithActor returns a context carrying the authenticated user's ID.
func WithActor(ctx context.Context, userID int64) context.Context {
	return context.WithValue(ctx, actorCtxKey{}, userID)
}

// WithActorIP returns a context carrying the caller's IP address.
func WithActorIP(ctx context.Context, ip string) context.Context {
	return context.WithValue(ctx, actorIPCtxKey{}, ip)
}

func recorderFromContext(ctx context.Context) Recorder {
	if r, ok := ctx.Value(recorderCtxKey{}).(Recorder); ok {
		return r
	}
	return noopRecorder{}
}

func actorFromContext(ctx context.Context) *int64 {
	if v, ok := ctx.Value(actorCtxKey{}).(int64); ok {
		return &v
	}
	return nil
}

// ActorFromContext exposes the authenticated actor's user ID, if any. It is
// intended for callers that need the acting user for domain decisions such as
// deriving an audit TargetID.
func ActorFromContext(ctx context.Context) (int64, bool) {
	if v, ok := ctx.Value(actorCtxKey{}).(int64); ok {
		return v, true
	}
	return 0, false
}

func actorIPFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(actorIPCtxKey{}).(string); ok {
		return v
	}
	return ""
}

// Record writes an audit event, pulling Actor and ActorIP from the context.
// This call never fails from the caller's perspective. If the recorder is
// missing or the write fails, a warning is emitted via slog but the caller
// continues unaffected.
func Record(ctx context.Context, event Event) {
	if event.Action == "" {
		slog.Warn("audit.Record called with empty action")
		return
	}

	resolved := RecordedEvent{
		ActorID:    actorFromContext(ctx),
		ActorIP:    actorIPFromContext(ctx),
		Action:     event.Action,
		TargetType: event.TargetType,
		TargetID:   event.TargetID,
		Metadata:   event.Metadata,
	}

	recorderFromContext(ctx).Record(ctx, resolved)
}

// noopRecorder discards events. Used when no Recorder is configured, such
// as in tests that don't care about audit output.
type noopRecorder struct{}

func (noopRecorder) Record(_ context.Context, _ RecordedEvent) {}
