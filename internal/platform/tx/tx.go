// Package tx 提供基于 GORM 的事务边界,把事务放在 context 里传递,
// 这样仓储和 outbox 自动参与同一个事务,应用层不需要 import gorm。
package tx

import (
	"context"

	"gorm.io/gorm"
)

type ctxKey struct{}

type Transactor struct{ db *gorm.DB }

func NewTransactor(db *gorm.DB) *Transactor { return &Transactor{db: db} }

func (t *Transactor) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if _, ok := ctx.Value(ctxKey{}).(*gorm.DB); ok {
		return fn(ctx) // 已在事务中,复用
	}
	return t.db.WithContext(ctx).Transaction(func(txdb *gorm.DB) error {
		return fn(context.WithValue(ctx, ctxKey{}, txdb))
	})
}

// DB 返回当前事务;不在事务中时返回普通连接。
func DB(ctx context.Context, fallback *gorm.DB) *gorm.DB {
	if txdb, ok := ctx.Value(ctxKey{}).(*gorm.DB); ok {
		return txdb
	}
	return fallback.WithContext(ctx)
}
