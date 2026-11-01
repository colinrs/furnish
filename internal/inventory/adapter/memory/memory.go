// Package memory 是库存上下文的内存实现,用于本地演示和测试。
// 生产环境按 ordering/adapter/postgres 的写法换成 GORM 实现即可。
package memory

import (
	"context"
	"log/slog"
	"sync"

	"github.com/colinrs/furnish/internal/inventory/domain"
)

type Stocks struct {
	mu sync.Mutex
	m  map[domain.SKU]*domain.Stock
}

func NewStocks(seed ...*domain.Stock) *Stocks {
	s := &Stocks{m: map[domain.SKU]*domain.Stock{}}
	for _, st := range seed {
		s.m[st.SKU()] = st
	}
	return s
}

func (s *Stocks) Get(_ context.Context, sku domain.SKU) (*domain.Stock, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.m[sku]
	if !ok {
		return nil, domain.ErrStockNotFound
	}
	return domain.Reconstitute(st.SKU(), st.OnHand(), st.Reserved(), st.Version()), nil
}

func (s *Stocks) Save(_ context.Context, st *domain.Stock) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[st.SKU()] = domain.Reconstitute(st.SKU(), st.OnHand(), st.Reserved(), st.Version()+1)
	return nil
}

type Processed struct {
	mu sync.Mutex
	m  map[string]bool
}

func NewProcessed() *Processed { return &Processed{m: map[string]bool{}} }

func (p *Processed) MarkProcessed(_ context.Context, consumer, key string) (bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	k := consumer + ":" + key
	if p.m[k] {
		return false, nil
	}
	p.m[k] = true
	return true, nil
}

type NoTx struct{}

func (NoTx) WithinTx(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }

type LogShortage struct{}

func (LogShortage) ReportShortage(ctx context.Context, orderID string, sku domain.SKU) error {
	slog.WarnContext(ctx, "stock shortage", "order_id", orderID, "sku", sku)
	return nil
}
