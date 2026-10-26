// Package domain holds Ledgerly's money rules: amounts, currencies, accounts,
// and the double-entry ledger.
//
// It must stay free of I/O (no database, network, files, or clock reads
// beyond what callers pass in) so every later milestone can build on it
// without changing it.
package domain
