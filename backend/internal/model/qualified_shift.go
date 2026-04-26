package model

type QualifiedShift struct {
	SlotID            int64
	PositionID        int64
	Weekday           int
	StartTime         string
	EndTime           string
	RequiredHeadcount int
}
