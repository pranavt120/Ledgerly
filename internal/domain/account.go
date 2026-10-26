package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

type AccountID string

type AccountStatus int

const (
	StatusActive AccountStatus = iota + 1 // zero value is deliberately invalid
	StatusFrozen
	StatusClosed
)

func (s AccountStatus) String() string {
	switch s {
	case 1:
		return "active"
	case 2:
		return "frozen"
	case 3:
		return "closed"
	default:
		return fmt.Sprintf("AccountStatus(%d)", s)
	}
} // "active" / "frozen" / "closed" / "AccountStatus(7)"

type Account struct {
	ID         AccountID
	Owner      string
	Currency   Currency
	Status     AccountStatus
	Balance    Money
	System     bool // only system accounts may go negative
	Timestamps      // embedded
}

func NewAccount(id AccountID, owner string, c Currency, now time.Time) (*Account, error) {
	var errs []error

	if id == "" {
		errs = append(errs, ValidationError{Field: "id", Reason: "must not be empty"})
	}
	if strings.TrimSpace(owner) == "" {
		errs = append(errs, ValidationError{Field: "owner", Reason: "must not be empty"})
	}
	if len(owner) > 100 {
		errs = append(errs, ValidationError{Field: "owner", Reason: "must not be more that 100 characters"})
	}
	if !c.Valid() {
		errs = append(errs, ValidationError{Field: "currency", Reason: "invalid currency code"})
	}

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	return &Account{
		ID:       id,
		Owner:    owner,
		Currency: c,
		Status:   StatusActive,
		Balance:  MustMoney(0, c), // Match structural invariant rules
		System:   false,
		Timestamps: Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}, nil

}
func (a *Account) Freeze(now time.Time) error {
	if a.Status == StatusClosed {
		return fmt.Errorf("cannot freeze account %s: %w", a.ID, ErrAccountClosed)
	}
	if a.Status == StatusFrozen {
		return nil // Idempotent transition validation success
	}

	a.Status = StatusFrozen
	a.Touch(now) // Executes embedded structural accessor method directly
	return nil
}
func (a *Account) Unfreeze(now time.Time) error {
	if a.Status == StatusClosed {
		return fmt.Errorf("cannot Unfreeze account %s: %w", a.ID, ErrAccountClosed)
	}
	if a.Status == StatusActive {
		return nil
	}

	a.Status = StatusActive
	a.Touch(now) // Executes embedded structural accessor method directly
	return nil
}
func (a *Account) Close(now time.Time) error {
	if a.Balance.Amount() != 0 {
		return ErrNonZeroBalance
	}
	if a.Status == StatusFrozen {
		return fmt.Errorf("cannot Close account %s: %w", a.ID, ErrAccountFrozen)
	}
	if a.Status == StatusClosed {
		return nil
	}

	a.Status = StatusClosed
	a.Touch(now) // Executes embedded structural accessor method directly
	return nil
}
