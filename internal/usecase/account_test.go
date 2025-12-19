package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/traP-jp/plutus/system/cornucopia/internal/domain"
)

func mustUUID(s string) uuid.UUID {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte(s))
}

type mockAccountRepo struct {
	accounts map[domain.AccountID]*domain.Account
	err      error
}

func newMockAccountRepo() *mockAccountRepo {
	return &mockAccountRepo{
		accounts: make(map[domain.AccountID]*domain.Account),
	}
}

func (m *mockAccountRepo) SaveAccount(ctx context.Context, account *domain.Account) error {
	if m.err != nil {
		return m.err
	}
	m.accounts[account.ID] = account
	return nil
}

func (m *mockAccountRepo) FindAccountByID(ctx context.Context, id domain.AccountID) (*domain.Account, error) {
	if m.err != nil {
		return nil, m.err
	}
	if acc, ok := m.accounts[id]; ok {
		return acc, nil
	}
	return nil, nil // Not found as nil, nil (or error depending on convention, but code checks nil)
}

func (m *mockAccountRepo) GetAccountForUpdate(ctx context.Context, id domain.AccountID) (*domain.Account, error) {
	// For mock, same as FindAccountByID
	return m.FindAccountByID(ctx, id)
}

func (m *mockAccountRepo) ListAccounts(ctx context.Context, filter domain.AccountFilter, sort domain.AccountSort, limit, offset int) ([]*domain.Account, int, error) {
	if m.err != nil {
		return nil, 0, m.err
	}
	// Simple filtering for mock
	var result []*domain.Account
	for _, acc := range m.accounts {
		if filter.MinBalance != nil && acc.Balance < *filter.MinBalance {
			continue
		}
		if filter.MaxBalance != nil && acc.Balance > *filter.MaxBalance {
			continue
		}
		if filter.CanOverdraft != nil && acc.CanOverdraft != *filter.CanOverdraft {
			continue
		}
		result = append(result, acc)
	}
	totalCount := len(result)
	// Apply offset/limit
	if offset > len(result) {
		result = nil
	} else {
		result = result[offset:]
		if limit < len(result) {
			result = result[:limit]
		}
	}
	return result, totalCount, nil
}

func (m *mockAccountRepo) FindAccountsByIDs(ctx context.Context, ids []domain.AccountID) ([]*domain.Account, error) {
	if m.err != nil {
		return nil, m.err
	}
	var result []*domain.Account
	for _, id := range ids {
		if acc, ok := m.accounts[id]; ok {
			result = append(result, acc)
		}
	}
	return result, nil
}

// mockTxManager is defined in transfer_test.go

func TestAccountUseCase_CreateAccount(t *testing.T) {
	repo := newMockAccountRepo()
	tm := &mockTxManager{}
	uc := NewAccountUseCase(repo, tm)
	ctx := context.Background()

	// 1. Create new account
	acc, err := uc.CreateAccount(ctx, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if acc == nil {
		t.Fatal("expected account, got nil")
	}
	if acc.Balance != 0 {
		t.Errorf("expected balance 0, got %d", acc.Balance)
	}

	// 2. Create another account
	acc2, err := uc.CreateAccount(ctx, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if acc.ID == acc2.ID {
		t.Errorf("expected different account IDs")
	}
	if !acc2.CanOverdraft {
		t.Errorf("expected can_overdraft true")
	}
}

func TestAccountUseCase_GetAccount(t *testing.T) {
	repo := newMockAccountRepo()
	tm := &mockTxManager{}
	uc := NewAccountUseCase(repo, tm)
	ctx := context.Background()

	// Setup: create an account directly in repo
	testID := domain.AccountID(mustUUID("acc-test"))
	existing := domain.NewAccount(testID, false)
	repo.SaveAccount(ctx, existing)

	// Test GetAccount
	found, err := uc.GetAccount(ctx, testID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found == nil {
		t.Fatal("expected account, got nil")
	}
	if found.ID != existing.ID {
		t.Errorf("expected ID %s, got %s", existing.ID, found.ID)
	}

	// Test GetAccount non-existent
	notFound, err := uc.GetAccount(ctx, domain.AccountID(mustUUID("acc-unknown")))
	if err != nil {
		// mocked repo returns nil, nil for not found
	}
	if notFound != nil {
		t.Errorf("expected nil for unknown account, got %v", notFound)
	}
}

func TestAccountUseCase_RepoError(t *testing.T) {
	repo := newMockAccountRepo()
	repo.err = errors.New("db error")
	tm := &mockTxManager{}
	uc := NewAccountUseCase(repo, tm)
	ctx := context.Background()

	// CreateAccount should fail if SaveAccount fails (assuming Find failed or passed)
	// In current impl, if Find fails, it tries Save. If Save fails, returns error.
	_, err := uc.CreateAccount(ctx, false)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestAccountUseCase_ListAccounts(t *testing.T) {
	repo := newMockAccountRepo()
	tm := &mockTxManager{}
	uc := NewAccountUseCase(repo, tm)
	ctx := context.Background()

	// Setup test accounts
	acc1 := &domain.Account{
		ID:           domain.AccountID(mustUUID("acc-1")),
		Balance:      100,
		CanOverdraft: false,
	}
	acc2 := &domain.Account{
		ID:           domain.AccountID(mustUUID("acc-2")),
		Balance:      500,
		CanOverdraft: true,
	}
	acc3 := &domain.Account{
		ID:           domain.AccountID(mustUUID("acc-3")),
		Balance:      50,
		CanOverdraft: false,
	}
	repo.SaveAccount(ctx, acc1)
	repo.SaveAccount(ctx, acc2)
	repo.SaveAccount(ctx, acc3)

	// Test 1: List all accounts
	out, err := uc.ListAccounts(ctx, ListAccountsInput{
		Limit: 100,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.TotalCount != 3 {
		t.Errorf("expected 3 accounts, got %d", out.TotalCount)
	}

	// Test 2: Filter by min balance
	minBal := int64(100)
	out, err = uc.ListAccounts(ctx, ListAccountsInput{
		Filter: domain.AccountFilter{MinBalance: &minBal},
		Limit:  100,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.TotalCount != 2 {
		t.Errorf("expected 2 accounts with balance >= 100, got %d", out.TotalCount)
	}

	// Test 3: Filter by can_overdraft
	canOverdraft := true
	out, err = uc.ListAccounts(ctx, ListAccountsInput{
		Filter: domain.AccountFilter{CanOverdraft: &canOverdraft},
		Limit:  100,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.TotalCount != 1 {
		t.Errorf("expected 1 account with overdraft, got %d", out.TotalCount)
	}

	// Test 4: Default limit applied (negative limit -> 100)
	out, err = uc.ListAccounts(ctx, ListAccountsInput{
		Limit: -1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.TotalCount != 3 {
		t.Errorf("expected 3 accounts, got %d", out.TotalCount)
	}

	// Test 5: Limit cap at 1000
	out, err = uc.ListAccounts(ctx, ListAccountsInput{
		Limit: 5000,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should still return all (cap at 1000 but only 3 exist)
	if out.TotalCount != 3 {
		t.Errorf("expected 3 accounts, got %d", out.TotalCount)
	}
}
