package app

import (
	"context"
	"time"

	"github.com/colinrs/furnish/internal/ordering/domain"
)

// 端口(ports):应用层需要外部世界提供的能力,用接口表达。
// Go 的习惯是"接口由使用方定义",所以它们写在这里,而不是写在实现方。

// Catalog 提供下单时刻的商品价格,由 adapter/catalogacl 通过防腐层实现。
type Catalog interface {
	UnitPrice(ctx context.Context, id domain.ProductID) (domain.Money, error)
}

// CreditPolicy 提供客户的信用额度。
type CreditPolicy interface {
	LimitOf(ctx context.Context, id domain.CustomerID) (domain.Money, error)
}

// Transactor 定义事务边界:一个事务只修改一个聚合 + 写 outbox。
type Transactor interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// EventRecorder 在同一事务内把集成事件写入 outbox 表。
type EventRecorder interface {
	Record(ctx context.Context, name string, payload any, occurredAt time.Time) error
}

// IdempotencyGuard 防止同一请求被重复处理(由 Redis SETNX 实现)。
type IdempotencyGuard interface {
	Claim(ctx context.Context, key string, ttl time.Duration) (bool, error)
}

type IDGenerator func() domain.OrderID
type Clock func() time.Time
