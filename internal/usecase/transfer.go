package usecase

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/traP-jp/plutus/system/cornucopia/internal/domain"
)

const (
	// MaxTransferAmount is the maximum allowed transfer amount (100 billion points)
	MaxTransferAmount int64 = 100_000_000_000
	// MaxDescriptionLength is the maximum allowed description length
	MaxDescriptionLength = 500
)

type TransferUseCase struct {
	accountRepo domain.AccountRepository
	repo        domain.JournalEntryRepository
	tm          domain.TransactionManager
}

func NewTransferUseCase(
	accountRepo domain.AccountRepository,
	repo domain.JournalEntryRepository,
	tm domain.TransactionManager,
) *TransferUseCase {
	return &TransferUseCase{
		accountRepo: accountRepo,
		repo:        repo,
		tm:          tm,
	}
}

type TransferInput struct {
	FromAccountID  string
	ToAccountID    string
	Amount         int64
	Description    string
	IdempotencyKey string
}

type TransferOutput struct {
	JournalEntryID string
	CreatedAt      time.Time
}

func (u *TransferUseCase) Transfer(ctx context.Context, input TransferInput) (*TransferOutput, error) {
	// Input Validation
	if input.Amount <= 0 {
		return nil, domain.ErrInvalidAmount
	}
	if input.Amount > MaxTransferAmount {
		return nil, domain.ErrAmountTooLarge
	}
	if input.FromAccountID == input.ToAccountID {
		return nil, domain.ErrSelfTransfer
	}
	if strings.TrimSpace(input.IdempotencyKey) == "" {
		return nil, domain.ErrInvalidIdempotencyKey
	}
	if len(input.Description) > MaxDescriptionLength {
		return nil, domain.ErrDescriptionTooLong
	}

	// 1. Idempotency Check (Quick check before TX)
	existing, err := u.repo.FindByIdempotencyKey(ctx, input.IdempotencyKey)
	if err != nil {
		return nil, err
	}

	if existing != nil {
		return &TransferOutput{
			JournalEntryID: string(existing.ID),
			CreatedAt:     existing.Timestamp,
		}, nil
	}

	var newEntry *domain.JournalEntry

	// 2. Atomic Transaction with Global Serialization for Hash Chain
	err = u.tm.RunSerialized(ctx, "journal_entry_chain", func(ctx context.Context) error {
		return u.tm.Run(ctx, func(ctx context.Context) error {
			// Re-check idempotency inside TX (double check locking)
			existing, err := u.repo.FindByIdempotencyKey(ctx, input.IdempotencyKey)
			if err == nil && existing != nil {
				newEntry = existing
				return nil
			}

			// Prevent Deadlock: Lock order must be consistent (e.g., lexical order)
			firstID, secondID := input.FromAccountID, input.ToAccountID
			if firstID > secondID {
				firstID, secondID = secondID, firstID
			}

			// Helper to load
			load := func(id string) (*domain.Account, error) {
				acc, err := u.accountRepo.GetAccountForUpdate(ctx, domain.AccountID(id))
				if err != nil {
					return nil, err
				}
				if acc == nil {
					return nil, domain.ErrAccountNotFound
				}
				return acc, nil
			}

			// acquire lock in order
			acc1, err := load(firstID)
			if err != nil {
				return err
			}
			acc2, err := load(secondID)
			if err != nil {
				return err
			}

			// Map back to from/to
			var from, to *domain.Account
			if firstID == input.FromAccountID {
				from = acc1
				to = acc2
			} else {
				from = acc2
				to = acc1
			}

			// Execute Transfer Logic
			if err := from.Withdraw(input.Amount); err != nil {
				return err
			}
			if err := to.Deposit(input.Amount); err != nil {
				return err
			}

			// Create Journal Entry Record
			// Lock latest entry for hash chain
			latestEntry, err := u.repo.GetLatestJournalEntry(ctx)
			prevHash := ""
			if err == nil && latestEntry != nil {
				prevHash = latestEntry.Hash
			}

			newEntry = &domain.JournalEntry{
				ID:             domain.JournalEntryID(uuid.New().String()),
				FromAccountID:  from.ID,
				ToAccountID:    to.ID,
				Amount:         input.Amount,
				Description:    input.Description,
				IdempotencyKey: input.IdempotencyKey,
				PreviousHash:   prevHash,
				Timestamp:      time.Now(),
			}
			// Compute Hash
			newEntry.Hash = newEntry.ComputeHash()

			// Save All
			if err := u.accountRepo.SaveAccount(ctx, from); err != nil {
				return err
			}
			if err := u.accountRepo.SaveAccount(ctx, to); err != nil {
				return err
			}
			if err := u.repo.SaveJournalEntry(ctx, newEntry); err != nil {
				return err
			}

			return nil
		})
	})

	if err != nil {
		return nil, err
	}

	return &TransferOutput{
		JournalEntryID: string(newEntry.ID),
		CreatedAt:      newEntry.Timestamp,
	}, nil
}

func (u *TransferUseCase) GetJournalEntries(ctx context.Context, accountID string, limit, offset int) ([]*domain.JournalEntry, error) {
	// Validate and normalize limit/offset
	if limit <= 0 {
		limit = 50
	}
	if limit > 1000 {
		limit = 1000
	}
	if offset < 0 {
		offset = 0
	}
	
	return u.repo.FindByAccountID(ctx, domain.AccountID(accountID), limit, offset)
}
