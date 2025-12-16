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
	byOwner  map[domain.OwnerID]*domain.Account
	err      error
}

func newMockAccountRepo() *mockAccountRepo {
	return &mockAccountRepo{
		accounts: make(map[domain.AccountID]*domain.Account),
		byOwner:  make(map[domain.OwnerID]*domain.Account),
	}
}

func (m *mockAccountRepo) SaveAccount(ctx context.Context, account *domain.Account) error {
	if m.err != nil {
		return m.err
	}
	m.accounts[account.ID] = account
	m.byOwner[account.OwnerID] = account
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

func (m *mockAccountRepo) FindByOwnerID(ctx context.Context, ownerID domain.OwnerID) (*domain.Account, error) {
	if m.err != nil {
		return nil, m.err
	}
	if acc, ok := m.byOwner[ownerID]; ok {
		return acc, nil
	}
	return nil, nil
}

func (m *mockAccountRepo) GetAccountForUpdate(ctx context.Context, id domain.AccountID) (*domain.Account, error) {
	// For mock, same as FindAccountByID
	return m.FindAccountByID(ctx, id)
}

func (m *mockAccountRepo) FindByOwnerIDForUpdate(ctx context.Context, ownerID domain.OwnerID) (*domain.Account, error) {
	// For mock, same as FindByOwnerID
	return m.FindByOwnerID(ctx, ownerID)
}

// mockTxManager is defined in transfer_test.go

func TestAccountUseCase_CreateAccount(t *testing.T) {
	repo := newMockAccountRepo()
	tm := &mockTxManager{}
	uc := NewAccountUseCase(repo, tm)
	ctx := context.Background()
	ownerID := domain.OwnerID(mustUUID("user-123"))

	// 1. Create new account
	acc, err := uc.CreateAccount(ctx, ownerID, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if acc == nil {
		t.Fatal("expected account, got nil")
	}
	if acc.OwnerID != ownerID {
		t.Errorf("expected ownerID %s, got %s", ownerID, acc.OwnerID)
	}
	if acc.Balance != 0 {
		t.Errorf("expected balance 0, got %d", acc.Balance)
	}

	// 2. Create existing account (should return same)
	acc2, err := uc.CreateAccount(ctx, ownerID, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if acc.ID != acc2.ID {
		t.Errorf("expected same account ID %s, got %s", acc.ID, acc2.ID)
	}
}

func TestAccountUseCase_GetAccount(t *testing.T) {
	repo := newMockAccountRepo()
	tm := &mockTxManager{}
	uc := NewAccountUseCase(repo, tm)
	ctx := context.Background()

	// Setup: create an account directly in repo
	testID := domain.AccountID(mustUUID("acc-test"))
	existing := domain.NewAccount(testID, domain.OwnerID(mustUUID("owner-test")), false)
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
	_, err := uc.CreateAccount(ctx, domain.OwnerID(mustUUID("user-err")), false)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
