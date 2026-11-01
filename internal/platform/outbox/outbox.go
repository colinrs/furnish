// Package outbox 实现事务性发件箱(Transactional Outbox):
// 1. 应用层在修改聚合的同一个事务里,把事件写进 outbox 表;
// 2. Relay 在后台轮询未发布的事件,交给订阅者处理,成功后标记已发布。
// 这样"数据改了但事件丢了"的情况不会发生。消费者需要按事件 ID 做幂等。
package outbox

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/colinrs/furnish/internal/platform/tx"
)

type record struct {
	ID          int64 `gorm:"primaryKey;autoIncrement"`
	Name        string
	Payload     []byte `gorm:"type:jsonb"`
	OccurredAt  time.Time
	PublishedAt *time.Time
}

func (record) TableName() string { return "outbox" }

// Recorder 实现 ordering/app.EventRecorder(结构上满足,不需要 import 它)。
type Recorder struct{ db *gorm.DB }

func NewRecorder(db *gorm.DB) *Recorder { return &Recorder{db: db} }

func (r *Recorder) Record(ctx context.Context, name string, payload any, occurredAt time.Time) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return tx.DB(ctx, r.db).Create(&record{Name: name, Payload: b, OccurredAt: occurredAt}).Error
}

// Handler 处理一条事件;返回错误时该事件下次重试。
type Handler func(ctx context.Context, eventID int64, payload []byte) error

type Relay struct {
	db       *gorm.DB
	handlers map[string][]Handler
	batch    int
}

func NewRelay(db *gorm.DB) *Relay {
	return &Relay{db: db, handlers: map[string][]Handler{}, batch: 100}
}

func (r *Relay) Subscribe(name string, h Handler) { r.handlers[name] = append(r.handlers[name], h) }

func (r *Relay) Run(ctx context.Context, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := r.dispatchOnce(ctx); err != nil {
				slog.ErrorContext(ctx, "outbox dispatch failed", "err", err)
			}
		}
	}
}

func (r *Relay) dispatchOnce(ctx context.Context) error {
	return r.db.WithContext(ctx).Transaction(func(db *gorm.DB) error {
		var recs []record
		// SKIP LOCKED:多实例部署时各自领取不同的事件
		if err := db.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("published_at IS NULL").Order("id").Limit(r.batch).Find(&recs).Error; err != nil {
			return err
		}
		for _, rec := range recs {
			for _, h := range r.handlers[rec.Name] {
				if err := h(ctx, rec.ID, rec.Payload); err != nil {
					return err // 回滚,整批稍后重试;消费者幂等保证重复投递无害
				}
			}
			now := time.Now()
			if err := db.Model(&record{}).Where("id = ?", rec.ID).Update("published_at", &now).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
