package email

import (
	"fmt"
	"strings"
)

// ShiftChangeType mirrors the service-level enum. We duplicate the constants
// here to avoid an import cycle between the email package and the model
// package.
type ShiftChangeType string

const (
	ShiftChangeTypeSwap       ShiftChangeType = "swap"
	ShiftChangeTypeGiveDirect ShiftChangeType = "give_direct"
	ShiftChangeTypeGivePool   ShiftChangeType = "give_pool"
)

// ShiftChangeOutcome distinguishes the resolution messages.
type ShiftChangeOutcome string

const (
	ShiftChangeOutcomeApproved  ShiftChangeOutcome = "approved"
	ShiftChangeOutcomeRejected  ShiftChangeOutcome = "rejected"
	ShiftChangeOutcomeClaimed   ShiftChangeOutcome = "claimed"
	ShiftChangeOutcomeCancelled ShiftChangeOutcome = "cancelled"
	ShiftChangeOutcomeInvalidated ShiftChangeOutcome = "invalidated"
)

// ShiftRef describes a single slot-position assignment in human-readable
// form for email bodies. Callers assemble this from slot-based scheduling
// data plus the display position name.
type ShiftRef struct {
	Weekday      string // "Mon", "Tue", …
	StartTime    string // "09:00"
	EndTime      string // "12:00"
	PositionName string
}

// ShiftChangeRequestReceivedData drives the "someone is asking you to swap"
// or "someone wants to give you a shift" email.
type ShiftChangeRequestReceivedData struct {
	To            string
	RecipientName string
	RequesterName string
	Type          ShiftChangeType
	RequesterShift ShiftRef
	CounterpartShift *ShiftRef // non-nil only for swap
	BaseURL       string
}

// ShiftChangeResolvedData drives the "your request was approved/rejected/
// claimed/cancelled" email to the requester.
type ShiftChangeResolvedData struct {
	To            string
	RecipientName string
	Outcome       ShiftChangeOutcome
	Type          ShiftChangeType
	ResponderName string // the counterpart / claimer; may be empty for cancelled
	RequesterShift ShiftRef
	CounterpartShift *ShiftRef
	BaseURL       string
}

// BuildShiftChangeRequestReceivedMessage builds the email sent to the
// counterpart when a swap or give_direct request is created.
func BuildShiftChangeRequestReceivedMessage(data ShiftChangeRequestReceivedData) Message {
	subject := "You have a new shift change request on Rota"

	var b strings.Builder
	fmt.Fprintf(&b, "Hi %s,\n\n", data.RecipientName)

	switch data.Type {
	case ShiftChangeTypeSwap:
		fmt.Fprintf(&b, "%s would like to swap shifts with you:\n", data.RequesterName)
		fmt.Fprintf(&b, "  They would take your shift: %s\n", formatShiftRef(data.CounterpartShift))
		fmt.Fprintf(&b, "  You would take their shift: %s\n", formatShiftRefValue(data.RequesterShift))
	case ShiftChangeTypeGiveDirect:
		fmt.Fprintf(&b, "%s would like to give you this shift:\n", data.RequesterName)
		fmt.Fprintf(&b, "  %s\n", formatShiftRefValue(data.RequesterShift))
	default:
		fmt.Fprintf(&b, "%s has opened a request involving your schedule.\n", data.RequesterName)
	}

	fmt.Fprintf(&b, "\nRespond here: %s\n", requestsLink(data.BaseURL))
	fmt.Fprint(&b, "\nThis request expires when the schedule is activated.\n")

	return Message{
		To:      data.To,
		Subject: subject,
		Body:    b.String(),
	}
}

// BuildShiftChangeResolvedMessage builds the email sent to the requester
// when their request is approved, rejected, claimed, or cancelled.
func BuildShiftChangeResolvedMessage(data ShiftChangeResolvedData) Message {
	subject := "Your shift change request was " + string(data.Outcome)

	var b strings.Builder
	fmt.Fprintf(&b, "Hi %s,\n\n", data.RecipientName)

	switch data.Outcome {
	case ShiftChangeOutcomeApproved:
		fmt.Fprintf(&b, "%s approved your shift change request.\n", data.ResponderName)
	case ShiftChangeOutcomeRejected:
		fmt.Fprintf(&b, "%s rejected your shift change request.\n", data.ResponderName)
	case ShiftChangeOutcomeClaimed:
		fmt.Fprintf(&b, "%s claimed the shift you released to the pool.\n", data.ResponderName)
	case ShiftChangeOutcomeCancelled:
		fmt.Fprint(&b, "Your shift change request was cancelled.\n")
	case ShiftChangeOutcomeInvalidated:
		fmt.Fprint(&b, "Your shift change request is no longer applicable because an administrator edited the referenced shift.\n")
	}

	if !isZeroShiftRef(data.RequesterShift) {
		fmt.Fprintf(&b, "\nShift involved: %s\n", formatShiftRefValue(data.RequesterShift))
	}
	if data.CounterpartShift != nil {
		fmt.Fprintf(&b, "Counterpart shift: %s\n", formatShiftRef(data.CounterpartShift))
	}

	fmt.Fprintf(&b, "\nSee details: %s\n", requestsLink(data.BaseURL))

	return Message{
		To:      data.To,
		Subject: subject,
		Body:    b.String(),
	}
}

func formatShiftRefValue(s ShiftRef) string {
	return fmt.Sprintf("%s %s–%s %s", s.Weekday, s.StartTime, s.EndTime, s.PositionName)
}

func formatShiftRef(s *ShiftRef) string {
	if s == nil {
		return "(unknown)"
	}
	return formatShiftRefValue(*s)
}

func isZeroShiftRef(s ShiftRef) bool {
	return s.Weekday == "" && s.StartTime == "" && s.EndTime == "" && s.PositionName == ""
}

func requestsLink(baseURL string) string {
	return strings.TrimRight(baseURL, "/") + "/requests"
}
