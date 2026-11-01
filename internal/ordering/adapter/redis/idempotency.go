// Package redis 用 Redis 实现幂等键。
// Redis 在这里只做辅助(幂等、缓存),业务数据的唯一事实来源是 PostgreSQL。
package redis

import (
	"context"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

type IdempotencyGuard struct{ rdb *goredis.Client }

func NewIdempotencyGuard(rdb *goredis.Client) *IdempotencyGuard { return &IdempotencyGuard{rdb: rdb} }

// Claim 第一次调用返回 true;同一个 key 在 ttl 内再次调用返回 false。
func (g *IdempotencyGuard) Claim(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	return g.rdb.SetNX(ctx, "idem:"+key, 1, ttl).Result()
}
