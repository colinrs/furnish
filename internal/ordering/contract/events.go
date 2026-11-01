// Package contract 是下单上下文对外的"发布语言":
// 其他上下文只能依赖这个包,不能 import ordering 的 domain/app/adapter。
// 结构带版本号,字段只增不改,变更时新增 V2。
package contract

import "time"

const OrderPlacedV1Name = "ordering.OrderPlaced.v1"

type OrderPlacedV1 struct {
	OrderID    string         `json:"order_id"`
	CustomerID string         `json:"customer_id"`
	Lines      []PlacedLineV1 `json:"lines"`
	OccurredAt time.Time      `json:"occurred_at"`
}

type PlacedLineV1 struct {
	ProductID string `json:"product_id"`
	Quantity  int    `json:"quantity"`
}
