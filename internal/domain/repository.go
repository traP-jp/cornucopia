package domain

import "context"

// SortField represents the field to sort by.
type SortField string

const (
	SortByBalance   SortField = "balance"
	SortByAccountID SortField = "account_id"
)

// SortOrder represents the sort direction.
type SortOrder string

const (
	SortAsc  SortOrder = "asc"
	SortDesc SortOrder = "desc"
)

// AccountFilter represents query filters for listing accounts.
type AccountFilter struct {
	MinBalance   *int64
	MaxBalance   *int64
	CanOverdraft *bool
}

// AccountSort represents sorting options for listing accounts.
type AccountSort struct {
	Field SortField
	Order SortOrder
}

// AccountRepository manages Account persistence.
type AccountRepository interface {
	SaveAccount(ctx context.Context, account *Account) error
	FindAccountByID(ctx context.Context, id AccountID) (*Account, error)
	GetAccountForUpdate(ctx context.Context, id AccountID) (*Account, error)
	// ListAccounts returns accounts matching the filter with pagination and sorting.
	// Returns (accounts, total_count, error).
	ListAccounts(ctx context.Context, filter AccountFilter, sort AccountSort, limit, offset int) ([]*Account, int, error)
}

// JournalEntryRepository manages JournalEntry persistence.
type JournalEntryRepository interface {
	SaveJournalEntry(ctx context.Context, tx *JournalEntry) error
	FindJournalEntryByID(ctx context.Context, id JournalEntryID) (*JournalEntry, error)
	FindByIdempotencyKey(ctx context.Context, key string) (*JournalEntry, error)

	// GetLatestJournalEntry returns the most recent entry to link the hash chain.
	// This usually involves a lock or specialized query.
	GetLatestJournalEntry(ctx context.Context) (*JournalEntry, error)

	FindByAccountID(ctx context.Context, accountID AccountID, limit, offset int) ([]*JournalEntry, error)
}

// TransactionManager handles database transactions.
type TransactionManager interface {
	// Run executes the given function within a transaction.
	Run(ctx context.Context, fn func(ctx context.Context) error) error

	// RunSerialized executes fn while holding a named lock.
	// This ensures that the entire execution of fn (and any transaction within it) is serialized against the same lock name.
	RunSerialized(ctx context.Context, name string, fn func(ctx context.Context) error) error
}
