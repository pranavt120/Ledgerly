package domain

import (
	"context"
)

type Repository interface {
	CreateAccount(ctx context.Context, a *Account) error            // ErrAlreadyExists
	GetAccount(ctx context.Context, id AccountID) (*Account, error) // ErrNotFound
	ListAccounts(ctx context.Context, req PageRequest) (Page[*Account], error)

	// UpdateAccount loads the account, calls fn on it, and saves it only if fn returns nil.
	UpdateAccount(ctx context.Context, id AccountID, fn func(a *Account) error) error

	// ExecTransfer atomically: rejects a reused t.ID (ErrDuplicateTransfer),
	// loads t.From and t.To (ErrNotFound), calls fn, and if fn returns nil
	// saves both accounts, the transfer, and its two entries. If fn fails, nothing is saved.
	ExecTransfer(ctx context.Context, t *Transfer, fn func(from, to *Account) error) error

	ListEntries(ctx context.Context, id AccountID, req PageRequest) (Page[Entry], error)
}
