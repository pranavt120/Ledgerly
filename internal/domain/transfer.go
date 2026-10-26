package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

type TransferID string

type Transfer struct {
	ID      TransferID
	From    AccountID
	To      AccountID
	Amount  Money    // always positive
	Entries [2]Entry // [0] = debit on From, [1] = credit on To
	Timestamps
}

type Entry struct {
	TransferID TransferID
	AccountID  AccountID
	Amount     Money // debit is negative, credit is positive
	CreatedAt  time.Time
}

func NewTransfer(id TransferID, from, to AccountID, amount Money, now time.Time) (*Transfer, error) {
	var errs []error // Corrected type interface instantiation

	// 1. Structural Validation Rules
	if strings.TrimSpace(string(id)) == "" {
		errs = append(errs, ValidationError{Field: "id", Reason: "id must not be empty"})
	}

	if strings.TrimSpace(string(from)) == "" {
		errs = append(errs, ValidationError{Field: "from", Reason: "from must not be empty"})
	}

	if strings.TrimSpace(string(to)) == "" { // Fixed single = assignment typo
		errs = append(errs, ValidationError{Field: "to", Reason: "to must not be empty"})
	}

	// 2. Core Business Invariants
	if from == to && from != "" {
		errs = append(errs, ErrSameAccount)
	}

	if amount.Amount() <= 0 { // Money must be strictly positive to initiate a transaction
		errs = append(errs, ErrInvalidAmount)
	}

	// If any error conditions matched above, fail-fast and return the joined slice
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	debitAmount, err := amount.Neg()
	if err != nil {
		return nil, fmt.Errorf("failed to process debit inversion: %w", err) // Captates edge-case integer overflows
	}

	transfer := &Transfer{
		ID:     id,
		From:   from,
		To:     to,
		Amount: amount,
		Entries: [2]Entry{
			0: { // Array key mapping index [0] = Debit on Source Account
				TransferID: id,
				AccountID:  from,
				Amount:     debitAmount,
				CreatedAt:  now,
			},
			1: { // Array key mapping index [1] = Credit on Destination Account
				TransferID: id,
				AccountID:  to,
				Amount:     amount,
				CreatedAt:  now,
			},
		},
		Timestamps: Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	return transfer, nil
}

func ApplyTransfer(t *Transfer, from, to *Account, now time.Time) error {

	if t.Amount.currency != from.Currency || t.Amount.currency != to.Currency {
		return fmt.Errorf("transfer %s: %w", t.ID, ErrCurrencyMismatch)
	}

	if from.Status == StatusFrozen {
		return fmt.Errorf("debit accout %s: %w", from.ID, ErrAccountFrozen)
	}

	if from.Status == StatusClosed {
		return fmt.Errorf("debit account %s: %w", from.ID, ErrAccountClosed)
	}

	if to.Status == StatusFrozen {
		return fmt.Errorf("credit accout %s: %w", to.ID, ErrAccountFrozen)
	}

	if to.Status == StatusClosed {
		return fmt.Errorf("credit account %s: %w", to.ID, ErrAccountClosed)
	}

	if from.ID != t.From || to.ID != t.To {
		return fmt.Errorf("transfer %s: accounts %s→%s do not match %s→%s", t.ID, from.ID, to.ID, t.From, t.To)
	}

	debit, err := from.Balance.Sub(t.Amount)
	if err != nil {
		return fmt.Errorf("debit account %s: %w", from.ID, err)
	}

	if !from.System && debit.IsNegative() {
		return fmt.Errorf("debit account %s: %w", from.ID, ErrInsufficientFunds)
	}

	credit, creditErr := to.Balance.Add(t.Amount)

	if creditErr != nil {
		return fmt.Errorf("credit account %s: %w", to.ID, creditErr)
	}

	from.Balance, to.Balance = debit, credit
	from.Touch(now)
	to.Touch(now)

	return nil

}
