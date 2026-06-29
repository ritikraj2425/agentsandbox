// Package workspace manages per-session filesystem isolation for API sessions.
package workspace

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	InitEmpty           = "empty"
	InitGitClone        = "git_clone"
	InitUploadedArchive = "uploaded_archive"
)

type Config struct {
	BaseDir   string
	Retention time.Duration
}

type Manager struct {
	baseDir   string
	retention time.Duration
}

type InitSpec struct {
	Type      string `json:"type,omitempty"`
	GitURL    string `json:"git_url,omitempty"`
	ArchiveID string `json:"archive_id,omitempty"`
}

type Paths struct {
	SessionID    string    `json:"session_id"`
	RootDir      string    `json:"root_dir"`
	WorkspaceDir string    `json:"workspace_dir"`
	ArtifactsDir string    `json:"artifacts_dir"`
	TracesDir    string    `json:"traces_dir"`
	TmpDir       string    `json:"tmp_dir"`
	ExpiresAt    time.Time `json:"expires_at"`
	RetainUntil  time.Time `json:"retain_until"`
}

func NewManager(cfg Config) (*Manager, error) {
	if cfg.BaseDir == "" {
		return nil, fmt.Errorf("workspace base dir is required")
	}
	base, err := filepath.Abs(cfg.BaseDir)
	if err != nil {
		return nil, fmt.Errorf("resolve workspace base dir: %w", err)
	}
	if err := os.MkdirAll(base, 0755); err != nil {
		return nil, fmt.Errorf("create workspace base dir: %w", err)
	}
	return &Manager{baseDir: base, retention: cfg.Retention}, nil
}

func (m *Manager) Create(ttl time.Duration, init InitSpec) (*Paths, error) {
	if ttl <= 0 {
		ttl = time.Hour
	}
	id := generateID()
	now := time.Now()
	paths := &Paths{
		SessionID:    id,
		RootDir:      filepath.Join(m.baseDir, id),
		WorkspaceDir: filepath.Join(m.baseDir, id, "workspace"),
		ArtifactsDir: filepath.Join(m.baseDir, id, "artifacts"),
		TracesDir:    filepath.Join(m.baseDir, id, "traces"),
		TmpDir:       filepath.Join(m.baseDir, id, "tmp"),
		ExpiresAt:    now.Add(ttl),
		RetainUntil:  now.Add(ttl).Add(m.retention),
	}

	for _, dir := range []string{paths.WorkspaceDir, paths.ArtifactsDir, paths.TracesDir, paths.TmpDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			_ = os.RemoveAll(paths.RootDir)
			return nil, fmt.Errorf("create workspace dir %s: %w", dir, err)
		}
	}

	if err := m.initialize(paths, init); err != nil {
		_ = os.RemoveAll(paths.RootDir)
		return nil, err
	}
	if err := writeMetadata(paths); err != nil {
		_ = os.RemoveAll(paths.RootDir)
		return nil, err
	}
	return paths, nil
}

func (m *Manager) GuardPath(paths *Paths, requested string) (string, error) {
	return GuardPath(paths.WorkspaceDir, requested)
}

func GuardPath(workspaceDir string, requested string) (string, error) {
	if strings.TrimSpace(requested) == "" {
		return "", fmt.Errorf("path is required")
	}
	workspaceAbs, err := filepath.Abs(workspaceDir)
	if err != nil {
		return "", fmt.Errorf("resolve workspace path: %w", err)
	}
	var final string
	if filepath.IsAbs(requested) {
		final = filepath.Clean(requested)
	} else {
		final = filepath.Clean(filepath.Join(workspaceAbs, requested))
	}
	rel, err := filepath.Rel(workspaceAbs, final)
	if err != nil {
		return "", fmt.Errorf("resolve requested path: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("path escapes workspace")
	}
	return final, nil
}

func (m *Manager) CleanupExpired(now time.Time) error {
	entries, err := os.ReadDir(m.baseDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		root := filepath.Join(m.baseDir, entry.Name())
		paths, err := readMetadata(root)
		if err != nil {
			continue
		}
		if !now.Before(paths.RetainUntil) {
			if err := os.RemoveAll(root); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *Manager) initialize(paths *Paths, init InitSpec) error {
	switch init.Type {
	case "", InitEmpty:
		return nil
	case InitGitClone:
		if strings.TrimSpace(init.GitURL) == "" {
			return fmt.Errorf("git clone initialization requires git_url")
		}
		cmd := exec.Command("git", "clone", "--depth", "1", init.GitURL, paths.WorkspaceDir)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git clone failed: %w: %s", err, string(out))
		}
		return nil
	case InitUploadedArchive:
		placeholder := filepath.Join(paths.ArtifactsDir, "uploaded-archive-placeholder.txt")
		message := fmt.Sprintf("uploaded archive initialization is not implemented yet\narchive_id=%s\n", init.ArchiveID)
		return os.WriteFile(placeholder, []byte(message), 0644)
	default:
		return fmt.Errorf("unsupported workspace initialization type: %s", init.Type)
	}
}

func writeMetadata(paths *Paths) error {
	data, err := json.MarshalIndent(paths, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(paths.RootDir, "metadata.json"), append(data, '\n'), 0644)
}

func readMetadata(root string) (*Paths, error) {
	data, err := os.ReadFile(filepath.Join(root, "metadata.json"))
	if err != nil {
		return nil, err
	}
	var paths Paths
	if err := json.Unmarshal(data, &paths); err != nil {
		return nil, err
	}
	return &paths, nil
}

func generateID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
