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

func (u *AccountUseCase) CreateAccount(ctx context.Context, ownerID domain.OwnerID, canOverdraft bool) (*domain.Account, error) {
	var acc *domain.Account

	err := u.tm.Run(ctx, func(ctx context.Context) error {
		// Check existing inside transaction with lock to prevent race conditions
		existing, err := u.accountRepo.FindByOwnerIDForUpdate(ctx, ownerID)
		if err == nil && existing != nil {
			acc = existing
			return nil
		}

		id, err := uuid.NewV7()
		if err != nil {
			return err
		}
		acc = domain.NewAccount(domain.AccountID(id), ownerID, canOverdraft)

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
