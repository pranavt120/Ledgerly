package domain

import (
	"fmt"
	"math"
	"strings"
)

type Currency string

const (
	INR Currency = "INR"
	USD Currency = "USD"
	EUR Currency = "EUR"
)

func (c Currency) Valid() bool {
	switch c {
	case INR, USD, EUR:
		return true

	default:
		return false
	}
}

func ParseCurrency(s string) (Currency, error) {
	c := Currency(s)
	if !c.Valid() {
		return "", fmt.Errorf("parse %q: %w", s, ErrInvalidCurrency)
	}
	return c, nil
}

type Money struct {
	amount   int64 // amount units; unexported so Money is immutable
	currency Currency
}

func NewMoney(amount int64, c Currency) (Money, error) {
	if !c.Valid() {
		return Money{}, fmt.Errorf("money constructor currency validation: %w", ErrInvalidCurrency)
	}

	return Money{
		amount:   amount,
		currency: c,
	}, nil
} // ErrInvalidCurrency if !c.Valid()

// MustMoney instantiates a Money object or panics if the currency is invalid.
// Intended strictly for test suites, seeding data, and code examples.
func MustMoney(amount int64, c Currency) Money {
	m, err := NewMoney(amount, c)
	if err != nil {
		panic(fmt.Errorf("MustMoney dropped on input context: %w", err))
	}
	return m
}

func (m Money) Amount() int64 {
	return m.amount
}
func (m Money) Currency() Currency {
	return m.currency
}
func (m Money) IsZero() bool {
	return m.amount == 0
} // amount == 0
func (m Money) IsNegative() bool {
	return m.amount < 0
}
func (m Money) IsPositive() bool {
	return m.amount > 0
}
func (m Money) Add(o Money) (Money, error) {
	if m.currency != o.currency {
		return Money{}, ErrCurrencyMismatch
	}

	if o.amount > 0 && m.amount > math.MaxInt64-o.amount {
		return Money{}, ErrOverflow
	}
	if o.amount < 0 && m.amount < math.MinInt64-o.amount {
		return Money{}, ErrOverflow
	}

	// 3. Return a brand new instance (Immutable pattern)
	return Money{
		amount:   m.amount + o.amount,
		currency: m.currency,
	}, nil
} // ErrCurrencyMismatch, ErrOverflow

func (m Money) Sub(o Money) (Money, error) {
	if m.currency != o.currency {
		return Money{}, ErrCurrencyMismatch
	}

	// For subtraction, overflow happens if subtracting a negative value pushes past MaxInt64,
	// or subtracting a positive value drops below MinInt64.
	if o.amount < 0 && m.amount > math.MaxInt64+o.amount {
		return Money{}, ErrOverflow
	}
	if o.amount > 0 && m.amount < math.MinInt64+o.amount {
		return Money{}, ErrOverflow
	}

	return Money{
		amount:   m.amount - o.amount,
		currency: m.currency,
	}, nil
} // ErrCurrencyMismatch, ErrOverflow

func (m Money) Neg() (Money, error) {
	if m.amount == math.MinInt64 {
		return Money{}, ErrOverflow
	}

	return Money{
		amount:   -m.amount,
		currency: m.currency,
	}, nil
} // ErrOverflow (think about math.MinInt64)

func (m Money) Cmp(o Money) (int, error) {
	if m.currency != o.currency {
		return 0, ErrCurrencyMismatch
	}

	if m.amount < o.amount {
		return -1, nil
	}
	if m.amount > o.amount {
		return 1, nil
	}
	return 0, nil
} // -1/0/1; ErrCurrencyMismatch

func (m Money) String() string {
	// 1. Separate amount units into major and absolute fractional segments
	major := m.amount / 100
	fractional := m.amount % 100

	// 2. Prevent negative signs from printing on the fractional segment (e.g. avoiding "USD -0.-05")
	if fractional < 0 {
		fractional = -fractional
	}

	// 3. Format components using zero-padded digit restrictions
	// %d prints the raw signed major integer digits.
	// %02d forces the fractional segment to always print exactly two digits with a leading zero if needed.
	return fmt.Sprintf("%s %d.%02d", m.currency, major, fractional)
}

// ParseMoney parses "<CUR> <amount>", e.g. "INR 1234.50", into Money.
// The amount is an optional '-', one or more digits, and optionally '.'
// followed by 1 or 2 digits. It is the inverse of String.
func ParseMoney(s string) (Money, error) {
	curStr, amt, ok := strings.Cut(s, " ")
	if !ok {
		return Money{}, fmt.Errorf("parse money %q: missing space: %w", s, ErrInvalidAmount)
	}

	cur, err := ParseCurrency(curStr)
	if err != nil {
		return Money{}, fmt.Errorf("parse money %q: %w", s, err)
	}

	neg := strings.HasPrefix(amt, "-")
	if neg {
		amt = amt[1:]
	}

	whole, frac, hasDot := strings.Cut(amt, ".")
	if whole == "" || (hasDot && (frac == "" || len(frac) > 2)) {
		return Money{}, fmt.Errorf("parse money %q: %w", s, ErrInvalidAmount)
	}
	// Pad to exactly 2 fractional digits so "12.5" and "12" both become paise:
	// the digits "1250" and "1200" can then be read as one integer.
	frac += strings.Repeat("0", 2-len(frac))

	// Accumulate as a negative number: int64 holds one more negative value
	// than positive (MinInt64 = -MaxInt64-1), so this is the only way to
	// parse MinInt64 without overflowing first and negating later.
	var total int64
	for _, digits := range [2]string{whole, frac} {
		for i := 0; i < len(digits); i++ {
			c := digits[i]
			if c < '0' || c > '9' {
				return Money{}, fmt.Errorf("parse money %q: %w", s, ErrInvalidAmount)
			}
			d := int64(c - '0')
			if total < math.MinInt64/10 || total*10 < math.MinInt64+d {
				return Money{}, fmt.Errorf("parse money %q: %w", s, ErrOverflow)
			}
			total = total*10 - d
		}
	}

	if !neg {
		if total == math.MinInt64 {
			return Money{}, fmt.Errorf("parse money %q: %w", s, ErrOverflow)
		}
		total = -total
	}

	return Money{amount: total, currency: cur}, nil
}
