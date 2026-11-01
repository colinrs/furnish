// Package postgres 是出站适配器:用 GORM 实现 domain.Repository。
// 持久化模型(PO)和领域模型分开:GORM 标签、表结构的变化不会影响 domain 包。
package postgres

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/colinrs/furnish/internal/ordering/domain"
	"github.com/colinrs/furnish/internal/platform/tx"
)

type orderPO struct {
	ID          string `gorm:"primaryKey"`
	CustomerID  string
	Status      string
	CreditLimit int64
	Currency    string
	Version     int
	Lines       []orderLinePO `gorm:"foreignKey:OrderID;references:ID"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (orderPO) TableName() string { return "ordering_orders" }

type orderLinePO struct {
	OrderID   string `gorm:"primaryKey"`
	LineNo    int    `gorm:"primaryKey"`
	ProductID string
	UnitPrice int64
	Quantity  int
}

func (orderLinePO) TableName() string { return "ordering_order_lines" }

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

var _ domain.Repository = (*Repository)(nil)

func (r *Repository) Get(ctx context.Context, id domain.OrderID) (*domain.Order, error) {
	var po orderPO
	err := tx.DB(ctx, r.db).Preload("Lines").First(&po, "id = ?", string(id)).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrOrderNotFound
	}
	if err != nil {
		return nil, err
	}
	return toDomain(po)
}

// Save 整体保存聚合。用 version 列做乐观锁:
// UPDATE ... WHERE id = ? AND version = ?,影响 0 行说明别人先改了。
func (r *Repository) Save(ctx context.Context, o *domain.Order) error {
	db := tx.DB(ctx, r.db)
	po := toPO(o)
	if o.Version() == 0 {
		po.Version = 1
		return db.Create(&po).Error // 级联插入 Lines
	}
	res := db.Model(&orderPO{}).
		Where("id = ? AND version = ?", po.ID, o.Version()).
		Updates(map[string]any{"status": po.Status, "version": o.Version() + 1, "updated_at": time.Now()})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrConcurrentModification
	}
	// 聚合内部的订单行随根一起替换
	if err := db.Where("order_id = ?", po.ID).Delete(&orderLinePO{}).Error; err != nil {
		return err
	}
	return db.Create(&po.Lines).Error
}

func toPO(o *domain.Order) orderPO {
	po := orderPO{
		ID: string(o.ID()), CustomerID: string(o.CustomerID()), Status: string(o.Status()),
		CreditLimit: o.CreditLimit().Amount(), Currency: o.CreditLimit().Currency(), Version: o.Version(),
	}
	for _, l := range o.Lines() {
		po.Lines = append(po.Lines, orderLinePO{
			OrderID: po.ID, LineNo: l.LineNo(), ProductID: string(l.ProductID()),
			UnitPrice: l.UnitPrice().Amount(), Quantity: l.Quantity(),
		})
	}
	return po
}

func toDomain(po orderPO) (*domain.Order, error) {
	limit, err := domain.NewMoney(po.CreditLimit, po.Currency)
	if err != nil {
		return nil, err
	}
	lines := make([]domain.OrderLine, 0, len(po.Lines))
	for _, l := range po.Lines {
		price, err := domain.NewMoney(l.UnitPrice, po.Currency)
		if err != nil {
			return nil, err
		}
		lines = append(lines, domain.NewOrderLineSnapshot(l.LineNo, domain.ProductID(l.ProductID), price, l.Quantity))
	}
	return domain.Reconstitute(domain.OrderID(po.ID), domain.CustomerID(po.CustomerID), limit,
		domain.Status(po.Status), po.Version, lines), nil
}
