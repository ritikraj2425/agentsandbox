package firecracker

import (
	"os"
	"os/exec"
	"testing"
)

// firecrackerAvailable checks if the Firecracker binary and KVM are present.
func firecrackerAvailable() bool {
	if _, err := exec.LookPath("firecracker"); err != nil {
		return false
	}
	if _, err := os.Stat("/dev/kvm"); err != nil {
		return false
	}
	return true
}

func TestRuntime_Name(t *testing.T) {
	if !firecrackerAvailable() {
		t.Skip("Firecracker is not available (requires Linux with KVM), skipping")
	}

	rt, err := New(Config{
		WorkDir:    "/tmp",
		KernelPath: "/tmp/vmlinux",
		RootFSPath: "/tmp/rootfs.ext4",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rt.Name() != "firecracker" {
		t.Errorf("expected name 'firecracker', got %q", rt.Name())
	}
}

func TestNew_FirecrackerUnavailable(t *testing.T) {
	if firecrackerAvailable() {
		t.Skip("Firecracker is available, cannot test unavailable path")
	}

	_, err := New(Config{WorkDir: "/tmp"})
	if err == nil {
		t.Fatal("expected error when Firecracker is unavailable")
	}
}
