package domain

import "fmt"

// Money 是值对象:没有标识、不可变,所有运算都返回新值(闭合操作)。
// 金额用最小货币单位(分)保存,避免浮点误差。
type Money struct {
	amount   int64
	currency string
}

func NewMoney(amount int64, currency string) (Money, error) {
	if currency == "" {
		return Money{}, fmt.Errorf("money: currency is required")
	}
	if amount < 0 {
		return Money{}, fmt.Errorf("money: amount must not be negative")
	}
	return Money{amount: amount, currency: currency}, nil
}

// MustMoney 只用于测试和常量构造。
func MustMoney(amount int64, currency string) Money {
	m, err := NewMoney(amount, currency)
	if err != nil {
		panic(err)
	}
	return m
}

func Zero(currency string) Money { return Money{currency: currency} }

func (m Money) Amount() int64    { return m.amount }
func (m Money) Currency() string { return m.currency }

// Add 是闭合操作:Money + Money = Money。
// 币种不同属于编程错误(调用方应先换算),因此直接 panic。
func (m Money) Add(o Money) Money {
	m.mustSameCurrency(o)
	return Money{amount: m.amount + o.amount, currency: m.currency}
}

func (m Money) Multiply(n int) Money {
	return Money{amount: m.amount * int64(n), currency: m.currency}
}

func (m Money) GreaterThan(o Money) bool {
	m.mustSameCurrency(o)
	return m.amount > o.amount
}

func (m Money) Equals(o Money) bool { return m == o }

func (m Money) String() string {
	return fmt.Sprintf("%s %d.%02d", m.currency, m.amount/100, m.amount%100)
}

func (m Money) mustSameCurrency(o Money) {
	if m.currency != o.currency {
		panic(fmt.Sprintf("money: currency mismatch %s vs %s", m.currency, o.currency))
	}
}
