package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(dsn string) (*PostgresStore, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	return &PostgresStore{db: db}, nil
}

func (s *PostgresStore) Init(ctx context.Context) error {
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id TEXT PRIMARY KEY,
		email TEXT UNIQUE NOT NULL,
		created_at TIMESTAMPTZ NOT NULL
	);
	
	CREATE TABLE IF NOT EXISTS api_keys (
		key_hash TEXT PRIMARY KEY,
		user_id TEXT NOT NULL,
		name TEXT NOT NULL,
		revoked BOOLEAN NOT NULL DEFAULT false,
		created_at TIMESTAMPTZ NOT NULL,
		FOREIGN KEY(user_id) REFERENCES users(id)
	);

	CREATE TABLE IF NOT EXISTS sessions (
		id TEXT PRIMARY KEY,
		user_id TEXT NOT NULL,
		backend TEXT NOT NULL,
		status TEXT NOT NULL,
		created_at TIMESTAMPTZ NOT NULL,
		finished_at TIMESTAMPTZ,
		FOREIGN KEY(user_id) REFERENCES users(id)
	);
	`
	_, err := s.db.ExecContext(ctx, schema)
	return err
}

func (s *PostgresStore) CreateUser(ctx context.Context, email string) (*User, error) {
	id := uuid.New().String()
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, "INSERT INTO users (id, email, created_at) VALUES ($1, $2, $3)", id, email, now)
	if err != nil {
		return nil, err
	}
	return &User{ID: id, Email: email, CreatedAt: now}, nil
}

func (s *PostgresStore) GetUser(ctx context.Context, id string) (*User, error) {
	u := &User{ID: id}
	err := s.db.QueryRowContext(ctx, "SELECT email, created_at FROM users WHERE id = $1", id).Scan(&u.Email, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (s *PostgresStore) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	u := &User{Email: email}
	err := s.db.QueryRowContext(ctx, "SELECT id, created_at FROM users WHERE email = $1", email).Scan(&u.ID, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return u, nil
}

// hashKey creates a SHA-256 hash of the API key to store securely.
func hashKey(rawKey string) string {
	hash := sha256.Sum256([]byte(rawKey))
	return hex.EncodeToString(hash[:])
}

// GenerateAPIKey creates a cryptographically secure key and stores its hash.
func (s *PostgresStore) GenerateAPIKey(ctx context.Context, userID, name string) (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	rawKey := "sb_live_" + hex.EncodeToString(b)
	hashed := hashKey(rawKey)

	_, err := s.db.ExecContext(ctx, "INSERT INTO api_keys (key_hash, user_id, name, revoked, created_at) VALUES ($1, $2, $3, false, $4)",
		hashed, userID, name, time.Now().UTC())
	if err != nil {
		return "", err
	}
	
	// We return the RAW key exactly once so the user can save it.
	return rawKey, nil
}

func (s *PostgresStore) ValidateAPIKey(ctx context.Context, rawKey string) (*User, error) {
	hashed := hashKey(rawKey)
	var userID string
	var revoked bool
	
	err := s.db.QueryRowContext(ctx, "SELECT user_id, revoked FROM api_keys WHERE key_hash = $1", hashed).Scan(&userID, &revoked)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("invalid api key")
		}
		return nil, err
	}
	
	if revoked {
		return nil, fmt.Errorf("api key is revoked")
	}
	
	return s.GetUser(ctx, userID)
}

func (s *PostgresStore) CreateSession(ctx context.Context, record SessionRecord) error {
	_, err := s.db.ExecContext(ctx, "INSERT INTO sessions (id, user_id, backend, status, created_at) VALUES ($1, $2, $3, $4, $5)",
		record.ID, record.UserID, record.Backend, record.Status, record.CreatedAt)
	return err
}

func (s *PostgresStore) UpdateSessionStatus(ctx context.Context, sessionID, status string) error {
	if status == "completed" || status == "failed" {
		_, err := s.db.ExecContext(ctx, "UPDATE sessions SET status = $1, finished_at = $2 WHERE id = $3", status, time.Now().UTC(), sessionID)
		return err
	}
	_, err := s.db.ExecContext(ctx, "UPDATE sessions SET status = $1 WHERE id = $2", status, sessionID)
	return err
}

func (s *PostgresStore) Close() error {
	return s.db.Close()
}
