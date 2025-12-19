package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
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
	query := `
		INSERT INTO accounts (id, balance, can_overdraft) 
		VALUES (?, ?, ?)
		ON DUPLICATE KEY UPDATE balance = VALUES(balance), can_overdraft = VALUES(can_overdraft)
	`
	idBytes := uuid.UUID(account.ID)
	_, err := r.getExecutor(ctx).ExecContext(ctx, query, idBytes[:], account.Balance, account.CanOverdraft)
	return err
}

func (r *MariaDBRepository) FindAccountByID(ctx context.Context, id domain.AccountID) (*domain.Account, error) {
	query := "SELECT id, balance, can_overdraft FROM accounts WHERE id = ?"
	idBytes := uuid.UUID(id)
	row := r.getExecutor(ctx).QueryRowContext(ctx, query, idBytes[:])

	var idRaw uuid.UUID
	var acc domain.Account
	if err := row.Scan(&idRaw, &acc.Balance, &acc.CanOverdraft); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Not found
		}
		return nil, err
	}
	acc.ID = domain.AccountID(idRaw)
	return &acc, nil
}

func (r *MariaDBRepository) FindAccountsByIDs(ctx context.Context, ids []domain.AccountID) ([]*domain.Account, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	// Build placeholders for IN clause
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		idBytes := uuid.UUID(id)
		args[i] = idBytes[:]
	}

	query := fmt.Sprintf(
		"SELECT id, balance, can_overdraft FROM accounts WHERE id IN (%s)",
		strings.Join(placeholders, ","),
	)

	rows, err := r.getExecutor(ctx).QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []*domain.Account
	for rows.Next() {
		var idRaw uuid.UUID
		var acc domain.Account
		if err := rows.Scan(&idRaw, &acc.Balance, &acc.CanOverdraft); err != nil {
			return nil, err
		}
		acc.ID = domain.AccountID(idRaw)
		accounts = append(accounts, &acc)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return accounts, nil
}


func (r *MariaDBRepository) GetAccountForUpdate(ctx context.Context, id domain.AccountID) (*domain.Account, error) {
	query := "SELECT id, balance, can_overdraft FROM accounts WHERE id = ? FOR UPDATE"
	idBytes := uuid.UUID(id)
	row := r.getExecutor(ctx).QueryRowContext(ctx, query, idBytes[:])

	var idRaw uuid.UUID
	var acc domain.Account
	if err := row.Scan(&idRaw, &acc.Balance, &acc.CanOverdraft); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Not found
		}
		return nil, err
	}
	acc.ID = domain.AccountID(idRaw)
	return &acc, nil
}

