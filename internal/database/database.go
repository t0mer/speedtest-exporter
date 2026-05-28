// Package database provides a SQLite-backed store for speed test results.
package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"

	"github.com/t0mer/speedtest-exporter/internal/model"
)

const schema = `
CREATE TABLE IF NOT EXISTS results (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp     DATETIME NOT NULL,
    source        TEXT NOT NULL,
    engine        TEXT NOT NULL,
    download_mbps REAL NOT NULL DEFAULT 0,
    upload_mbps   REAL NOT NULL DEFAULT 0,
    ping_ms       REAL NOT NULL DEFAULT 0,
    jitter_ms     REAL NOT NULL DEFAULT 0,
    packet_loss   REAL NOT NULL DEFAULT 0,
    server_name   TEXT NOT NULL DEFAULT '',
    server_id     TEXT NOT NULL DEFAULT '',
    isp           TEXT NOT NULL DEFAULT '',
    external_ip   TEXT NOT NULL DEFAULT '',
    duration_sec  REAL NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_results_timestamp ON results(timestamp DESC);
CREATE TABLE IF NOT EXISTS settings (
    id      INTEGER PRIMARY KEY CHECK (id = 1),
    data    TEXT NOT NULL,
    updated DATETIME NOT NULL
);
`

// DB wraps a SQLite connection.
type DB struct {
	db *sql.DB
}

