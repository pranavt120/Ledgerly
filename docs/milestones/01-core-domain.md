# Milestone 1 spec: Core domain

Every business decision is made here. Your job is the engineering: implement exactly this, in `internal/domain/`, with no I/O and no third-party dependencies.

Build in this order. Each file should compile and have passing tests before you start the next.

| # | File | Contents |
| --- | --- | --- |
| 1 | `errors.go` | Sentinel errors, `ValidationError` |
| 2 | `money.go` | `Currency`, `Money`, `ParseMoney` |
| 3 | `timestamps.go` | `Timestamps` |
| 4 | `account.go` | `AccountID`, `AccountStatus`, `Account` |
| 5 | `transfer.go` | `Transfer`, `Entry`, `ApplyTransfer` |
| 6 | `generics.go` | `Result[T]`, `Page[T]`, `Map`, `Filter`, `GroupBy` |
| 7 | `repository.go` | `Repository` interface |
| 8 | `memory.go` | `MemoryRepository` |
| 9 | `ledger.go` | `Ledger`, the service everything later calls |

---

## 1. `errors.go`

Sentinels (all `errors.New`, all exported):

```
ErrInvalidCurrency   ErrInvalidAmount     ErrOverflow
ErrCurrencyMismatch  ErrInsufficientFunds ErrAccountFrozen
ErrAccountClosed     ErrInvalidTransition ErrNonZeroBalance
ErrSameAccount       ErrNotFound          ErrAlreadyExists
ErrDuplicateTransfer
```

```go
type ValidationError struct {
    Field  string // e.g. "owner"
    Reason string // e.g. "must not be empty"
}
```

- `Error()` returns `"<field>: <reason>"`, e.g. `owner: must not be empty`.
- When several fields are invalid, return **all** of them with `errors.Join`, not just the first.
- A caller must be able to get the first one back out with `errors.As(err, &ve)`.

Whenever you return a sentinel, add context by wrapping it with `%w`, for example `fmt.Errorf("debit account %s: %w", id, ErrAccountFrozen)`. `errors.Is` must still match.

---

## 2. `money.go`

### Currency

```go
type Currency string

const (
    INR Currency = "INR"
    USD Currency = "USD"
    EUR Currency = "EUR"
)

func (c Currency) Valid() bool
func ParseCurrency(s string) (Currency, error) // exact match, case-sensitive; else ErrInvalidCurrency
```

Every currency has 2 decimal places (1 rupee = 100 paise).

### Money

```go
type Money struct {
    amount   int64    // minor units; unexported so Money is immutable
    currency Currency
}

func NewMoney(minor int64, c Currency) (Money, error) // ErrInvalidCurrency if !c.Valid()
func MustMoney(minor int64, c Currency) Money          // panics; tests and examples only
func (m Money) Amount() int64
func (m Money) Currency() Currency
func (m Money) IsZero() bool      // amount == 0
func (m Money) IsNegative() bool
func (m Money) IsPositive() bool
func (m Money) Add(o Money) (Money, error) // ErrCurrencyMismatch, ErrOverflow
func (m Money) Sub(o Money) (Money, error) // ErrCurrencyMismatch, ErrOverflow
func (m Money) Neg() (Money, error)        // ErrOverflow (think about math.MinInt64)
func (m Money) Cmp(o Money) (int, error)   // -1/0/1; ErrCurrencyMismatch
func (m Money) String() string
func ParseMoney(s string) (Money, error)
```

Rules:

- Never use `float64` anywhere in this package, not even in parsing.
- Overflow must be detected *before* it happens. Go's `int64` silently wraps around.

### `String()` and `ParseMoney` format

The format is `<CUR> <amount>`: one space, then the amount with exactly 2 decimals.

