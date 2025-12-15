package grpc

import (
	"context"
	"errors"

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

func (h *CornucopiaHandler) CreateAccount(ctx context.Context, req *pb.CreateAccountRequest) (*pb.CreateAccountResponse, error) {
	acc, err := h.accountUC.CreateAccount(ctx, req.OwnerId, req.CanOverdraft)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pb.CreateAccountResponse{
		AccountId:    string(acc.ID),
		OwnerId:      string(acc.OwnerID),
		Balance:      acc.Balance,
		CanOverdraft: acc.CanOverdraft,
	}, nil
}

func (h *CornucopiaHandler) GetAccount(ctx context.Context, req *pb.GetAccountRequest) (*pb.GetAccountResponse, error) {
	acc, err := h.accountUC.GetAccount(ctx, req.AccountId)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if acc == nil {
		return nil, status.Error(codes.NotFound, "account not found")
	}
	return &pb.GetAccountResponse{
		AccountId:    string(acc.ID),
		OwnerId:      string(acc.OwnerID),
		Balance:      acc.Balance,
		CanOverdraft: acc.CanOverdraft,
	}, nil
}

func (h *CornucopiaHandler) Transfer(ctx context.Context, req *pb.TransferRequest) (*pb.TransferResponse, error) {
	input := usecase.TransferInput{
		FromAccountID:  req.FromAccountId,
		ToAccountID:    req.ToAccountId,
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
		JournalEntryId: out.JournalEntryID,
		CreatedAt:      timestamppb.New(out.CreatedAt),
	}, nil
}

func (h *CornucopiaHandler) GetJournalEntries(ctx context.Context, req *pb.GetJournalEntriesRequest) (*pb.GetJournalEntriesResponse, error) {
	entries, err := h.transferUC.GetJournalEntries(ctx, req.AccountId, int(req.Limit), int(req.Offset))
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	pbEntries := make([]*pb.JournalEntry, len(entries))
	for i, e := range entries {
		pbEntries[i] = &pb.JournalEntry{
			JournalEntryId: string(e.ID),
			FromAccountId:  string(e.FromAccountID),
			ToAccountId:    string(e.ToAccountID),
			Amount:         e.Amount,
			Description:    e.Description,
			CreatedAt:      timestamppb.New(e.Timestamp),
		}
	}

	return &pb.GetJournalEntriesResponse{
		JournalEntries: pbEntries,
	}, nil
}
