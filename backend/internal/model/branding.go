package model

import "time"

const (
	DefaultProductName      = "Rota"
	DefaultOrganizationName = ""
)

type Branding struct {
	ProductName      string
	OrganizationName string
	Version          int
	CreatedAt        time.Time
	UpdatedAt        time.Time
}
