package domain

import "context"

// Repository 是仓储接口:定义在领域层,实现在 adapter/postgres。
// 只为聚合根 Order 提供,没有 OrderLineRepository。
type Repository interface {
	Get(ctx context.Context, id OrderID) (*Order, error)
	// Save 新建或更新整个聚合;版本号不匹配时返回 ErrConcurrentModification。
	Save(ctx context.Context, o *Order) error
}