| Money | `String()` |
| --- | --- |
| `MustMoney(123450, INR)` | `INR 1234.50` |
| `MustMoney(5, INR)` | `INR 0.05` |
| `MustMoney(-5, USD)` | `USD -0.05` |
| `MustMoney(0, EUR)` | `EUR 0.00` |
| `MustMoney(math.MinInt64, INR)` | `INR -92233720368547758.08` |

`ParseMoney` accepts `<CUR> <amount>`, where the amount matches `-?[0-9]+(\.[0-9]{1,2})?`. Don't use `regexp`; parse it by hand.

| Input | Result |
| --- | --- |
| `INR 1234.50` | 123450 INR |
| `INR 12.5` | 1250 INR |
| `INR 7` | 700 INR |
| `INR -0.05` | -5 INR |
| `INR -92233720368547758.08` | `math.MinInt64` (must work) |
| `INR 92233720368547758.08` | `ErrOverflow` |
| `inr 1.00`, `XYZ 1.00` | `ErrInvalidCurrency` |
| `INR 1.234`, `INR .5`, `INR 5.`, `INR +5`, `INR 1,000`, `INR  5`, `INR`, `""` | `ErrInvalidAmount` |

Round-trip property (this is what the fuzz test checks): if `ParseMoney(s)` succeeds with `m`, then `ParseMoney(m.String())` must return exactly `m`.

---

## 3. `timestamps.go`

```go
type Timestamps struct {
    CreatedAt time.Time
    UpdatedAt time.Time
}

func NewTimestamps(now time.Time) Timestamps // both fields = now
func (t *Timestamps) Touch(now time.Time)    // UpdatedAt = now
```

The domain never calls `time.Now()`. Time is always passed in, which is what makes it testable.

---

## 4. `account.go`

```go
type AccountID string

type AccountStatus int

const (
    StatusActive AccountStatus = iota + 1 // zero value is deliberately invalid
    StatusFrozen
    StatusClosed
)

func (s AccountStatus) String() string // "active" / "frozen" / "closed" / "AccountStatus(7)"

type Account struct {
    ID       AccountID
    Owner    string
    Currency Currency
    Status   AccountStatus
    Balance  Money
    System   bool // only system accounts may go negative
    Timestamps    // embedded
}

func NewAccount(id AccountID, owner string, c Currency, now time.Time) (*Account, error)
func (a *Account) Freeze(now time.Time) error
func (a *Account) Unfreeze(now time.Time) error
func (a *Account) Close(now time.Time) error
```

`NewAccount` validation. Collect every failure into one `errors.Join` of `*ValidationError`s:

| Field | Rule | Reason text |
| --- | --- | --- |
| `id` | not empty | `must not be empty` |
| `owner` | not empty after `strings.TrimSpace`; at most 100 runes | `must not be empty` / `must be at most 100 characters` |
| `currency` | `Valid()` | `unsupported currency "XYZ"` |

A new account starts `Active`, with a zero balance in its own currency and `System: false`.

Status transitions. Anything not listed returns `ErrInvalidTransition`:

| From | Method | To | Extra rule |
| --- | --- | --- | --- |
| Active | `Freeze` | Frozen | |
| Frozen | `Unfreeze` | Active | |
| Active or Frozen | `Close` | Closed | balance must be zero, else `ErrNonZeroBalance` |

Every successful transition calls `Touch(now)`.

---

## 5. `transfer.go`

```go
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

func NewTransfer(id TransferID, from, to AccountID, amount Money, now time.Time) (*Transfer, error)
func ApplyTransfer(t *Transfer, from, to *Account, now time.Time) error
```

`NewTransfer` checks:
- `id`, `from` and `to` are not empty (`ValidationError`s, joined).
- `from != to`, else `ErrSameAccount`.
- `amount` is positive, else `ErrInvalidAmount`.

It then builds both entries. Invariant: `Entries[0].Amount + Entries[1].Amount == 0`, always.

`ApplyTransfer` updates the two accounts in memory and returns the **first** failing check, in this order:

