package domain

import (
	"testing"
)

func TestNewAccount(t *testing.T) {
	acc := NewAccount("acc-1", "owner-1", false)
	if acc.ID != "acc-1" {
		t.Errorf("expected ID acc-1, got %s", acc.ID)
	}
	if acc.OwnerID != "owner-1" {
		t.Errorf("expected OwnerID owner-1, got %s", acc.OwnerID)
	}
	if acc.Balance != 0 {
		t.Errorf("expected Balance 0, got %d", acc.Balance)
	}
}

func TestAccount_Deposit(t *testing.T) {
	acc := NewAccount("acc-1", "owner-1", false)
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
	acc := NewAccount("acc-1", "owner-1", false)
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
	acc := NewAccount("acc-od", "owner-od", true)
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
