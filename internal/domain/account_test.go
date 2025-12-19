package domain

import (
	"testing"

	"github.com/google/uuid"
)

func mustUUID(s string) uuid.UUID {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte(s))
}

func TestNewAccount(t *testing.T) {
	id := AccountID(mustUUID("acc-1"))
	acc := NewAccount(id, false)
	if acc.ID != id {
		t.Errorf("expected ID %s, got %s", id, acc.ID)
	}
	if acc.Balance != 0 {
		t.Errorf("expected Balance 0, got %d", acc.Balance)
	}
}

func TestAccount_Deposit(t *testing.T) {
	id := AccountID(mustUUID("acc-1"))
	acc := NewAccount(id, false)

	acc.Deposit(100)
	if acc.Balance != 100 {
		t.Errorf("expected Balance 100, got %d", acc.Balance)
	}
	acc.Deposit(50)
	if acc.Balance != 150 {
		t.Errorf("expected Balance 150, got %d", acc.Balance)
	}
}

func TestAccount_Withdraw(t *testing.T) {
	id := AccountID(mustUUID("acc-1"))
	acc := NewAccount(id, false)

	acc.Deposit(100)

	// Successful withdraw
	err := acc.Withdraw(40)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if acc.Balance != 60 {
		t.Errorf("expected Balance 60, got %d", acc.Balance)
	}

	// Insufficient balance
	err = acc.Withdraw(100)
	if err != ErrInsufficientBalance {
		t.Errorf("expected ErrInsufficientBalance, got %v", err)
	}
	if acc.Balance != 60 {
		t.Errorf("balance should not change on error, got %d", acc.Balance)
	}
}

func TestAccount_Withdraw_Overdraft(t *testing.T) {
	id := AccountID(mustUUID("acc-od"))
	acc := NewAccount(id, true)

	acc.Deposit(50)

	// Withdraw more than balance
	err := acc.Withdraw(100)
	if err != nil {
		t.Errorf("unexpected error on overdraft: %v", err)
	}
	if acc.Balance != -50 {
		t.Errorf("expected Balance -50, got %d", acc.Balance)
	}
}
