package domain

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

func Map[T, U any](s []T, f func(T) U) []U {
	retArr := make([]U, len(s))
	for i, item := range s {
		retArr[i] = f(item)
	}

	return retArr
}
func Filter[T any](s []T, keep func(T) bool) []T {
	retArr := make([]T, 0, len(s))
	for _, item := range s {
		if keep(item) {
			retArr = append(retArr, item)
		}
	}

	return retArr
}

func GroupBy[T any, K comparable](s []T, key func(T) K) map[K][]T {
	result := make(map[K][]T)
	for _, item := range s {
		k := key(item)
		result[k] = append(result[k], item)
	}
	return result
}

// Real use of the helpers: Filter out system accounts, GroupBy currency,
// then sum balances with Money.Add.
func TotalsByCurrency(accounts []*Account) (map[Currency]Money, error) {
	filteredAccounts := Filter(accounts, func(account *Account) bool {
		return account.System != true
	})

	groupedAccounts := GroupBy(filteredAccounts, func(account *Account) Currency {
		return account.Currency
	})

	// Initialize the totals map
	totals := make(map[Currency]Money)

	// Iterate through each currency group
	for currency, accList := range groupedAccounts {
		// Start with a zero/empty money value for this currency group
		var currentTotal Money
		var err error

		for _, account := range accList {
			// Assuming Add returns the new Money total and an error (e.g., overflow)
			currentTotal, err = currentTotal.Add(account.Balance)
			if err != nil {
				return nil, err
			}
		}

		totals[currency] = currentTotal
	}

	return totals, nil
}
