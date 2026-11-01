package domain

import "time"

// Event 是领域事件:过去时命名,描述领域里已经发生的事实。
type Event interface {
	EventName() string
	OccurredAt() time.Time
}

type OrderPlaced struct {
	OrderID    OrderID
	CustomerID CustomerID
	Lines      []OrderLine
	Total      Money
	At         time.Time
}

func (e OrderPlaced) EventName() string     { return "ordering.OrderPlaced" }
func (e OrderPlaced) OccurredAt() time.Time { return e.At }

type OrderLineQuantityChanged struct {
	OrderID   OrderID
	LineNo    int
	ProductID ProductID
	From, To  int
	At        time.Time
}

func (e OrderLineQuantityChanged) EventName() string     { return "ordering.OrderLineQuantityChanged" }
func (e OrderLineQuantityChanged) OccurredAt() time.Time { return e.At }

type OrderLocked struct {
	OrderID OrderID
	Reason  string
	At      time.Time
}

func (e OrderLocked) EventName() string     { return "ordering.OrderLocked" }
func (e OrderLocked) OccurredAt() time.Time { return e.At }
