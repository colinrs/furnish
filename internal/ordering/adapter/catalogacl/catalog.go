// Package catalogacl 是防腐层(Anticorruption Layer):
// 老 ERP 用物料号 MATNR、字符串金额和自己的状态码,
// 这里把它们翻译成下单上下文的 ProductID 和 Money,老模型不会渗进 domain。
package catalogacl

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/colinrs/furnish/internal/ordering/domain"
)

var ErrProductNotSellable = errors.New("product is not sellable")

// LegacyERP 描述老系统的接口形状(遵循它原本的命名,不去美化)。
type LegacyERP interface {
	GetMaterial(ctx context.Context, matnr string) (Material, error)
}

type Material struct {
	MATNR string
	NETPR string // 净价,形如 "2999.00"
	WAERS string // 币种
	MSTAE string // 物料状态:"01" 可售,"99" 停售
}

// Catalog 实现 app.Catalog 端口。
type Catalog struct {
	erp LegacyERP
	// 我们的 SKU 与 ERP 物料号不一致时,在这里做映射
	toMATNR func(domain.ProductID) string
}

func New(erp LegacyERP, toMATNR func(domain.ProductID) string) *Catalog {
	return &Catalog{erp: erp, toMATNR: toMATNR}
}

func (c *Catalog) UnitPrice(ctx context.Context, id domain.ProductID) (domain.Money, error) {
	m, err := c.erp.GetMaterial(ctx, c.toMATNR(id))
	if err != nil {
		return domain.Money{}, fmt.Errorf("erp get material %s: %w", id, err)
	}
	if m.MSTAE != "01" {
		return domain.Money{}, ErrProductNotSellable
	}
	cents, err := parseCents(m.NETPR)
	if err != nil {
		return domain.Money{}, fmt.Errorf("erp price %q: %w", m.NETPR, err)
	}
	return domain.NewMoney(cents, m.WAERS)
}

func parseCents(s string) (int64, error) {
	whole, frac, _ := strings.Cut(strings.TrimSpace(s), ".")
	frac = (frac + "00")[:2]
	w, err := strconv.ParseInt(whole, 10, 64)
	if err != nil {
		return 0, err
	}
	f, err := strconv.ParseInt(frac, 10, 64)
	if err != nil {
		return 0, err
	}
	return w*100 + f, nil
}
