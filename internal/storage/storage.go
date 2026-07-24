package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func Open(dataDir string) (*Store, error) {
	dsn := "file:" + filepath.Join(dataDir, "tgbotd.db") + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS bots (
			token       TEXT PRIMARY KEY,
			bot_id      INTEGER NOT NULL,
			created_at  INTEGER NOT NULL,
			last_seen   INTEGER NOT NULL,
			webhook_url TEXT DEFAULT '',
			webhook_secret TEXT DEFAULT '',
			webhook_ip TEXT DEFAULT '',
			max_conns   INTEGER DEFAULT 40,
			allowed_updates TEXT DEFAULT '',
			drop_pending INTEGER DEFAULT 0,
			cert_pem    BLOB
		)`,
		`CREATE TABLE IF NOT EXISTS sessions (
			token   TEXT PRIMARY KEY,
			dc_id   INTEGER NOT NULL,
			auth_key BLOB NOT NULL,
			hash    BLOB NOT NULL,
			salt    INTEGER NOT NULL,
			hostname TEXT NOT NULL,
			updated_at INTEGER NOT NULL,
			FOREIGN KEY(token) REFERENCES bots(token) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS peers (
			token       TEXT NOT NULL,
			peer_id     INTEGER NOT NULL,
			peer_type   INTEGER NOT NULL,
			access_hash INTEGER NOT NULL,
			username    TEXT DEFAULT '',
			PRIMARY KEY(token, peer_id),
			FOREIGN KEY(token) REFERENCES bots(token) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_peers_username ON peers(token, username)`,
		`CREATE TABLE IF NOT EXISTS updates_queue (
			token    TEXT NOT NULL,
			seq      INTEGER PRIMARY KEY AUTOINCREMENT,
			payload  BLOB NOT NULL,
			created_at INTEGER NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_updates_token ON updates_queue(token, seq)`,
		`CREATE TABLE IF NOT EXISTS file_map (
			file_id_hash TEXT PRIMARY KEY,
			file_id      TEXT NOT NULL,
			file_unique_id TEXT NOT NULL,
			dc_id        INTEGER NOT NULL,
			owner_id     INTEGER NOT NULL,
			mtproto_type INTEGER NOT NULL,
			payload      BLOB NOT NULL,
			size         INTEGER DEFAULT 0,
			created_at   INTEGER NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_file_unique ON file_map(file_unique_id)`,
	}
	for _, q := range stmts {
		if _, err := s.db.Exec(q); err != nil {
			return fmt.Errorf("migrate %.60s...: %w", q, err)
		}
	}
	return nil
}

type BotRow struct {
	Token          string
	BotID          int64
	CreatedAt      time.Time
	LastSeen       time.Time
	WebhookURL     string
	WebhookSecret  string
	WebhookIP      string
	MaxConns       int
	AllowedUpdates string
	DropPending    bool
}

func (s *Store) ListBotTokens(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT token FROM bots ORDER BY last_seen DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) UpsertBot(ctx context.Context, token string, botID int64) error {
	now := time.Now().Unix()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO bots(token, bot_id, created_at, last_seen) VALUES(?,?,?,?)
		ON CONFLICT(token) DO UPDATE SET bot_id=excluded.bot_id, last_seen=excluded.last_seen
	`, token, botID, now, now)
	return err
}

func (s *Store) GetBot(ctx context.Context, token string) (*BotRow, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT token, bot_id, created_at, last_seen, webhook_url, webhook_secret, webhook_ip, max_conns, allowed_updates, drop_pending
		FROM bots WHERE token = ?`, token)
	var b BotRow
	var created, last int64
	var dropPending int
	if err := row.Scan(&b.Token, &b.BotID, &created, &last, &b.WebhookURL, &b.WebhookSecret, &b.WebhookIP, &b.MaxConns, &b.AllowedUpdates, &dropPending); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	b.CreatedAt = time.Unix(created, 0)
	b.LastSeen = time.Unix(last, 0)
	b.DropPending = dropPending != 0
	return &b, nil
}

func (s *Store) SetWebhook(ctx context.Context, token, url, secret, ip string, maxConns int, allowedUpdates string, dropPending bool) error {
	dp := 0
	if dropPending {
		dp = 1
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE bots SET webhook_url=?, webhook_secret=?, webhook_ip=?, max_conns=?, allowed_updates=?, drop_pending=? WHERE token=?
	`, url, secret, ip, maxConns, allowedUpdates, dp, token)
	return err
}

type SessionRow struct {
	Token    string
	DC       int
	AuthKey  []byte
	Hash     []byte
	Salt     int64
	Hostname string
}

func (s *Store) LoadSession(ctx context.Context, token string) (*SessionRow, error) {
	row := s.db.QueryRowContext(ctx, `SELECT token, dc_id, auth_key, hash, salt, hostname FROM sessions WHERE token=?`, token)
	var r SessionRow
	if err := row.Scan(&r.Token, &r.DC, &r.AuthKey, &r.Hash, &r.Salt, &r.Hostname); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &r, nil
}

func (s *Store) SaveSession(ctx context.Context, r *SessionRow) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO sessions(token, dc_id, auth_key, hash, salt, hostname, updated_at) VALUES(?,?,?,?,?,?,?)
		ON CONFLICT(token) DO UPDATE SET dc_id=excluded.dc_id, auth_key=excluded.auth_key, hash=excluded.hash, salt=excluded.salt, hostname=excluded.hostname, updated_at=excluded.updated_at
	`, r.Token, r.DC, r.AuthKey, r.Hash, r.Salt, r.Hostname, time.Now().Unix())
	return err
}

func (s *Store) EnqueueUpdate(ctx context.Context, token string, payload []byte) (int64, error) {
	res, err := s.db.ExecContext(ctx, `INSERT INTO updates_queue(token, payload, created_at) VALUES(?,?,?)`,
		token, payload, time.Now().Unix())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) FetchUpdates(ctx context.Context, token string, offset int64, limit int) ([]struct {
	Seq     int64
	Payload []byte
}, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT seq, payload FROM updates_queue WHERE token=? AND seq >= ? ORDER BY seq ASC LIMIT ?`,
		token, offset, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []struct {
		Seq     int64
		Payload []byte
	}
	for rows.Next() {
		var it struct {
			Seq     int64
			Payload []byte
		}
		if err := rows.Scan(&it.Seq, &it.Payload); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func (s *Store) AckUpdates(ctx context.Context, token string, upTo int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM updates_queue WHERE token=? AND seq < ?`, token, upTo)
	return err
}

func (s *Store) DropUpdates(ctx context.Context, token string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM updates_queue WHERE token=?`, token)
	return err
}

type FileRow struct {
	FileIDHash   string
	FileID       string
	FileUniqueID string
	DCID         int
	OwnerID      int64
	MTProtoType  int
	Payload      []byte
	Size         int64
}

func (s *Store) PutFile(ctx context.Context, r *FileRow) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO file_map(file_id_hash, file_id, file_unique_id, dc_id, owner_id, mtproto_type, payload, size, created_at)
		VALUES(?,?,?,?,?,?,?,?,?)
		ON CONFLICT(file_id_hash) DO UPDATE SET file_id=excluded.file_id, file_unique_id=excluded.file_unique_id, payload=excluded.payload, size=excluded.size
	`, r.FileIDHash, r.FileID, r.FileUniqueID, r.DCID, r.OwnerID, r.MTProtoType, r.Payload, r.Size, time.Now().Unix())
	return err
}

func (s *Store) GetFileByID(ctx context.Context, fileIDHash string) (*FileRow, error) {
	row := s.db.QueryRowContext(ctx, `SELECT file_id_hash, file_id, file_unique_id, dc_id, owner_id, mtproto_type, payload, size FROM file_map WHERE file_id_hash=?`, fileIDHash)
	var r FileRow
	if err := row.Scan(&r.FileIDHash, &r.FileID, &r.FileUniqueID, &r.DCID, &r.OwnerID, &r.MTProtoType, &r.Payload, &r.Size); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &r, nil
}
