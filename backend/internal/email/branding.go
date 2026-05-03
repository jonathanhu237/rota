package email

import "strings"

const (
	DefaultProductName      = "Rota"
	DefaultOrganizationName = ""
)

type Branding struct {
	ProductName      string
	OrganizationName string
}

func DefaultBranding() Branding {
	return Branding{
		ProductName:      DefaultProductName,
		OrganizationName: DefaultOrganizationName,
	}
}

func NormalizeBranding(branding Branding) Branding {
	normalized := Branding{
		ProductName:      strings.TrimSpace(branding.ProductName),
		OrganizationName: strings.TrimSpace(branding.OrganizationName),
	}
	if normalized.ProductName == "" {
		normalized.ProductName = DefaultProductName
	}
	return normalized
}
