package domain

import (
	"errors"
	"fmt"
)

var (
	ErrInvalidCurrency   = errors.New("invalid currency")
	ErrInvalidAmount     = errors.New("invalid amount")
	ErrOverflow          = errors.New("arithmetic overflow")
	ErrCurrencyMismatch  = errors.New("currency mismatch")
	ErrInsufficientFunds = errors.New("insufficient funds")
	ErrAccountFrozen     = errors.New("account frozen")
	ErrAccountClosed     = errors.New("account closed")
	ErrInvalidTransition = errors.New("invalid status transition")
	ErrNonZeroBalance    = errors.New("account balance must be zero")
	ErrSameAccount       = errors.New("source and destination accounts must be different")
	ErrNotFound          = errors.New("resource not found")
	ErrAlreadyExists     = errors.New("resource already exists")
	ErrDuplicateTransfer = errors.New("duplicate transfer reference")
)

type ValidationError struct {
	Field  string
	Reason string
}

func (ve ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", ve.Field, ve.Reason)
}
