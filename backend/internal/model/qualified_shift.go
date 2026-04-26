package model

type QualifiedShift struct {
	SlotID      int64
	Weekday     int
	StartTime   string
	EndTime     string
	Composition []QualifiedShiftComposition
}

type QualifiedShiftComposition struct {
	PositionID        int64
	PositionName      string
	RequiredHeadcount int
}
