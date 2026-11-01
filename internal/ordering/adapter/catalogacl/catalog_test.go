package catalogacl

import (
	"context"
	"errors"
	"testing"

	"github.com/colinrs/furnish/internal/ordering/domain"
)

type fakeERP map[string]Material

func (f fakeERP) GetMaterial(_ context.Context, matnr string) (Material, error) { return f[matnr], nil }

func TestTranslatesLegacyMaterial(t *testing.T) {
	erp := fakeERP{"000000000010023": {MATNR: "000000000010023", NETPR: "2999.5", WAERS: "CNY", MSTAE: "01"}}
	c := New(erp, func(domain.ProductID) string { return "000000000010023" })
	got, err := c.UnitPrice(context.Background(), "SOFA-OAT-3S")
	if err != nil || got.Amount() != 299950 || got.Currency() != "CNY" {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestDiscontinuedMaterialIsNotSellable(t *testing.T) {
	erp := fakeERP{"X": {NETPR: "1.00", WAERS: "CNY", MSTAE: "99"}}
	c := New(erp, func(domain.ProductID) string { return "X" })
	if _, err := c.UnitPrice(context.Background(), "ANY"); !errors.Is(err, ErrProductNotSellable) {
		t.Fatalf("err = %v", err)
	}
}
