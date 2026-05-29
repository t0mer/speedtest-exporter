package notifications

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/t0mer/speedtest-exporter/internal/crypto"
)

// Store manages notification channels in SQLite with AES-256-GCM encrypted credentials.
type Store struct {
	db  *sql.DB
	key []byte
}

// NewStore creates a Store. db is the raw *sql.DB from database.DB.SQL().
func NewStore(db *sql.DB, key []byte) *Store {
	return &Store{db: db, key: key}
}

// List returns all channels ordered by id. Config is decrypted.
func (s *Store) List(ctx context.Context) ([]Channel, error) {
	const q = `SELECT id, name, provider, config_encrypted, enabled, notify_on_success, notify_on_failure
		FROM notification_channels ORDER BY id`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list channels: %w", err)
	}
	defer rows.Close()
	channels := make([]Channel, 0)
	for rows.Next() {
		ch, err := s.scan(rows.Scan)
		if err != nil {
			return nil, err
		}
		channels = append(channels, *ch)
	}
	return channels, rows.Err()
}

// Get returns the channel with the given id, or nil if not found.
func (s *Store) Get(ctx context.Context, id int64) (*Channel, error) {
	const q = `SELECT id, name, provider, config_encrypted, enabled, notify_on_success, notify_on_failure
		FROM notification_channels WHERE id = ?`
	return s.scan(s.db.QueryRowContext(ctx, q, id).Scan)
}

// Save inserts a new channel and sets ch.ID.
func (s *Store) Save(ctx context.Context, ch *Channel) error {
	enc, err := crypto.Encrypt(s.key, ch.Config)
	if err != nil {
		return fmt.Errorf("encrypt config: %w", err)
	}
	const q = `INSERT INTO notification_channels
		(name, provider, config_encrypted, enabled, notify_on_success, notify_on_failure, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?)`
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx, q,
		ch.Name, string(ch.Provider), enc,
		btoi(ch.Enabled), btoi(ch.NotifyOnSuccess), btoi(ch.NotifyOnFailure),
		now, now,
	)
	if err != nil {
		return fmt.Errorf("insert channel: %w", err)
	}
	id, _ := res.LastInsertId()
	ch.ID = id
	return nil
}

// Update replaces all mutable fields of an existing channel.
func (s *Store) Update(ctx context.Context, ch *Channel) error {
	enc, err := crypto.Encrypt(s.key, ch.Config)
	if err != nil {
		return fmt.Errorf("encrypt config: %w", err)
	}
	const q = `UPDATE notification_channels
		SET name=?, provider=?, config_encrypted=?, enabled=?, notify_on_success=?, notify_on_failure=?, updated_at=?
		WHERE id=?`
	res, err := s.db.ExecContext(ctx, q,
		ch.Name, string(ch.Provider), enc,
		btoi(ch.Enabled), btoi(ch.NotifyOnSuccess), btoi(ch.NotifyOnFailure),
		time.Now().UTC(), ch.ID,
	)
	if err != nil {
		return fmt.Errorf("update channel: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("channel %d not found", ch.ID)
	}
	return nil
}

// Delete removes the channel with the given id.
func (s *Store) Delete(ctx context.Context, id int64) error {
	const q = `DELETE FROM notification_channels WHERE id = ?`
	res, err := s.db.ExecContext(ctx, q, id)
	if err != nil {
		return fmt.Errorf("delete channel: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("channel %d not found", id)
	}
	return nil
}

// DeleteAll removes all notification channels.
func (s *Store) DeleteAll(ctx context.Context) error {
	const q = `DELETE FROM notification_channels`
	if _, err := s.db.ExecContext(ctx, q); err != nil {
		return fmt.Errorf("delete all channels: %w", err)
	}
	return nil
}

// ReplaceAll atomically deletes all channels and inserts the given list.
func (s *Store) ReplaceAll(ctx context.Context, channels []Channel) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM notification_channels`); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("delete channels: %w", err)
	}
	for _, ch := range channels {
		enc, err := crypto.Encrypt(s.key, ch.Config)
		if err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("encrypt channel %q: %w", ch.Name, err)
		}
		const q = `INSERT INTO notification_channels
			(name, provider, config_encrypted, enabled, notify_on_success, notify_on_failure, created_at, updated_at)
			VALUES (?,?,?,?,?,?,?,?)`
		now := time.Now().UTC()
		if _, err := tx.ExecContext(ctx, q,
			ch.Name, string(ch.Provider), enc,
			btoi(ch.Enabled), btoi(ch.NotifyOnSuccess), btoi(ch.NotifyOnFailure),
			now, now,
		); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("insert channel %q: %w", ch.Name, err)
		}
	}
	return tx.Commit()
}

func (s *Store) scan(scan func(...any) error) (*Channel, error) {
	var ch Channel
	var prov string
	var enc []byte
	var en, onS, onF int
	if err := scan(&ch.ID, &ch.Name, &prov, &enc, &en, &onS, &onF); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("scan channel: %w", err)
	}
	ch.Provider = Provider(prov)
	ch.Enabled = en != 0
	ch.NotifyOnSuccess = onS != 0
	ch.NotifyOnFailure = onF != 0
	plain, err := crypto.Decrypt(s.key, enc)
	if err != nil {
		return nil, fmt.Errorf("decrypt config for channel %d: %w", ch.ID, err)
	}
	ch.Config = plain
	return &ch, nil
}

func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}
