package grpc

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/traP-jp/plutus/api/protobuf"
	"github.com/traP-jp/plutus/system/cornucopia/internal/domain"
	"github.com/traP-jp/plutus/system/cornucopia/internal/usecase"
)

type CornucopiaHandler struct {
	pb.UnimplementedCornucopiaServiceServer
	transferUC *usecase.TransferUseCase
	accountUC  *usecase.AccountUseCase
}

func NewCornucopiaHandler(
	transferUC *usecase.TransferUseCase,
	accountUC *usecase.AccountUseCase,
) *CornucopiaHandler {
	return &CornucopiaHandler{
		transferUC: transferUC,
		accountUC:  accountUC,
	}
}

func parseAccountID(s string) (domain.AccountID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return domain.AccountID{}, err
	}
	return domain.AccountID(id), nil
}

func (h *CornucopiaHandler) CreateAccount(ctx context.Context, req *pb.CreateAccountRequest) (*pb.CreateAccountResponse, error) {
	acc, err := h.accountUC.CreateAccount(ctx, req.CanOverdraft)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pb.CreateAccountResponse{
		AccountId:    acc.ID.String(),
		Balance:      acc.Balance,
		CanOverdraft: acc.CanOverdraft,
	}, nil
}

func (h *CornucopiaHandler) GetAccount(ctx context.Context, req *pb.GetAccountRequest) (*pb.GetAccountResponse, error) {
	id, err := parseAccountID(req.AccountId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid account_id")
	}

	acc, err := h.accountUC.GetAccount(ctx, id)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if acc == nil {
		return nil, status.Error(codes.NotFound, "account not found")
	}
	return &pb.GetAccountResponse{
		AccountId:    acc.ID.String(),
		Balance:      acc.Balance,
		CanOverdraft: acc.CanOverdraft,
	}, nil
}

func (h *CornucopiaHandler) Transfer(ctx context.Context, req *pb.TransferRequest) (*pb.TransferResponse, error) {
	fromID, err := parseAccountID(req.FromAccountId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid from_account_id")
	}
	toID, err := parseAccountID(req.ToAccountId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid to_account_id")
	}

	input := usecase.TransferInput{
		FromAccountID:  fromID,
		ToAccountID:    toID,
		Amount:         req.Amount,
		Description:    req.Description,
		IdempotencyKey: req.IdempotencyKey,
	}

	out, err := h.transferUC.Transfer(ctx, input)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrAccountNotFound):
			return nil, status.Error(codes.NotFound, err.Error())
		case errors.Is(err, domain.ErrInsufficientBalance):
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		case errors.Is(err, domain.ErrInvalidAmount):
			return nil, status.Error(codes.InvalidArgument, err.Error())
		case errors.Is(err, domain.ErrSelfTransfer):
			return nil, status.Error(codes.InvalidArgument, err.Error())
		case errors.Is(err, domain.ErrInvalidIdempotencyKey):
			return nil, status.Error(codes.InvalidArgument, err.Error())
		case errors.Is(err, domain.ErrAmountTooLarge):
			return nil, status.Error(codes.InvalidArgument, err.Error())
		case errors.Is(err, domain.ErrDescriptionTooLong):
			return nil, status.Error(codes.InvalidArgument, err.Error())
		case errors.Is(err, domain.ErrBalanceOverflow):
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		default:
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	return &pb.TransferResponse{
		JournalEntryId: out.JournalEntryID.String(),
		CreatedAt:      timestamppb.New(out.CreatedAt),
	}, nil
}

func (h *CornucopiaHandler) GetJournalEntries(ctx context.Context, req *pb.GetJournalEntriesRequest) (*pb.GetJournalEntriesResponse, error) {
	id, err := parseAccountID(req.AccountId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid account_id")
	}

	entries, err := h.transferUC.GetJournalEntries(ctx, id, int(req.Limit), int(req.Offset))

	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	pbEntries := make([]*pb.JournalEntry, len(entries))
	for i, e := range entries {
		pbEntries[i] = &pb.JournalEntry{
			JournalEntryId: e.ID.String(),
			FromAccountId:  e.FromAccountID.String(),
			ToAccountId:    e.ToAccountID.String(),
			Amount:         e.Amount,
			Description:    e.Description,
			CreatedAt:      timestamppb.New(e.Timestamp),
		}
	}

	return &pb.GetJournalEntriesResponse{
		JournalEntries: pbEntries,
	}, nil
}

func (h *CornucopiaHandler) GetAccounts(ctx context.Context, req *pb.GetAccountsRequest) (*pb.GetAccountsResponse, error) {
	ids := make([]domain.AccountID, 0, len(req.AccountIds))
	for _, idStr := range req.AccountIds {
		id, err := parseAccountID(idStr)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid account_id: "+idStr)
		}
		ids = append(ids, id)
	}

	accounts, err := h.accountUC.GetAccounts(ctx, ids)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	pbAccounts := make([]*pb.Account, len(accounts))
	for i, acc := range accounts {
		pbAccounts[i] = &pb.Account{
			AccountId:    acc.ID.String(),
			Balance:      acc.Balance,
			CanOverdraft: acc.CanOverdraft,
		}
	}

	return &pb.GetAccountsResponse{
		Accounts: pbAccounts,
	}, nil
}

func (h *CornucopiaHandler) ListAccounts(ctx context.Context, req *pb.ListAccountsRequest) (*pb.ListAccountsResponse, error) {
	// Build filter
	filter := domain.AccountFilter{}
	if req.MinBalance != nil {
		filter.MinBalance = req.MinBalance
	}
	if req.MaxBalance != nil {
		filter.MaxBalance = req.MaxBalance
	}
	if req.CanOverdraft != nil {
		filter.CanOverdraft = req.CanOverdraft
	}

	// Build sort
	sort := domain.AccountSort{}
	switch req.SortField {
	case pb.SortField_SORT_FIELD_BALANCE:
		sort.Field = domain.SortByBalance
	case pb.SortField_SORT_FIELD_ACCOUNT_ID:
		sort.Field = domain.SortByAccountID
	default:
		sort.Field = domain.SortByAccountID
	}
	switch req.SortOrder {
	case pb.SortOrder_SORT_ORDER_DESC:
		sort.Order = domain.SortDesc
	default:
		sort.Order = domain.SortAsc
	}

	input := usecase.ListAccountsInput{
		Filter: filter,
		Sort:   sort,
		Limit:  int(req.Limit),
		Offset: int(req.Offset),
	}

	out, err := h.accountUC.ListAccounts(ctx, input)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	pbAccounts := make([]*pb.Account, len(out.Accounts))
	for i, acc := range out.Accounts {
		pbAccounts[i] = &pb.Account{
			AccountId:    acc.ID.String(),
			Balance:      acc.Balance,
			CanOverdraft: acc.CanOverdraft,
		}
	}

	return &pb.ListAccountsResponse{
		Accounts:   pbAccounts,
		TotalCount: int32(out.TotalCount),
	}, nil
}
