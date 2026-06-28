package browser

import (
	"context"
	"os/exec"
	"testing"

	"github.com/ritikraj2425/agentsandbox/pkg/protocol"
)

func TestBrowserRuntime(t *testing.T) {
	if err := exec.Command("docker", "info").Run(); err != nil {
		t.Skip("Docker daemon is not running or accessible, skipping browser runtime tests")
	}

	rt, err := New(Config{})
	if err != nil {
		t.Fatalf("Failed to create browser runtime: %v", err)
	}

	ctx := context.Background()

	// Ensure cleanup
	defer rt.Stop()

	// 1. Test starting the container
	err = rt.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start browser container: %v", err)
	}

	if rt.containerID == "" {
		t.Fatal("Expected container ID to be set")
	}
	if rt.cdpClient == nil {
		t.Fatal("Expected CDP client to be initialized")
	}

	// 2. Test navigation
	gotoAction := protocol.NewAction(protocol.ActionTypeBrowserGoto, map[string]interface{}{
		"url": "about:blank",
	})
	obs, err := rt.Run(ctx, gotoAction)
	if err != nil {
		t.Fatalf("Failed to execute goto action: %v", err)
	}
	if obs.Status != protocol.ObsStatusCompleted {
		t.Fatalf("Expected status completed, got %v", obs.Status)
	}
	if obs.PageURL != "about:blank" {
		t.Errorf("Expected URL about:blank, got %s", obs.PageURL)
	}

	// 3. Test screenshot
	screenshotAction := protocol.NewAction(protocol.ActionTypeBrowserScreenshot, nil)
	obs, err = rt.Run(ctx, screenshotAction)
	if err != nil {
		t.Fatalf("Failed to execute screenshot action: %v", err)
	}
	if obs.Status != protocol.ObsStatusCompleted {
		t.Fatalf("Expected status completed, got %v", obs.Status)
	}
	if obs.Screenshot == "" {
		t.Error("Expected screenshot data, got empty string")
	}

	// 4. Test clicking (using coordinates to avoid needing an element)
	clickAction := protocol.NewAction(protocol.ActionTypeBrowserClick, map[string]interface{}{
		"x": 100,
		"y": 100,
	})
	obs, err = rt.Run(ctx, clickAction)
	if err != nil {
		t.Fatalf("Failed to execute click action: %v", err)
	}
	if obs.Status != protocol.ObsStatusCompleted {
		t.Fatalf("Expected status completed, got %v", obs.Status)
	}

	// 5. Test stop
	rt.Stop()
	if rt.cdpClient != nil {
		t.Error("Expected CDP client to be nil after stop")
	}
	
	// Verify container is gone
	err = exec.Command("docker", "inspect", rt.containerID).Run()
	if err == nil {
		t.Error("Expected container to be removed after stop")
	}
}
