package domain_test

import (
	"errors"
	"testing"
	"time"

	"github.com/colinrs/furnish/internal/ordering/domain"
)

var (
	now    = time.Date(2026, 10, 7, 10, 0, 0, 0, time.UTC)
	sofa   = domain.LineSpec{ProductID: "SOFA-OAT-3S", UnitPrice: domain.MustMoney(299900, "CNY"), Quantity: 1}
	pillow = domain.LineSpec{ProductID: "PILLOW-01", UnitPrice: domain.MustMoney(9900, "CNY"), Quantity: 2}
)

func TestPlaceOrder(t *testing.T) {
	tests := []struct {
		name    string
		limit   int64
		specs   []domain.LineSpec
		wantErr error
	}{
		{"within credit limit", 500000, []domain.LineSpec{sofa, pillow}, nil},
		{"exceeds credit limit", 300000, []domain.LineSpec{sofa, pillow}, domain.ErrCreditLimitExceeded},
		{"empty order", 500000, nil, domain.ErrEmptyOrder},
		{"zero quantity", 500000, []domain.LineSpec{{ProductID: "X", UnitPrice: domain.MustMoney(100, "CNY"), Quantity: 0}}, domain.ErrInvalidQuantity},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o, err := domain.PlaceOrder("SO-1", "C-1", domain.MustMoney(tt.limit, "CNY"), tt.specs, now)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if got := o.Total().Amount(); got != 319700 {
				t.Errorf("total = %d, want 319700", got)
			}
			ev := o.PullEvents()
			if len(ev) != 1 || ev[0].EventName() != "ordering.OrderPlaced" {
				t.Errorf("events = %v, want one OrderPlaced", ev)
			}
		})
	}
}

func TestChangeQuantityKeepsInvariant(t *testing.T) {
	o, _ := domain.PlaceOrder("SO-1", "C-1", domain.MustMoney(400000, "CNY"), []domain.LineSpec{sofa}, now)
	o.PullEvents()

	if err := o.ChangeQuantity(1, 2, now); !errors.Is(err, domain.ErrCreditLimitExceeded) {
		t.Fatalf("err = %v, want ErrCreditLimitExceeded", err)
	}
	if q := o.Lines()[0].Quantity(); q != 1 {
		t.Errorf("quantity after rejected change = %d, want 1 (rolled back)", q)
	}
	if len(o.PullEvents()) != 0 {
		t.Error("rejected change must not record events")
	}
}

func TestLockedOrderCannotChange(t *testing.T) {
	o, _ := domain.PlaceOrder("SO-1", "C-1", domain.MustMoney(900000, "CNY"), []domain.LineSpec{sofa}, now)
	_ = o.Lock("customer request", now)
	if err := o.ChangeQuantity(1, 2, now); !errors.Is(err, domain.ErrOrderLocked) {
		t.Fatalf("err = %v, want ErrOrderLocked", err)
	}
}

func TestLinesReturnsCopy(t *testing.T) {
	o, _ := domain.PlaceOrder("SO-1", "C-1", domain.MustMoney(900000, "CNY"), []domain.LineSpec{sofa}, now)
	lines := o.Lines()
	lines[0] = domain.NewOrderLineSnapshot(1, "HACK", domain.MustMoney(1, "CNY"), 1)
	if o.Lines()[0].ProductID() != "SOFA-OAT-3S" {
		t.Error("outside code must not be able to modify lines inside the aggregate")
	}
}
