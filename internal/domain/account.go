package domain

import (
	"math"

	"github.com/google/uuid"
)

// AccountID identifies an account.
type AccountID uuid.UUID

// String returns the string representation of AccountID.
func (id AccountID) String() string {
	return uuid.UUID(id).String()
}

// OwnerID identifies the owner (user/project) from Pteron.
type OwnerID uuid.UUID

// String returns the string representation of OwnerID.
func (id OwnerID) String() string {
	return uuid.UUID(id).String()
}

// Account represents a points account.
type Account struct {
	ID           AccountID
	OwnerID      OwnerID
	Balance      int64
	CanOverdraft bool
}

// NewAccount creates a new account with 0 balance.
func NewAccount(id AccountID, ownerID OwnerID, canOverdraft bool) *Account {
	return &Account{
		ID:           id,
		OwnerID:      ownerID,
		Balance:      0,
		CanOverdraft: canOverdraft,
	}
}

// Deposit adds amount to the balance with overflow protection.
func (a *Account) Deposit(amount int64) error {
	if amount <= 0 {
		return ErrInvalidAmount
	}
	// Check for overflow before addition
	if a.Balance > math.MaxInt64-amount {
		return ErrBalanceOverflow
	}
	a.Balance += amount
	return nil
}

// Withdraw subtracts amount from balance. Returns error if insufficient and overdraft not allowed.
func (a *Account) Withdraw(amount int64) error {
	if amount <= 0 {
		return ErrInvalidAmount
	}
	if !a.CanOverdraft && a.Balance < amount {
		return ErrInsufficientBalance
	}
	a.Balance -= amount
	return nil
}
