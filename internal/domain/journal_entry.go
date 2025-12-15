package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// JournalEntryID is a UUID.
type JournalEntryID string

// JournalEntry represents an immutable record of money movement.
type JournalEntry struct {
	ID             JournalEntryID
	FromAccountID  AccountID
	ToAccountID    AccountID
	Amount         int64
	Description    string
	IdempotencyKey string

	// Integrity
	PreviousHash string
	Hash         string
	Timestamp    time.Time
}

// ComputeHash calculates the hash of the journal entry including the previous hash.
// Hash = SHA256(PrevHash + ID + From + To + Amount + Timestamp + Idempotency)
func (t *JournalEntry) ComputeHash() string {
	payload := fmt.Sprintf("%s:%s:%s:%s:%d:%d:%s",
		t.PreviousHash,
		t.ID,
		t.FromAccountID,
		t.ToAccountID,
		t.Amount,
		t.Timestamp.UnixNano(),
		t.IdempotencyKey,
	)
	hash := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(hash[:])
}

// ValidateHash checks if the current Hash matches the computed hash.
func (t *JournalEntry) ValidateHash() bool {
	return t.Hash == t.ComputeHash()
}
