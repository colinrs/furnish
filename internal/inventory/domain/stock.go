// Package domain 是库存上下文的模型。注意这里的"商品"叫 SKU,
// 关心的是可用数和预留数,和下单上下文的 ProductID 是不同的概念。
package domain

import (
	"context"
	"errors"
)

var (
	ErrInsufficientStock = errors.New("insufficient stock")
	ErrStockNotFound     = errors.New("stock not found")
)

type SKU string

// Stock 是聚合根,一个 SKU 一个聚合(小聚合)。
type Stock struct {
	sku      SKU
	onHand   int
	reserved int
	version  int
}

func NewStock(sku SKU, onHand int) *Stock { return &Stock{sku: sku, onHand: onHand} }

func Reconstitute(sku SKU, onHand, reserved, version int) *Stock {
	return &Stock{sku: sku, onHand: onHand, reserved: reserved, version: version}
}

func (s *Stock) Available() int { return s.onHand - s.reserved }

// Reserve 不变量:预留数不能超过在库数。
func (s *Stock) Reserve(qty int) error {
	if qty > s.Available() {
		return ErrInsufficientStock
	}
	s.reserved += qty
	return nil
}

func (s *Stock) SKU() SKU      { return s.sku }
func (s *Stock) OnHand() int   { return s.onHand }
func (s *Stock) Reserved() int { return s.reserved }
func (s *Stock) Version() int  { return s.version }

type Repository interface {
	Get(ctx context.Context, sku SKU) (*Stock, error)
	Save(ctx context.Context, s *Stock) error
}
