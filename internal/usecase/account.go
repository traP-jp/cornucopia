package usecase

import (
	"context"

	"github.com/google/uuid"
	"github.com/traP-jp/plutus/system/cornucopia/internal/domain"
)

type AccountUseCase struct {
	accountRepo domain.AccountRepository
	tm          domain.TransactionManager
}

func NewAccountUseCase(accountRepo domain.AccountRepository, tm domain.TransactionManager) *AccountUseCase {
	return &AccountUseCase{
		accountRepo: accountRepo,
		tm:          tm,
	}
}

func (u *AccountUseCase) CreateAccount(ctx context.Context, canOverdraft bool) (*domain.Account, error) {
	var acc *domain.Account

	err := u.tm.Run(ctx, func(ctx context.Context) error {
		id, err := uuid.NewV7()
		if err != nil {
			return err
		}
		acc = domain.NewAccount(domain.AccountID(id), canOverdraft)

		return u.accountRepo.SaveAccount(ctx, acc)
	})

	if err != nil {
		return nil, err
	}
	return acc, nil
}

func (u *AccountUseCase) GetAccount(ctx context.Context, id domain.AccountID) (*domain.Account, error) {
	return u.accountRepo.FindAccountByID(ctx, id)
}

// ListAccountsInput represents the input for listing accounts.
type ListAccountsInput struct {
	Filter domain.AccountFilter
	Sort   domain.AccountSort
	Limit  int
	Offset int
}

// ListAccountsOutput represents the output for listing accounts.
type ListAccountsOutput struct {
	Accounts   []*domain.Account
	TotalCount int
}

func (u *AccountUseCase) ListAccounts(ctx context.Context, input ListAccountsInput) (*ListAccountsOutput, error) {
	// Apply defaults and limits
	limit := input.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}

	offset := input.Offset
	if offset < 0 {
		offset = 0
	}

	accounts, totalCount, err := u.accountRepo.ListAccounts(ctx, input.Filter, input.Sort, limit, offset)
	if err != nil {
		return nil, err
	}

	return &ListAccountsOutput{
		Accounts:   accounts,
		TotalCount: totalCount,
	}, nil
}
