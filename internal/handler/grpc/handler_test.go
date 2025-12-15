package grpc

import (
	"context"
	"testing"

	pb "github.com/traP-jp/plutus/api/protobuf"
	"github.com/traP-jp/plutus/system/cornucopia/internal/domain"
	"github.com/traP-jp/plutus/system/cornucopia/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// -- Mocks --

type mockAccountRepo struct {
	accounts map[string]*domain.Account
}

func (m *mockAccountRepo) SaveAccount(ctx context.Context, account *domain.Account) error {
	m.accounts[string(account.ID)] = account
	m.accounts[string(account.OwnerID)] = account // Simple indexing
	return nil
}

func (m *mockAccountRepo) FindAccountByID(ctx context.Context, id domain.AccountID) (*domain.Account, error) {
	if a, ok := m.accounts[string(id)]; ok {
		return a, nil
	}
	return nil, nil
}


func (m *mockAccountRepo) GetAccountForUpdate(ctx context.Context, id domain.AccountID) (*domain.Account, error) {
	return m.FindAccountByID(ctx, id)
}

func (m *mockAccountRepo) FindByOwnerID(ctx context.Context, ownerID domain.OwnerID) (*domain.Account, error) {
	if a, ok := m.accounts[string(ownerID)]; ok {
		return a, nil
	}
	return nil, nil
}

func (m *mockAccountRepo) FindByOwnerIDForUpdate(ctx context.Context, ownerID domain.OwnerID) (*domain.Account, error) {
	return m.FindByOwnerID(ctx, ownerID)
}

type mockJournalEntryRepo struct {
    entries []*domain.JournalEntry
}

func (m *mockJournalEntryRepo) SaveJournalEntry(ctx context.Context, tx *domain.JournalEntry) error { 
    m.entries = append(m.entries, tx)
    return nil 
}
func (m *mockJournalEntryRepo) FindJournalEntryByID(ctx context.Context, id domain.JournalEntryID) (*domain.JournalEntry, error) {
	return nil, nil
}
func (m *mockJournalEntryRepo) FindByIdempotencyKey(ctx context.Context, key string) (*domain.JournalEntry, error) {
	return nil, nil
}
func (m *mockJournalEntryRepo) GetLatestJournalEntry(ctx context.Context) (*domain.JournalEntry, error) {
	return nil, nil
}

func (m *mockJournalEntryRepo) FindByAccountID(ctx context.Context, accountID domain.AccountID, limit, offset int) ([]*domain.JournalEntry, error) {
	var res []*domain.JournalEntry
    for _, e := range m.entries {
        if e.FromAccountID == accountID || e.ToAccountID == accountID {
            res = append(res, e)
        }
    }
    return res, nil
}

type mockTxManager struct{}

func (m *mockTxManager) Run(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

func (m *mockTxManager) RunSerialized(ctx context.Context, name string, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

// -- Tests --

func TestCornucopiaHandler_CreateAccount(t *testing.T) {
	repo := &mockAccountRepo{accounts: make(map[string]*domain.Account)}
	tm := &mockTxManager{}
	uc := usecase.NewAccountUseCase(repo, tm)
	h := NewCornucopiaHandler(nil, uc)

	req := &pb.CreateAccountRequest{OwnerId: "user-1"}
	resp, err := h.CreateAccount(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.OwnerId != "user-1" {
		t.Errorf("expected owner user-1, got %s", resp.OwnerId)
    }
	if resp.AccountId == "" {
		t.Error("expected account id")
	}
}

func TestCornucopiaHandler_Transfer(t *testing.T) {
	accRepo := &mockAccountRepo{accounts: make(map[string]*domain.Account)}
	txRepo := &mockJournalEntryRepo{}
	tm := &mockTxManager{}

	// Wire up
	transferUC := usecase.NewTransferUseCase(accRepo, txRepo, tm)
	h := NewCornucopiaHandler(transferUC, nil)

	// Setup accounts
    // Note: Use pointers to ensure updates are reflected if checking repo later, though here we just check response.
	accRepo.SaveAccount(context.Background(), &domain.Account{ID: "acc-1", Balance: 100})
	accRepo.SaveAccount(context.Background(), &domain.Account{ID: "acc-2", Balance: 0})

	req := &pb.TransferRequest{
		FromAccountId:  "acc-1",
		ToAccountId:    "acc-2",
		Amount:         50,
		Description:    "grpc test",
		IdempotencyKey: "idem-1",
	}

	resp, err := h.Transfer(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.JournalEntryId == "" {
		t.Error("expected journal entry id")
	}
}

func TestCornucopiaHandler_Transfer_InsufficientBalance(t *testing.T) {
	accRepo := &mockAccountRepo{accounts: make(map[string]*domain.Account)}
	txRepo := &mockJournalEntryRepo{}
	tm := &mockTxManager{}

	uc := usecase.NewTransferUseCase(accRepo, txRepo, tm)
	h := NewCornucopiaHandler(uc, nil)

	// acc-1 has 0 balance, transfer 100 -> error
	accRepo.SaveAccount(context.Background(), &domain.Account{ID: "acc-1", Balance: 0, CanOverdraft: false})
	accRepo.SaveAccount(context.Background(), &domain.Account{ID: "acc-2", Balance: 0})

	req := &pb.TransferRequest{
		FromAccountId:  "acc-1",
		ToAccountId:    "acc-2",
		Amount:         100,
		IdempotencyKey: "idem-fail",
	}

	_, err := h.Transfer(context.Background(), req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	
	st, ok := status.FromError(err)
	if !ok {
		t.Fatal("expected gRPC status error")
	}
	if st.Code() != codes.FailedPrecondition {
		t.Errorf("expected code FailedPrecondition, got %v", st.Code())
	}
}

func TestCornucopiaHandler_GetJournalEntries(t *testing.T) {
	accRepo := &mockAccountRepo{accounts: make(map[string]*domain.Account)}
	txRepo := &mockJournalEntryRepo{}
	tm := &mockTxManager{}

	uc := usecase.NewTransferUseCase(accRepo, txRepo, tm)
	h := NewCornucopiaHandler(uc, nil)

	// Seed some entries
    txRepo.entries = append(txRepo.entries, &domain.JournalEntry{ID: "tx-1", FromAccountID: "acc-A", Amount: 100})
    txRepo.entries = append(txRepo.entries, &domain.JournalEntry{ID: "tx-2", FromAccountID: "acc-B", ToAccountID: "acc-A", Amount: 200})

	req := &pb.GetJournalEntriesRequest{
		AccountId: "acc-A",
		Limit:     10,
		Offset:    0,
	}

	resp, err := h.GetJournalEntries(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.JournalEntries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(resp.JournalEntries))
	}
}
