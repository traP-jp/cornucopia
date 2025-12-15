package domain

import "context"

// AccountRepository manages Account persistence.
type AccountRepository interface {
	SaveAccount(ctx context.Context, account *Account) error
	FindAccountByID(ctx context.Context, id AccountID) (*Account, error)
	GetAccountForUpdate(ctx context.Context, id AccountID) (*Account, error)
	FindByOwnerID(ctx context.Context, ownerID OwnerID) (*Account, error)
	// FindByOwnerIDForUpdate locks the row for update to prevent race conditions.
	FindByOwnerIDForUpdate(ctx context.Context, ownerID OwnerID) (*Account, error)
    // UpdateBalance is atomic balance update. In some designs Save handles it.
    // For Transfer service using DB TX, we might rely on generic Transaction handling at Usecase level.
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
