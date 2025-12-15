package repository

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
	"github.com/traP-jp/plutus/system/cornucopia/internal/domain"
)

type key int

const (
	txKey key = iota
)

// MariaDBRepository implements AccountRepository, TransactionRepository, and TransactionManager.
type MariaDBRepository struct {
	db *sql.DB
}

func NewMariaDBRepository(db *sql.DB) *MariaDBRepository {
	return &MariaDBRepository{db: db}
}

// -- TransactionManager --

func (r *MariaDBRepository) Run(ctx context.Context, fn func(ctx context.Context) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	// Inject tx into context
	txCtx := context.WithValue(ctx, txKey, tx)

	if err := fn(txCtx); err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}



func (r *MariaDBRepository) RunSerialized(ctx context.Context, name string, fn func(ctx context.Context) error) error {
	conn, err := r.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	// 10s timeout for lock acquisition
	var lockRes sql.NullInt64
	err = conn.QueryRowContext(ctx, "SELECT GET_LOCK(?, 10)", name).Scan(&lockRes)
	if err != nil {
		return err
	}
	if !lockRes.Valid || lockRes.Int64 != 1 {
		// 0 = timeout, NULL = error
		return fmt.Errorf("failed to acquire lock %s", name)
	}

	// Ensure unlock happens
	defer func() {
		// Use ExecContext to release. The result (1/0) doesn't strictly matter if we are closing,
		// but best practice to release explicitly.
		conn.ExecContext(ctx, "SELECT RELEASE_LOCK(?)", name)
	}()

	return fn(ctx)
}

func (r *MariaDBRepository) getExecutor(ctx context.Context) interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
} {
	if tx, ok := ctx.Value(txKey).(*sql.Tx); ok {
		return tx
	}
	return r.db
}

// -- AccountRepository --

func (r *MariaDBRepository) SaveAccount(ctx context.Context, account *domain.Account) error {
	// Upsert (On Duplicate Key Update) or just Update if exists?
	// Simplified: Insert or Update.
	// We'll use INSERT ON DUPLICATE KEY UPDATE for simplicity with MariaDB.
	query := `
		INSERT INTO accounts (id, owner_id, balance, can_overdraft) 
		VALUES (?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE balance = VALUES(balance), can_overdraft = VALUES(can_overdraft)
	`
	_, err := r.getExecutor(ctx).ExecContext(ctx, query, account.ID, account.OwnerID, account.Balance, account.CanOverdraft)
	return err
}

func (r *MariaDBRepository) FindAccountByID(ctx context.Context, id domain.AccountID) (*domain.Account, error) {
	query := "SELECT id, owner_id, balance, can_overdraft FROM accounts WHERE id = ?"
	row := r.getExecutor(ctx).QueryRowContext(ctx, query, id)

	var acc domain.Account
	if err := row.Scan(&acc.ID, &acc.OwnerID, &acc.Balance, &acc.CanOverdraft); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Not found
		}
		return nil, err
	}
	return &acc, nil
}

func (r *MariaDBRepository) GetAccountForUpdate(ctx context.Context, id domain.AccountID) (*domain.Account, error) {
	query := "SELECT id, owner_id, balance, can_overdraft FROM accounts WHERE id = ? FOR UPDATE"
	row := r.getExecutor(ctx).QueryRowContext(ctx, query, id)

	var acc domain.Account
	if err := row.Scan(&acc.ID, &acc.OwnerID, &acc.Balance, &acc.CanOverdraft); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Not found
		}
		return nil, err
	}
	return &acc, nil
}

func (r *MariaDBRepository) FindByOwnerID(ctx context.Context, ownerID domain.OwnerID) (*domain.Account, error) {
	query := "SELECT id, owner_id, balance, can_overdraft FROM accounts WHERE owner_id = ?"
	row := r.getExecutor(ctx).QueryRowContext(ctx, query, ownerID)

	var acc domain.Account
	if err := row.Scan(&acc.ID, &acc.OwnerID, &acc.Balance, &acc.CanOverdraft); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &acc, nil
}

