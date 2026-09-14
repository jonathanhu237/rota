package application

import (
	"context"
	"errors"
	"testing"

	"github.com/jonathanhu237/rota/api/internal/auth/domain"
)

type identityBrandingStoreFake struct {
	record SystemIdentityRecord
	err    error
}

func (f *identityBrandingStoreFake) GetSystemIdentity(context.Context) (SystemIdentityRecord, error) {
	if f.err != nil {
		return SystemIdentityRecord{}, f.err
	}
	return f.record, nil
}
func (f *identityBrandingStoreFake) SaveSystemIdentity(_ context.Context, _ int64, record SystemIdentityRecord) (SystemIdentityRecord, error) {
	if f.err != nil {
		return SystemIdentityRecord{}, f.err
	}
	record.Revision++
	f.record = record
	return record, nil
}

func TestSystemIdentityOrganizationBrandingIsNormalizedAndRetained(t *testing.T) {
	store := &identityBrandingStoreFake{record: SystemIdentityRecord{SystemName: "Temvia", Revision: 4}}
	management := NewSystemIdentityManagement(store)
	view, err := management.SaveSystemIdentity(context.Background(), SystemIdentityInput{
		SystemName:       " Rota ",
		OrganizationName: "  Acme Operations  ",
		IconAction:       SystemIconPreserve,
		Revision:         4,
	})
	if err != nil {
		t.Fatalf("SaveSystemIdentity() error = %v", err)
	}
	if view.SystemName != "Rota" || view.OrganizationName != "Acme Operations" || store.record.OrganizationName != "Acme Operations" {
		t.Fatalf("branding = %#v, stored=%#v", view, store.record)
	}
}

func TestSystemIdentityOrganizationBrandingRejectsLengthAndControlCharacters(t *testing.T) {
	management := NewSystemIdentityManagement(&identityBrandingStoreFake{record: SystemIdentityRecord{SystemName: "Temvia", Revision: 1}})
	for _, value := range []string{"x\norganization", string(make([]rune, MaxOrganizationNameLength+1))} {
		_, err := management.SaveSystemIdentity(context.Background(), SystemIdentityInput{
			SystemName:       "Temvia",
			OrganizationName: value,
			Revision:         1,
		})
		var validation *domain.ValidationErrors
		if !errors.As(err, &validation) || len(validation.Items) != 1 || validation.Items[0].Field != "organizationName" {
			t.Fatalf("organization %q error = %v, want organizationName validation", value, err)
		}
	}
}
