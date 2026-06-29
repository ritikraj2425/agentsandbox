package workspace

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestManagerCreateCreatesSessionDirectories(t *testing.T) {
	mgr, err := NewManager(Config{BaseDir: t.TempDir(), Retention: time.Hour})
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	paths, err := mgr.Create(time.Minute, InitSpec{Type: InitEmpty})
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	for _, dir := range []string{paths.RootDir, paths.WorkspaceDir, paths.ArtifactsDir, paths.TracesDir, paths.TmpDir} {
		info, err := os.Stat(dir)
		if err != nil {
			t.Fatalf("expected dir %s: %v", dir, err)
		}
		if !info.IsDir() {
			t.Fatalf("expected %s to be a directory", dir)
		}
	}
	if _, err := os.Stat(filepath.Join(paths.RootDir, "metadata.json")); err != nil {
		t.Fatalf("expected metadata.json: %v", err)
	}
}

func TestGuardPathBlocksTraversal(t *testing.T) {
	workspace := t.TempDir()
	if _, err := GuardPath(workspace, "../secret.txt"); err == nil {
		t.Fatal("expected traversal to be blocked")
	}
}

func TestGuardPathAllowsWorkspacePath(t *testing.T) {
	workspace := t.TempDir()
	got, err := GuardPath(workspace, "dir/file.txt")
	if err != nil {
		t.Fatalf("guard path: %v", err)
	}
	want := filepath.Join(workspace, "dir", "file.txt")
	if got != want {
		t.Fatalf("expected %s, got %s", want, got)
	}
}

func TestCleanupExpiredRespectsRetention(t *testing.T) {
	mgr, err := NewManager(Config{BaseDir: t.TempDir(), Retention: time.Hour})
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	paths, err := mgr.Create(time.Minute, InitSpec{})
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	if err := mgr.CleanupExpired(paths.ExpiresAt.Add(30 * time.Minute)); err != nil {
		t.Fatalf("cleanup before retention: %v", err)
	}
	if _, err := os.Stat(paths.RootDir); err != nil {
		t.Fatalf("workspace should be retained: %v", err)
	}

	if err := mgr.CleanupExpired(paths.RetainUntil.Add(time.Second)); err != nil {
		t.Fatalf("cleanup after retention: %v", err)
	}
	if _, err := os.Stat(paths.RootDir); !os.IsNotExist(err) {
		t.Fatalf("workspace should be removed after retention, err=%v", err)
	}
}
