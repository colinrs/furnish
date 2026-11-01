// Package credit 是信用额度端口的简单实现。真实系统里它可能调用会员上下文。
package credit

import (
	"context"

	"github.com/colinrs/furnish/internal/ordering/domain"
)

type Fixed struct{ Limit domain.Money }

func (f Fixed) LimitOf(context.Context, domain.CustomerID) (domain.Money, error) { return f.Limit, nil }
