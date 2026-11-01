package domain

import "errors"

// 领域错误用通用语言命名,应用层和 HTTP 适配器据此决定如何响应。
var (
	ErrEmptyOrder             = errors.New("order must contain at least one line")
	ErrInvalidQuantity        = errors.New("quantity must be between 1 and 20")
	ErrCreditLimitExceeded    = errors.New("order total exceeds customer credit limit")
	ErrOrderLocked            = errors.New("order is locked")
	ErrOrderLineNotFound      = errors.New("order line not found")
	ErrOrderNotFound          = errors.New("order not found")
	ErrConcurrentModification = errors.New("order was modified by someone else, retry")
)