func (r *MariaDBRepository) ListAccounts(ctx context.Context, filter domain.AccountFilter, sort domain.AccountSort, limit, offset int) ([]*domain.Account, int, error) {
	// Build WHERE clause dynamically
	var conditions []string
	var args []any

	if filter.MinBalance != nil {
		conditions = append(conditions, "balance >= ?")
		args = append(args, *filter.MinBalance)
	}
	if filter.MaxBalance != nil {
		conditions = append(conditions, "balance <= ?")
		args = append(args, *filter.MaxBalance)
	}
	if filter.CanOverdraft != nil {
		conditions = append(conditions, "can_overdraft = ?")
		args = append(args, *filter.CanOverdraft)
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + conditions[0]
		for i := 1; i < len(conditions); i++ {
			whereClause += " AND " + conditions[i]
		}
	}

	// Get total count
	countQuery := "SELECT COUNT(*) FROM accounts " + whereClause
	var totalCount int
	if err := r.getExecutor(ctx).QueryRowContext(ctx, countQuery, args...).Scan(&totalCount); err != nil {
		return nil, 0, err
	}

	// Build ORDER BY clause (whitelist to prevent SQL injection)
	orderBy := "id" // default
	switch sort.Field {
	case domain.SortByBalance:
		orderBy = "balance"
	case domain.SortByAccountID:
		orderBy = "id"
	}
	orderDir := "ASC"
	if sort.Order == domain.SortDesc {
		orderDir = "DESC"
	}

	// Build final query
	query := fmt.Sprintf(
		"SELECT id, balance, can_overdraft FROM accounts %s ORDER BY %s %s LIMIT ? OFFSET ?",
		whereClause, orderBy, orderDir,
	)
	args = append(args, limit, offset)

	rows, err := r.getExecutor(ctx).QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var accounts []*domain.Account
	for rows.Next() {
		var idRaw uuid.UUID
		var acc domain.Account
		if err := rows.Scan(&idRaw, &acc.Balance, &acc.CanOverdraft); err != nil {
			return nil, 0, err
		}
		acc.ID = domain.AccountID(idRaw)
		accounts = append(accounts, &acc)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return accounts, totalCount, nil
}

// -- JournalEntryRepository --

func (r *MariaDBRepository) SaveJournalEntry(ctx context.Context, tx *domain.JournalEntry) error {
	query := `
		INSERT INTO transactions 
		(id, from_account_id, to_account_id, amount, description, idempotency_key, prev_hash, hash, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	// Convert UUIDs to byte slices for BINARY(16) storage.
	idBytes := uuid.UUID(tx.ID)
	fromBytes := uuid.UUID(tx.FromAccountID)
	toBytes := uuid.UUID(tx.ToAccountID)
	_, err := r.getExecutor(ctx).ExecContext(ctx, query,
		idBytes[:],
		fromBytes[:],
		toBytes[:],
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
	idBytes := uuid.UUID(id)
	row := r.getExecutor(ctx).QueryRowContext(ctx, query, idBytes[:])
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
	query := `
		SELECT id, from_account_id, to_account_id, amount, description, idempotency_key, prev_hash, hash, created_at 
		FROM transactions 
		ORDER BY id DESC 
		LIMIT 1 FOR UPDATE
	`
	row := r.getExecutor(ctx).QueryRowContext(ctx, query)
	return scanJournalEntry(row)
}

func (r *MariaDBRepository) FindByAccountID(ctx context.Context, accountID domain.AccountID, limit, offset int) ([]*domain.JournalEntry, error) {
	query := `
		SELECT id, from_account_id, to_account_id, amount, description, idempotency_key, prev_hash, hash, created_at 
		FROM transactions 
		WHERE from_account_id = ? OR to_account_id = ?
		ORDER BY id DESC
		LIMIT ? OFFSET ?
	`
	accIDBytes := uuid.UUID(accountID)
	rows, err := r.getExecutor(ctx).QueryContext(ctx, query, accIDBytes[:], accIDBytes[:], limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var txs []*domain.JournalEntry
	for rows.Next() {
		var idRaw, fromRaw, toRaw uuid.UUID
		var tx domain.JournalEntry
		if err := rows.Scan(&idRaw, &fromRaw, &toRaw, &tx.Amount, &tx.Description, &tx.IdempotencyKey, &tx.PreviousHash, &tx.Hash, &tx.Timestamp); err != nil {
			return nil, err
		}
		tx.ID = domain.JournalEntryID(idRaw)
		tx.FromAccountID = domain.AccountID(fromRaw)
		tx.ToAccountID = domain.AccountID(toRaw)
		txs = append(txs, &tx)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return txs, nil
}

// Helper to scan single row
func scanJournalEntry(row *sql.Row) (*domain.JournalEntry, error) {
	var idRaw, fromRaw, toRaw uuid.UUID
	var tx domain.JournalEntry
	err := row.Scan(
		&idRaw,
		&fromRaw,
		&toRaw,
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
	tx.ID = domain.JournalEntryID(idRaw)
	tx.FromAccountID = domain.AccountID(fromRaw)
	tx.ToAccountID = domain.AccountID(toRaw)
	return &tx, nil
}
