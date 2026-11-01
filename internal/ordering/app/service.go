package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/colinrs/furnish/internal/ordering/contract"
	"github.com/colinrs/furnish/internal/ordering/domain"
)

var ErrDuplicateRequest = errors.New("duplicate request")

// Service 是应用服务:一个方法对应一个用例。
// 它很薄:取数据、调用领域对象、定事务边界、发事件。没有业务规则。
type Service struct {
	Orders  domain.Repository
	Catalog Catalog
	Credit  CreditPolicy
	Tx      Transactor
	Events  EventRecorder
	Idem    IdempotencyGuard
	NewID   IDGenerator
	Now     Clock
}

type PlaceOrderCmd struct {
	IdempotencyKey string
	CustomerID     string
	Items          []ItemCmd
}

type ItemCmd struct {
	ProductID string
	Quantity  int
}

func (s *Service) PlaceOrder(ctx context.Context, cmd PlaceOrderCmd) (domain.OrderID, error) {
	if cmd.IdempotencyKey != "" {
		ok, err := s.Idem.Claim(ctx, "place-order:"+cmd.IdempotencyKey, 24*time.Hour)
		if err != nil {
			return "", fmt.Errorf("claim idempotency key: %w", err)
		}
		if !ok {
			return "", ErrDuplicateRequest
		}
	}

	customer := domain.CustomerID(cmd.CustomerID)
	limit, err := s.Credit.LimitOf(ctx, customer)
	if err != nil {
		return "", err
	}
	specs := make([]domain.LineSpec, 0, len(cmd.Items))
	for _, it := range cmd.Items {
		pid := domain.ProductID(it.ProductID)
		price, err := s.Catalog.UnitPrice(ctx, pid) // 价格快照
		if err != nil {
			return "", err
		}
		specs = append(specs, domain.LineSpec{ProductID: pid, UnitPrice: price, Quantity: it.Quantity})
	}

	order, err := domain.PlaceOrder(s.NewID(), customer, limit, specs, s.Now())
	if err != nil {
		return "", err
	}
	return order.ID(), s.saveWithEvents(ctx, order)
}

func (s *Service) ChangeQuantity(ctx context.Context, id string, lineNo, qty int) error {
	return s.Tx.WithinTx(ctx, func(ctx context.Context) error {
		order, err := s.Orders.Get(ctx, domain.OrderID(id))
		if err != nil {
			return err
		}
		if err := order.ChangeQuantity(lineNo, qty, s.Now()); err != nil {
			return err
		}
		return s.saveAndRecord(ctx, order)
	})
}

func (s *Service) Lock(ctx context.Context, id, reason string) error {
	return s.Tx.WithinTx(ctx, func(ctx context.Context) error {
		order, err := s.Orders.Get(ctx, domain.OrderID(id))
		if err != nil {
			return err
		}
		if err := order.Lock(reason, s.Now()); err != nil {
			return err
		}
		return s.saveAndRecord(ctx, order)
	})
}

func (s *Service) saveWithEvents(ctx context.Context, o *domain.Order) error {
	return s.Tx.WithinTx(ctx, func(ctx context.Context) error { return s.saveAndRecord(ctx, o) })
}

// saveAndRecord:保存聚合和写 outbox 在同一个事务里,保证"改了就一定发出事件"。
func (s *Service) saveAndRecord(ctx context.Context, o *domain.Order) error {
	if err := s.Orders.Save(ctx, o); err != nil {
		return err
	}
	for _, e := range o.PullEvents() {
		name, payload, ok := toContract(e)
		if !ok {
			continue // 不是所有领域事件都需要对外发布
		}
		if err := s.Events.Record(ctx, name, payload, e.OccurredAt()); err != nil {
			return err
		}
	}
	return nil
}

// toContract 把内部领域事件翻译成发布语言,内部模型可以自由演化。
func toContract(e domain.Event) (string, any, bool) {
	switch ev := e.(type) {
	case domain.OrderPlaced:
		lines := make([]contract.PlacedLineV1, 0, len(ev.Lines))
		for _, l := range ev.Lines {
			lines = append(lines, contract.PlacedLineV1{ProductID: string(l.ProductID()), Quantity: l.Quantity()})
		}
		return contract.OrderPlacedV1Name, contract.OrderPlacedV1{
			OrderID: string(ev.OrderID), CustomerID: string(ev.CustomerID), Lines: lines, OccurredAt: ev.At,
		}, true
	default:
		return "", nil, false
	}
}
