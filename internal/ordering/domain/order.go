package domain

import "time"

type (
	OrderID    string
	CustomerID string
	ProductID  string
)

type Status string

const (
	StatusPlaced Status = "placed"
	StatusLocked Status = "locked"
)

const maxQuantityPerLine = 20

// OrderLine 是聚合内部的实体:只有本地标识 lineNo,外部不能长期持有它。
// unitPrice 是下单时的价格快照,商品之后调价不会影响它。
type OrderLine struct {
	lineNo    int
	productID ProductID
	unitPrice Money
	quantity  int
}

func (l OrderLine) LineNo() int          { return l.lineNo }
func (l OrderLine) ProductID() ProductID { return l.productID }
func (l OrderLine) UnitPrice() Money     { return l.unitPrice }
func (l OrderLine) Quantity() int        { return l.quantity }
func (l OrderLine) Subtotal() Money      { return l.unitPrice.Multiply(l.quantity) }

// LineSpec 是创建订单时的输入。
type LineSpec struct {
	ProductID ProductID
	UnitPrice Money
	Quantity  int
}

// Order 是聚合根。字段全部小写(未导出),外部只能通过方法修改,
// 由根负责守住不变量:订单总额不得超过客户信用额度。
type Order struct {
	id          OrderID
	customerID  CustomerID
	lines       []OrderLine
	creditLimit Money
	status      Status
	version     int // 乐观锁版本号,由仓储维护
	events      []Event
}

// PlaceOrder 是工厂函数:要么返回合法的订单,要么返回错误。
func PlaceOrder(id OrderID, customer CustomerID, creditLimit Money, specs []LineSpec, now time.Time) (*Order, error) {
	if len(specs) == 0 {
		return nil, ErrEmptyOrder
	}
	o := &Order{id: id, customerID: customer, creditLimit: creditLimit, status: StatusPlaced}
	for i, s := range specs {
		if err := validQuantity(s.Quantity); err != nil {
			return nil, err
		}
		o.lines = append(o.lines, OrderLine{lineNo: i + 1, productID: s.ProductID, unitPrice: s.UnitPrice, quantity: s.Quantity})
	}
	if err := o.checkCredit(); err != nil {
		return nil, err
	}
	o.record(OrderPlaced{OrderID: o.id, CustomerID: o.customerID, Lines: o.Lines(), Total: o.Total(), At: now})
	return o, nil
}

// Reconstitute 供仓储从数据库重建聚合,不产生领域事件。
func Reconstitute(id OrderID, customer CustomerID, creditLimit Money, status Status, version int, lines []OrderLine) *Order {
	return &Order{id: id, customerID: customer, creditLimit: creditLimit, status: status, version: version, lines: lines}
}

// NewOrderLineSnapshot 供仓储重建内部实体。
func NewOrderLineSnapshot(lineNo int, productID ProductID, unitPrice Money, quantity int) OrderLine {
	return OrderLine{lineNo: lineNo, productID: productID, unitPrice: unitPrice, quantity: quantity}
}

func (o *Order) ChangeQuantity(lineNo, qty int, now time.Time) error {
	if o.status == StatusLocked {
		return ErrOrderLocked
	}
	if err := validQuantity(qty); err != nil {
		return err
	}
	idx := -1
	for i := range o.lines {
		if o.lines[i].lineNo == lineNo {
			idx = i
		}
	}
	if idx < 0 {
		return ErrOrderLineNotFound
	}
	old := o.lines[idx].quantity
	o.lines[idx].quantity = qty
	if err := o.checkCredit(); err != nil {
		o.lines[idx].quantity = old // 不变量不满足时回滚,聚合保持一致
		return err
	}
	o.record(OrderLineQuantityChanged{OrderID: o.id, LineNo: lineNo, ProductID: o.lines[idx].productID, From: old, To: qty, At: now})
	return nil
}

// Lock 对应运营说的"锁单"。
func (o *Order) Lock(reason string, now time.Time) error {
	if o.status == StatusLocked {
		return nil // 幂等
	}
	o.status = StatusLocked
	o.record(OrderLocked{OrderID: o.id, Reason: reason, At: now})
	return nil
}

func (o *Order) Total() Money {
	total := Zero(o.creditLimit.Currency())
	for _, l := range o.lines {
		total = total.Add(l.Subtotal())
	}
	return total
}

func (o *Order) ID() OrderID            { return o.id }
func (o *Order) CustomerID() CustomerID { return o.customerID }
func (o *Order) CreditLimit() Money     { return o.creditLimit }
func (o *Order) Status() Status         { return o.status }
func (o *Order) Version() int           { return o.version }
func (o *Order) IsLocked() bool         { return o.status == StatusLocked }

// Lines 返回副本,外部拿到的只是快照,改不到聚合内部。
func (o *Order) Lines() []OrderLine {
	out := make([]OrderLine, len(o.lines))
	copy(out, o.lines)
	return out
}

// PullEvents 取出并清空本次操作产生的领域事件,由应用层写入 outbox。
func (o *Order) PullEvents() []Event {
	ev := o.events
	o.events = nil
	return ev
}

func (o *Order) checkCredit() error {
	if o.Total().GreaterThan(o.creditLimit) {
		return ErrCreditLimitExceeded
	}
	return nil
}

func (o *Order) record(e Event) { o.events = append(o.events, e) }

func validQuantity(q int) error {
	if q < 1 || q > maxQuantityPerLine {
		return ErrInvalidQuantity
	}
	return nil
}