1. `t.Amount` currency equals `from.Currency` and `to.Currency`, else `ErrCurrencyMismatch`.
2. `from` status: Frozen gives `ErrAccountFrozen`, Closed gives `ErrAccountClosed`.
3. `to` status: same rules (a frozen account can neither send nor receive).
4. If `!from.System` and the new balance would be negative, `ErrInsufficientFunds`.
5. Compute both new balances (an overflow is `ErrOverflow`).

If any check fails, **neither account is modified**. Compute everything first, then assign. On success, `Touch(now)` both accounts.

---

## 6. `generics.go`

```go
type Result[T any] struct {
    Value T
    Err   error
}

type PageRequest struct {
    Limit  int    // 0 means 50; capped at 200; negative is a ValidationError on "limit"
    Cursor string // opaque; empty = first page
}

type Page[T any] struct {
    Items      []T
    NextCursor string // empty = no more pages
}

func Map[T, U any](s []T, f func(T) U) []U
func Filter[T any](s []T, keep func(T) bool) []T
func GroupBy[T any, K comparable](s []T, key func(T) K) map[K][]T

// Real use of the helpers: Filter out system accounts, GroupBy currency,
// then sum balances with Money.Add.
func TotalsByCurrency(accounts []*Account) (map[Currency]Money, error)
```

`Map` and `Filter` on a nil slice return an empty non-nil slice, so JSON encodes it as `[]`, not `null`.

---

## 7. `repository.go`

Every method takes `context.Context` now, even though the memory store doesn't need it. Postgres in Milestone 4 will need it, and the roadmap says this interface **must not change** when that happens.

```go
type Repository interface {
    CreateAccount(ctx context.Context, a *Account) error              // ErrAlreadyExists
    GetAccount(ctx context.Context, id AccountID) (*Account, error)   // ErrNotFound
    ListAccounts(ctx context.Context, req PageRequest) (Page[*Account], error)

    // UpdateAccount loads the account, calls fn on it, and saves it only if fn returns nil.
    UpdateAccount(ctx context.Context, id AccountID, fn func(a *Account) error) error

    // ExecTransfer atomically: rejects a reused t.ID (ErrDuplicateTransfer),
    // loads t.From and t.To (ErrNotFound), calls fn, and if fn returns nil
    // saves both accounts, the transfer, and its two entries. If fn fails, nothing is saved.
    ExecTransfer(ctx context.Context, t *Transfer, fn func(from, to *Account) error) error

    ListEntries(ctx context.Context, id AccountID, req PageRequest) (Page[Entry], error)
}
```

Why the closure-based methods exist: a `Get` then `Save` pair races, because two goroutines read the same balance and both spend it. The repository owns the lock (or, later, the DB transaction). The domain owns the rules and passes them in as `fn`.

---

## 8. `memory.go`

```go
type MemoryRepository struct { /* sync.RWMutex + maps */ }

func NewMemoryRepository() *MemoryRepository
var _ Repository = (*MemoryRepository)(nil) // compile-time interface check
```

Rules:
- Check `ctx.Err()` at the start of every method and return it if set.
- Reads use `RLock`; `CreateAccount`, `UpdateAccount` and `ExecTransfer` use `Lock`.
- **Never hand out or keep internal pointers.** Store copies, return copies, and give `fn` copies, writing them back only on success. A caller mutating a returned `*Account` must not change the store. Write a test that proves it.
- `ListAccounts` is ordered by `ID` ascending. `ListEntries` is ordered by `CreatedAt`, then `TransferID`. Cursor = the sort key of the last item returned.

---

## 9. `ledger.go`

