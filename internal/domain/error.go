package domain

import (
	"errors"
)

var (
	// ErrAccountNotFound indicates that the requested account was not found.
	ErrAccountNotFound = errors.New("account not found")
	
	// ErrInsufficientBalance indicates that the account has insufficient funds.
	ErrInsufficientBalance = errors.New("insufficient balance")
	
	// ErrInvalidAmount indicates that the amount is invalid (e.g. negative or zero).
	ErrInvalidAmount = errors.New("amount must be positive")
	
	// ErrSelfTransfer indicates that the source and destination accounts are the same.
	ErrSelfTransfer = errors.New("cannot transfer to self")

	// ErrBalanceOverflow indicates that the operation would cause a balance overflow.
	ErrBalanceOverflow = errors.New("balance would overflow")

	// ErrInvalidIdempotencyKey indicates that the idempotency key is invalid or empty.
	ErrInvalidIdempotencyKey = errors.New("idempotency key must not be empty")

	// ErrAmountTooLarge indicates that the transfer amount exceeds the maximum allowed.
	ErrAmountTooLarge = errors.New("amount exceeds maximum allowed value")

	// ErrInvalidLimit indicates that the limit parameter is invalid.
	ErrInvalidLimit = errors.New("limit must be between 1 and 1000")

	// ErrDescriptionTooLong indicates that the description exceeds the maximum length.
	ErrDescriptionTooLong = errors.New("description is too long")
)

// Sentinel Error Wrapping helpers (optional, but keep simple for now)
func IsError(err error, target error) bool {
	return errors.Is(err, target)
}
