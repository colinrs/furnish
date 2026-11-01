package app_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/colinrs/furnish/internal/ordering/app"
	"github.com/colinrs/furnish/internal/ordering/contract"
	"github.com/colinrs/furnish/internal/ordering/domain"
)

// 应用层测试用内存替身(fake),不需要数据库和 Redis。

type memRepo struct {
	m map[domain.OrderID]*domain.Order
}

func (r *memRepo) Get(_ context.Context, id domain.OrderID) (*domain.Order, error) {
	o, ok := r.m[id]
	if !ok {
		return nil, domain.ErrOrderNotFound
	}
	return o, nil
}
func (r *memRepo) Save(_ context.Context, o *domain.Order) error { r.m[o.ID()] = o; return nil }

type fixedCatalog map[domain.ProductID]int64

func (c fixedCatalog) UnitPrice(_ context.Context, id domain.ProductID) (domain.Money, error) {
	return domain.MustMoney(c[id], "CNY"), nil
}

type fixedCredit int64

func (c fixedCredit) LimitOf(context.Context, domain.CustomerID) (domain.Money, error) {
	return domain.MustMoney(int64(c), "CNY"), nil
}

type noTx struct{}

func (noTx) WithinTx(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }

type recorder struct {
	names    []string
	payloads []any
}

func (r *recorder) Record(_ context.Context, name string, p any, _ time.Time) error {
	r.names = append(r.names, name)
	r.payloads = append(r.payloads, p)
	return nil
}

type memIdem map[string]bool

func (m memIdem) Claim(_ context.Context, k string, _ time.Duration) (bool, error) {
	if m[k] {
		return false, nil
	}
	m[k] = true
	return true, nil
}

func newService() (*app.Service, *recorder) {
	rec := &recorder{}
	return &app.Service{
		Orders:  &memRepo{m: map[domain.OrderID]*domain.Order{}},
		Catalog: fixedCatalog{"SOFA-OAT-3S": 299900},
		Credit:  fixedCredit(500000),
		Tx:      noTx{},
		Events:  rec,
		Idem:    memIdem{},
		NewID:   func() domain.OrderID { return "SO-1" },
		Now:     func() time.Time { return time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC) },
	}, rec
}

func TestPlaceOrderPublishesContractEvent(t *testing.T) {
	svc, rec := newService()
	id, err := svc.PlaceOrder(context.Background(), app.PlaceOrderCmd{
		IdempotencyKey: "k1", CustomerID: "C-1", Items: []app.ItemCmd{{ProductID: "SOFA-OAT-3S", Quantity: 1}},
	})
	if err != nil || id != "SO-1" {
		t.Fatalf("PlaceOrder = %q, %v", id, err)
	}
	if len(rec.names) != 1 || rec.names[0] != contract.OrderPlacedV1Name {
		t.Fatalf("recorded = %v", rec.names)
	}
	ev := rec.payloads[0].(contract.OrderPlacedV1)
	if ev.Lines[0].Quantity != 1 {
		t.Errorf("payload = %+v", ev)
	}
}

func TestPlaceOrderIsIdempotent(t *testing.T) {
	svc, _ := newService()
	cmd := app.PlaceOrderCmd{IdempotencyKey: "k1", CustomerID: "C-1", Items: []app.ItemCmd{{ProductID: "SOFA-OAT-3S", Quantity: 1}}}
	_, _ = svc.PlaceOrder(context.Background(), cmd)
	if _, err := svc.PlaceOrder(context.Background(), cmd); !errors.Is(err, app.ErrDuplicateRequest) {
		t.Fatalf("second call err = %v, want ErrDuplicateRequest", err)
	}
}

func TestChangeQuantityRejectedByAggregate(t *testing.T) {
	svc, _ := newService()
	_, _ = svc.PlaceOrder(context.Background(), app.PlaceOrderCmd{CustomerID: "C-1", Items: []app.ItemCmd{{ProductID: "SOFA-OAT-3S", Quantity: 1}}})
	if err := svc.ChangeQuantity(context.Background(), "SO-1", 1, 2); !errors.Is(err, domain.ErrCreditLimitExceeded) {
		t.Fatalf("err = %v, want ErrCreditLimitExceeded", err)
	}
}
