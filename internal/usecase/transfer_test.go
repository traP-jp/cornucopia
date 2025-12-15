package usecase

import (
	"context"
	"testing"

	"github.com/traP-jp/plutus/system/cornucopia/internal/domain"
)

// mockJournalEntryRepo implements domain.JournalEntryRepository
type mockJournalEntryRepo struct {
	txs         map[string]*domain.JournalEntry
	idempotency map[string]*domain.JournalEntry
	lastTx      *domain.JournalEntry
}

func newMockJournalEntryRepo() *mockJournalEntryRepo {
	return &mockJournalEntryRepo{
		txs:         make(map[string]*domain.JournalEntry),
		idempotency: make(map[string]*domain.JournalEntry),
	}
}

func (m *mockJournalEntryRepo) SaveJournalEntry(ctx context.Context, tx *domain.JournalEntry) error {
	m.txs[string(tx.ID)] = tx
	m.idempotency[tx.IdempotencyKey] = tx
	m.lastTx = tx
	return nil
}

func (m *mockJournalEntryRepo) FindJournalEntryByID(ctx context.Context, id domain.JournalEntryID) (*domain.JournalEntry, error) {
	if tx, ok := m.txs[string(id)]; ok {
		return tx, nil
	}
	return nil, nil
}

func (m *mockJournalEntryRepo) FindByIdempotencyKey(ctx context.Context, key string) (*domain.JournalEntry, error) {
	if tx, ok := m.idempotency[key]; ok {
		return tx, nil
	}
	return nil, nil
}

func (m *mockJournalEntryRepo) GetLatestJournalEntry(ctx context.Context) (*domain.JournalEntry, error) {
	return m.lastTx, nil
}

func (m *mockJournalEntryRepo) FindByAccountID(ctx context.Context, accountID domain.AccountID, limit, offset int) ([]*domain.JournalEntry, error) {
	var result []*domain.JournalEntry
	// Simple scan for mock
	for _, tx := range m.txs {
		if tx.FromAccountID == accountID || tx.ToAccountID == accountID {
			result = append(result, tx)
		}
	}
	// In real DB, we'd sort and limit/offset. For mock test, we might just return all or simple slice.
	// Let's implement basics.
	if offset >= len(result) {
		return []*domain.JournalEntry{}, nil
	}
	end := offset + limit
	if end > len(result) {
		end = len(result)
	}
	return result[offset:end], nil
}

// mockTxManager implements domain.TransactionManagerStub
type mockTxManager struct{}

func (m *mockTxManager) Run(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

func (m *mockTxManager) RunSerialized(ctx context.Context, name string, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

func TestTransferUseCase_Transfer(t *testing.T) {
	accRepo := newMockAccountRepo() // Defined in account_test.go
	txRepo := newMockJournalEntryRepo()
	tm := &mockTxManager{}
	uc := NewTransferUseCase(accRepo, txRepo, tm)
	ctx := context.Background()

	// Setup accounts
	fromAcc := domain.NewAccount("acc-from", "owner-1", false)
	fromAcc.Balance = 1000
	accRepo.SaveAccount(ctx, fromAcc)

	toAcc := domain.NewAccount("acc-to", "owner-2", false)
	accRepo.SaveAccount(ctx, toAcc)

	// 1. Success Transfer
	input := TransferInput{
		FromAccountID:  "acc-from",
		ToAccountID:    "acc-to",
		Amount:         500,
		Description:    "test transfer",
		IdempotencyKey: "key-1",
	}
	// Warning: TransferInput definition in file didn't have Timestamp in previous view?
	// Let's check:
	// type TransferInput struct {
	// 	FromAccountID  string
	// 	ToAccountID    string
	// 	Amount         int64
	// 	Description    string
	// 	IdempotencyKey string
	// }
	// Correct.

	out, err := uc.Transfer(ctx, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.JournalEntryID == "" {
		t.Error("expected transaction ID")
	}

	// Verify balances
	if fromAcc.Balance != 500 {
		t.Errorf("from balance expected 500, got %d", fromAcc.Balance)
	}
	// Note: In a real scenario, repo.SaveAccount would update the stored object.
	// Since we are passing pointers to mock repo and modifying them in place (Account is pointer), 
	// and repo.SaveAccount in mock just stores the pointer, the modification in Transfer (loading pointer) works.
	// Wait, Transfer calls FindAccountByID which returns the pointer stored in mock.
	// Then calls Withdraw on that pointer.
	// So checking fromAcc.Balance works because it points to the same object.
	
	toCheck, _ := accRepo.FindAccountByID(ctx, "acc-to")
	if toCheck.Balance != 500 {
		t.Errorf("to balance expected 500, got %d", toCheck.Balance)
	}

	// 2. Idempotency Check
	out2, err := uc.Transfer(ctx, input)
	if err != nil {
		t.Fatalf("unexpected error on retry: %v", err)
	}
	if out.JournalEntryID != out2.JournalEntryID {
		t.Errorf("expected same transaction ID %s, got %s", out.JournalEntryID, out2.JournalEntryID)
	}
	
	// Balance verification again
	if fromAcc.Balance != 500 {
		t.Errorf("balance changed on idempotent call: %d", fromAcc.Balance)
	}

	// 3. Insufficient Balance
	inputFail := TransferInput{
		FromAccountID:  "acc-from",
		ToAccountID:    "acc-to",
		Amount:         10000,
		Description:    "fail transfer",
		IdempotencyKey: "key-2",
	}
	_, err = uc.Transfer(ctx, inputFail)
	if err != domain.ErrInsufficientBalance {
		t.Errorf("expected ErrInsufficientBalance, got %v", err)
	}
}

func TestTransferUseCase_GetJournalEntries(t *testing.T) {
	accRepo := newMockAccountRepo()
	txRepo := newMockJournalEntryRepo()
	tm := &mockTxManager{}
	uc := NewTransferUseCase(accRepo, txRepo, tm)
	ctx := context.Background()

	// Pre-populate some entries
	entry1 := &domain.JournalEntry{
		ID:            "tx-1",
		FromAccountID: "acc-A",
		ToAccountID:   "acc-B",
		Amount:        100,
	}
	txRepo.SaveJournalEntry(ctx, entry1)

	entry2 := &domain.JournalEntry{
		ID:            "tx-2",
		FromAccountID: "acc-B",
		ToAccountID:   "acc-C",
		Amount:        200,
	}
	txRepo.SaveJournalEntry(ctx, entry2)

	// Test: Get entries for acc-B
	entries, err := uc.GetJournalEntries(ctx, "acc-B", 10, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 entries for acc-B, got %d", len(entries))
	}

	// Test: Get entries for acc-A
	entriesA, err := uc.GetJournalEntries(ctx, "acc-A", 10, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entriesA) != 1 {
		t.Errorf("expected 1 entry for acc-A, got %d", len(entriesA))
	}
}
