package usecase

import (
	"context"
	"testing"

	"github.com/google/uuid"
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
	m.txs[tx.ID.String()] = tx
	m.idempotency[tx.IdempotencyKey] = tx
	m.lastTx = tx
	return nil
}

func (m *mockJournalEntryRepo) FindJournalEntryByID(ctx context.Context, id domain.JournalEntryID) (*domain.JournalEntry, error) {
	if tx, ok := m.txs[id.String()]; ok {
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
	fromID := domain.AccountID(mustUUID("acc-from"))
	toID := domain.AccountID(mustUUID("acc-to"))

	fromAcc := domain.NewAccount(fromID, domain.OwnerID(mustUUID("owner-1")), false)
	fromAcc.Balance = 1000
	accRepo.SaveAccount(ctx, fromAcc)

	toAcc := domain.NewAccount(toID, domain.OwnerID(mustUUID("owner-2")), false)
	accRepo.SaveAccount(ctx, toAcc)

	// 1. Success Transfer
	input := TransferInput{
		FromAccountID:  fromID,
		ToAccountID:    toID,
		Amount:         500,
		Description:    "test transfer",
		IdempotencyKey: "key-1",
	}

	out, err := uc.Transfer(ctx, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.JournalEntryID == domain.JournalEntryID(uuid.Nil) {
		t.Error("expected transaction ID")
	}

	// Verify balances
	if fromAcc.Balance != 500 {
		t.Errorf("from balance expected 500, got %d", fromAcc.Balance)
	}

	toCheck, _ := accRepo.FindAccountByID(ctx, toID)
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
		FromAccountID:  fromID,
		ToAccountID:    toID,
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
	accA := domain.AccountID(mustUUID("acc-A"))
	accB := domain.AccountID(mustUUID("acc-B"))
	accC := domain.AccountID(mustUUID("acc-C"))

	entry1 := &domain.JournalEntry{
		ID:            domain.JournalEntryID(mustUUID("tx-1")),
		FromAccountID: accA,
		ToAccountID:   accB,
		Amount:        100,
	}
	txRepo.SaveJournalEntry(ctx, entry1)

	entry2 := &domain.JournalEntry{
		ID:            domain.JournalEntryID(mustUUID("tx-2")),
		FromAccountID: accB,
		ToAccountID:   accC,
		Amount:        200,
	}
	txRepo.SaveJournalEntry(ctx, entry2)

	// Test: Get entries for acc-B
	entries, err := uc.GetJournalEntries(ctx, accB, 10, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 entries for acc-B, got %d", len(entries))
	}

	// Test: Get entries for acc-A
	entriesA, err := uc.GetJournalEntries(ctx, accA, 10, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entriesA) != 1 {
		t.Errorf("expected 1 entry for acc-A, got %d", len(entriesA))
	}
}