```go
type Ledger struct { /* repo, now func() time.Time */ }

func NewLedger(repo Repository, now func() time.Time) *Ledger

// Bootstrap creates one system account per currency, ID "system:<CUR>", Owner "system", System: true.
// Safe to call twice (ignore ErrAlreadyExists).
func (l *Ledger) Bootstrap(ctx context.Context) error

func (l *Ledger) OpenAccount(ctx context.Context, id AccountID, owner string, c Currency) (*Account, error)
func (l *Ledger) GetAccount(ctx context.Context, id AccountID) (*Account, error)

// Deposit is a Transfer from the currency's system account.
func (l *Ledger) Deposit(ctx context.Context, id TransferID, to AccountID, amount Money) (*Transfer, error)
func (l *Ledger) Transfer(ctx context.Context, id TransferID, from, to AccountID, amount Money) (*Transfer, error)

type TransferRequest struct {
    ID       TransferID
    From, To AccountID
    Amount   Money
}

// TransferBatch runs each request independently; one failure does not stop the others.
// Result i corresponds to reqs[i].
func (l *Ledger) TransferBatch(ctx context.Context, reqs []TransferRequest) []Result[*Transfer]

func (l *Ledger) Freeze(ctx context.Context, id AccountID) error
func (l *Ledger) Unfreeze(ctx context.Context, id AccountID) error
func (l *Ledger) Close(ctx context.Context, id AccountID) error
```

`Transfer` = `NewTransfer`, then `repo.ExecTransfer(ctx, t, func(from, to) error { return ApplyTransfer(t, from, to, now) })`.

System-wide invariant: **the sum of all balances, system accounts included, is always 0 per currency.**

---

## Required tests

| File | Test | What it proves |
| --- | --- | --- |
| `money_test.go` | Table-driven `TestParseMoney` with every row from the table above, as subtests with `t.Parallel()` | Parsing rules |
| | `TestMoneyString` with every row above | Formatting, including `MinInt64` |
| | `TestMoneyAddSub`: normal, currency mismatch, overflow at `MaxInt64` and `MinInt64` | Overflow detection |
| | `FuzzParseMoney`, seeded with all the table inputs | Never panics; round-trip holds |
| | `ExampleMoney_String`, `ExampleParseMoney` with `// Output:` | Docs that are also tests |
| | `BenchmarkMoneyAdd`, `BenchmarkParseMoney` | Baseline for Milestone 9 |
| `account_test.go` | Every allowed and forbidden transition | State machine |
| | `NewAccount` with all three fields invalid returns 3 errors; `errors.As` finds a `*ValidationError` | `errors.Join` |
| `transfer_test.go` | Each `ApplyTransfer` error in order; accounts unchanged after any failure | Atomicity in memory |
| | Entries sum to zero | Double entry |
| `memory_test.go` | Mutating a returned account doesn't change the store; pagination across 3 pages; cancelled context returns `context.Canceled` | Encapsulation |
| `ledger_test.go` | Duplicate transfer ID gives `ErrDuplicateTransfer`; `TransferBatch` with one bad request among good ones | Service behaviour |
| | `BenchmarkLedgerTransfer` | Baseline for Milestone 9 |
| `concurrency_test.go` | **The Verify test** (below) | Correctness under load |

### The Verify test (`TestConcurrentTransfersConserveMoney`)

1. Bootstrap, open 10 INR accounts, and deposit `INR 1000.00` into each.
2. Start 1,000 goroutines. Each picks two distinct random accounts and a random amount from 1 to 50,000 paise, then calls `Transfer` with a unique ID.
3. The only error allowed is `ErrInsufficientFunds`. Anything else fails the test.
4. After all goroutines finish (`sync.WaitGroup`), check that:
   - the sum of the 10 balances is exactly `INR 10000.00`;
   - the sum of all balances including `system:INR` is 0;
   - for each account, the sum of its entries (via `ListEntries`, all pages) equals its balance;
   - no balance is negative.
5. Run it with `go test -race -count=20`.

### Done when

- `go test -race -count=1 ./...` passes.
- `go test -fuzz=FuzzParseMoney -fuzztime=60s ./internal/domain` finds nothing.
- `go test -cover ./internal/domain` reports at least 85%.
- `golangci-lint run` is clean.
- `LEARNINGS.md` has a line for every bug you hit.
