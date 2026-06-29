// Package store provides persistent storage for users, API keys, and session data.
package store

import (
	"context"
	"time"
)

// User represents a developer account in the system.
type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

// APIKey represents an authentication token for a user.
type APIKey struct {
	KeyHash   string    `json:"-"`
	UserID    string    `json:"user_id"`
	Name      string    `json:"name"`
	Revoked   bool      `json:"revoked"`
	CreatedAt time.Time `json:"created_at"`
}

// SessionRecord represents the metadata for a sandbox session.
type SessionRecord struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	Backend    string    `json:"backend"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	FinishedAt *time.Time `json:"finished_at"`
}

// Store defines the interface for data persistence.
type Store interface {
	// Init initializes the database schema.
	Init(ctx context.Context) error

	// Users
	CreateUser(ctx context.Context, email string) (*User, error)
	GetUser(ctx context.Context, id string) (*User, error)
	GetUserByEmail(ctx context.Context, email string) (*User, error)

	// API Keys
	GenerateAPIKey(ctx context.Context, userID, name string) (string, error)
	ValidateAPIKey(ctx context.Context, rawKey string) (*User, error)

	// Sessions
	CreateSession(ctx context.Context, record SessionRecord) error
	UpdateSessionStatus(ctx context.Context, sessionID, status string) error
	
	Close() error
}
