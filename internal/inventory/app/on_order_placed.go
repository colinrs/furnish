// Package app 是库存上下文的应用层。它通过发布语言(ordering/contract)
// 订阅下单上下文的事件,只依赖 contract,不依赖 ordering 的内部模型。
package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/colinrs/furnish/internal/inventory/domain"
	orderingcontract "github.com/colinrs/furnish/internal/ordering/contract"
)

// ProcessedEvents 记录已处理的消息,保证重复投递时幂等。
// 生产实现应写在同一个数据库事务里(例如 processed_messages 表的唯一索引),
// 这样预留失败回滚时,"已处理"标记也一起回滚。
type ProcessedEvents interface {
	MarkProcessed(ctx context.Context, consumer, key string) (first bool, err error)
}

type Transactor interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// ShortageReporter:库存不足时通知(例如发出 StockShortage 事件让订单锁单)。
type ShortageReporter interface {
	ReportShortage(ctx context.Context, orderID string, sku domain.SKU) error
}

type Reservations struct {
	Stocks    domain.Repository
	Processed ProcessedEvents
	Tx        Transactor
	Shortage  ShortageReporter
}

const consumerName = "inventory.reserve-on-order-placed"

// OnOrderPlaced 是最终一致性的另一半:下单事务已经提交,这里稍后预留库存。
// 每个 SKU 是一个独立聚合,所以每个预留各用一个事务。
func (r *Reservations) OnOrderPlaced(ctx context.Context, eventID int64, payload []byte) error {
	var ev orderingcontract.OrderPlacedV1
	if err := json.Unmarshal(payload, &ev); err != nil {
		return err
	}
	for _, l := range ev.Lines {
		sku := domain.SKU(l.ProductID)            // 这里假设 SKU 与 ProductID 一致;不一致时在此翻译
		key := fmt.Sprintf("%d:%s", eventID, sku) // 按"事件+SKU"粒度去重,部分成功后重试也安全
		err := r.Tx.WithinTx(ctx, func(ctx context.Context) error {
			first, err := r.Processed.MarkProcessed(ctx, consumerName, key)
			if err != nil || !first {
				return err
			}
			st, err := r.Stocks.Get(ctx, sku)
			if err != nil {
				return err
			}
			if err := st.Reserve(l.Quantity); err != nil {
				return err
			}
			return r.Stocks.Save(ctx, st)
		})
		if errors.Is(err, domain.ErrInsufficientStock) {
			if err := r.Shortage.ReportShortage(ctx, ev.OrderID, sku); err != nil {
				return err
			}
			continue
		}
		if err != nil {
			return err
		}
	}
	return nil
}