func (r *MariaDBRepository) FindByOwnerIDForUpdate(ctx context.Context, ownerID domain.OwnerID) (*domain.Account, error) {
	query := "SELECT id, owner_id, balance, can_overdraft FROM accounts WHERE owner_id = ? FOR UPDATE"
	row := r.getExecutor(ctx).QueryRowContext(ctx, query, ownerID)

	var acc domain.Account
	if err := row.Scan(&acc.ID, &acc.OwnerID, &acc.Balance, &acc.CanOverdraft); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &acc, nil
}

// -- JournalEntryRepository --

func (r *MariaDBRepository) SaveJournalEntry(ctx context.Context, tx *domain.JournalEntry) error {
	query := `
		INSERT INTO transactions 
		(id, from_account_id, to_account_id, amount, description, idempotency_key, prev_hash, hash, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := r.getExecutor(ctx).ExecContext(ctx, query,
		tx.ID,
		tx.FromAccountID,
		tx.ToAccountID,
		tx.Amount,
		tx.Description,
		tx.IdempotencyKey,
		tx.PreviousHash,
		tx.Hash,
		tx.Timestamp,
	)
	return err
}

func (r *MariaDBRepository) FindJournalEntryByID(ctx context.Context, id domain.JournalEntryID) (*domain.JournalEntry, error) {
	query := `
		SELECT id, from_account_id, to_account_id, amount, description, idempotency_key, prev_hash, hash, created_at 
		FROM transactions WHERE id = ?
	`
	row := r.getExecutor(ctx).QueryRowContext(ctx, query, id)
	return scanJournalEntry(row)
}

func (r *MariaDBRepository) FindByIdempotencyKey(ctx context.Context, key string) (*domain.JournalEntry, error) {
	query := `
		SELECT id, from_account_id, to_account_id, amount, description, idempotency_key, prev_hash, hash, created_at 
		FROM transactions WHERE idempotency_key = ?
	`
	row := r.getExecutor(ctx).QueryRowContext(ctx, query, key)
	return scanJournalEntry(row)
}

func (r *MariaDBRepository) GetLatestJournalEntry(ctx context.Context) (*domain.JournalEntry, error) {
	// Assuming strict ordering by created_at or an auto-inc sequence.
	// For hash chain, we usually want the *absolute* last one inserted.
	// NOTE: Locking mechanism might be needed here to ensure linear appending if high concurrency.
	// "SELECT ... FOR UPDATE" if inside verification/gap checks.
	query := `
		SELECT id, from_account_id, to_account_id, amount, description, idempotency_key, prev_hash, hash, created_at 
		FROM transactions 
		ORDER BY created_at DESC, id DESC 
		LIMIT 1 FOR UPDATE
	` 
	// FOR UPDATE locks the row, ensuring serialization if referenced in the same TX.
	// BUT, if the table is empty, FOR UPDATE doesn't lock "the next insert".
	// For strict Chain, we might need a separate "ChainHead" table to lock.
	// For this prototype, we'll assume this is "good enough" or use a table lock if desired.
	
	row := r.getExecutor(ctx).QueryRowContext(ctx, query)
	return scanJournalEntry(row)
}

func (r *MariaDBRepository) FindByAccountID(ctx context.Context, accountID domain.AccountID, limit, offset int) ([]*domain.JournalEntry, error) {
	query := `
		SELECT id, from_account_id, to_account_id, amount, description, idempotency_key, prev_hash, hash, created_at 
		FROM transactions 
		WHERE from_account_id = ? OR to_account_id = ?
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`
	rows, err := r.getExecutor(ctx).QueryContext(ctx, query, accountID, accountID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var txs []*domain.JournalEntry
	for rows.Next() {
		var tx domain.JournalEntry
		if err := rows.Scan(&tx.ID, &tx.FromAccountID, &tx.ToAccountID, &tx.Amount, &tx.Description, &tx.IdempotencyKey, &tx.PreviousHash, &tx.Hash, &tx.Timestamp); err != nil {
			return nil, err
		}
		txs = append(txs, &tx)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return txs, nil
}

// Helper to scan single row
func scanJournalEntry(row *sql.Row) (*domain.JournalEntry, error) {
	var tx domain.JournalEntry
	err := row.Scan(
		&tx.ID,
		&tx.FromAccountID,
		&tx.ToAccountID,
		&tx.Amount,
		&tx.Description,
		&tx.IdempotencyKey,
		&tx.PreviousHash,
		&tx.Hash,
		&tx.Timestamp,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &tx, nil
}