// Open opens or creates the SQLite database in dir and applies the schema.
func Open(dir string) (*DB, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	dbPath := filepath.Join(dir, "results.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1) // SQLite: single writer
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	return &DB{db: db}, nil
}

// Close closes the underlying database connection.
func (d *DB) Close() error { return d.db.Close() }

// Save persists r and sets r.ID to the new row's ID.
func (d *DB) Save(ctx context.Context, r *model.Result) error {
	const q = `INSERT INTO results
		(timestamp, source, engine, download_mbps, upload_mbps, ping_ms, jitter_ms,
		 packet_loss, server_name, server_id, isp, external_ip, duration_sec)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`
	res, err := d.db.ExecContext(ctx, q,
		r.Timestamp.UTC(), string(r.Source), string(r.Engine),
		r.DownloadMbps, r.UploadMbps, r.PingMs, r.JitterMs,
		r.PacketLoss, r.ServerName, r.ServerID, r.ISP, r.ExternalIP, r.DurationSec,
	)
	if err != nil {
		return fmt.Errorf("insert result: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("last insert id: %w", err)
	}
	r.ID = id
	return nil
}

// Get returns the result with the given id, or nil if not found.
func (d *DB) Get(ctx context.Context, id int64) (*model.Result, error) {
	const q = `SELECT id, timestamp, source, engine, download_mbps, upload_mbps,
		ping_ms, jitter_ms, packet_loss, server_name, server_id, isp, external_ip,
		duration_sec FROM results WHERE id = ?`
	row := d.db.QueryRowContext(ctx, q, id)
	return scanRow(row.Scan)
}

// Latest returns the most recent result, or nil if the table is empty.
func (d *DB) Latest(ctx context.Context) (*model.Result, error) {
	const q = `SELECT id, timestamp, source, engine, download_mbps, upload_mbps,
		ping_ms, jitter_ms, packet_loss, server_name, server_id, isp, external_ip,
		duration_sec FROM results ORDER BY timestamp DESC LIMIT 1`
	row := d.db.QueryRowContext(ctx, q)
	return scanRow(row.Scan)
}

// ListOptions filters for List queries.
type ListOptions struct {
	Limit  int
	Offset int
	Since  time.Time
	Until  time.Time
}

// List returns results matching opts, newest first.
func (d *DB) List(ctx context.Context, opts ListOptions) ([]model.Result, error) {
	limit := opts.Limit
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	since := opts.Since
	if since.IsZero() {
		since = time.Unix(0, 0)
	}
	until := opts.Until
	if until.IsZero() {
		until = time.Now().Add(24 * time.Hour)
	}
	const q = `SELECT id, timestamp, source, engine, download_mbps, upload_mbps,
		ping_ms, jitter_ms, packet_loss, server_name, server_id, isp, external_ip,
		duration_sec FROM results
		WHERE timestamp BETWEEN ? AND ?
		ORDER BY timestamp DESC LIMIT ? OFFSET ?`
	rows, err := d.db.QueryContext(ctx, q, since.UTC(), until.UTC(), limit, opts.Offset)
	if err != nil {
		return nil, fmt.Errorf("list results: %w", err)
	}
	defer rows.Close()
	results := make([]model.Result, 0)
	for rows.Next() {
		r, err := scanRow(rows.Scan)
		if err != nil {
			return nil, err
		}
		results = append(results, *r)
	}
	return results, rows.Err()
}

// Summary holds aggregate statistics over a time window.
type Summary struct {
	Count       int     `json:"count"`
	AvgDownload float64 `json:"avg_download_mbps"`
	AvgUpload   float64 `json:"avg_upload_mbps"`
	AvgPing     float64 `json:"avg_ping_ms"`
	MinDownload float64 `json:"min_download_mbps"`
	MaxDownload float64 `json:"max_download_mbps"`
	MinUpload   float64 `json:"min_upload_mbps"`
	MaxUpload   float64 `json:"max_upload_mbps"`
	MinPing     float64 `json:"min_ping_ms"`
	MaxPing     float64 `json:"max_ping_ms"`
	Days        int     `json:"days"`
}

// Summary queries aggregate stats for the last `days` days.
func (d *DB) Summary(ctx context.Context, days int) (*Summary, error) {
	since := time.Now().UTC().AddDate(0, 0, -days)
	const q = `SELECT COUNT(*),
		COALESCE(AVG(download_mbps),0), COALESCE(AVG(upload_mbps),0), COALESCE(AVG(ping_ms),0),
		COALESCE(MIN(download_mbps),0), COALESCE(MAX(download_mbps),0),
		COALESCE(MIN(upload_mbps),0),   COALESCE(MAX(upload_mbps),0),
		COALESCE(MIN(ping_ms),0),       COALESCE(MAX(ping_ms),0)
		FROM results WHERE timestamp >= ?`
	row := d.db.QueryRowContext(ctx, q, since)
	s := &Summary{Days: days}
	err := row.Scan(&s.Count,
		&s.AvgDownload, &s.AvgUpload, &s.AvgPing,
		&s.MinDownload, &s.MaxDownload,
		&s.MinUpload, &s.MaxUpload,
		&s.MinPing, &s.MaxPing,
	)
	if err != nil {
		return nil, fmt.Errorf("summary query: %w", err)
	}
	return s, nil
}

// GetSettings returns the persisted runtime settings, or nil if none have been saved.
func (d *DB) GetSettings(ctx context.Context) (*model.Settings, error) {
	const q = `SELECT data FROM settings WHERE id = 1`
	row := d.db.QueryRowContext(ctx, q)
	var data string
	if err := row.Scan(&data); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get settings: %w", err)
	}
	var s model.Settings
	if err := json.Unmarshal([]byte(data), &s); err != nil {
		return nil, fmt.Errorf("unmarshal settings: %w", err)
	}
	return &s, nil
}

// SaveSettings persists settings, replacing any previously saved row.
func (d *DB) SaveSettings(ctx context.Context, s *model.Settings) error {
	data, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}
	const q = `INSERT INTO settings (id, data, updated) VALUES (1, ?, ?)
		ON CONFLICT(id) DO UPDATE SET data = excluded.data, updated = excluded.updated`
	if _, err := d.db.ExecContext(ctx, q, string(data), time.Now().UTC()); err != nil {
		return fmt.Errorf("save settings: %w", err)
	}
	return nil
}

// scanRow is a generic row scanner that works with both *sql.Row.Scan and rows.Scan.
func scanRow(scan func(...any) error) (*model.Result, error) {
	r := &model.Result{}
	var src, eng string
	err := scan(
		&r.ID, &r.Timestamp, &src, &eng,
		&r.DownloadMbps, &r.UploadMbps, &r.PingMs, &r.JitterMs,
		&r.PacketLoss, &r.ServerName, &r.ServerID, &r.ISP, &r.ExternalIP, &r.DurationSec,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan result: %w", err)
	}
	r.Source = model.Source(src)
	r.Engine = model.Engine(eng)
	return r, nil
}
