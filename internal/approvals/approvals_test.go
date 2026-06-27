package approvals

import (
	"bytes"
	"testing"
)

func TestAsk_AutoApprove(t *testing.T) {
	cfg := Config{AutoApprove: true}
	req := Request{Command: "npm install", Reason: "test"}

	if !Ask(req, cfg) {
		t.Error("expected true when AutoApprove is true")
	}
}

func TestAsk_NonInteractive(t *testing.T) {
	cfg := Config{NonInteractive: true}
	req := Request{Command: "npm install", Reason: "test"}

	if Ask(req, cfg) {
		t.Error("expected false when NonInteractive is true")
	}
}

func TestAsk_InteractiveYes(t *testing.T) {
	var out bytes.Buffer
	in := bytes.NewBufferString("y\n")

	cfg := Config{
		Reader: in,
		Writer: &out,
	}
	req := Request{Command: "npm install", Reason: "test"}

	if !Ask(req, cfg) {
		t.Error("expected true when input is 'y'")
	}
}

func TestAsk_InteractiveYesFull(t *testing.T) {
	var out bytes.Buffer
	in := bytes.NewBufferString("yes\n")

	cfg := Config{
		Reader: in,
		Writer: &out,
	}
	req := Request{Command: "npm install", Reason: "test"}

	if !Ask(req, cfg) {
		t.Error("expected true when input is 'yes'")
	}
}

func TestAsk_InteractiveNo(t *testing.T) {
	var out bytes.Buffer
	in := bytes.NewBufferString("n\n")

	cfg := Config{
		Reader: in,
		Writer: &out,
	}
	req := Request{Command: "npm install", Reason: "test"}

	if Ask(req, cfg) {
		t.Error("expected false when input is 'n'")
	}
}

func TestAsk_InteractiveEmpty(t *testing.T) {
	var out bytes.Buffer
	in := bytes.NewBufferString("\n")

	cfg := Config{
		Reader: in,
		Writer: &out,
	}
	req := Request{Command: "npm install", Reason: "test"}

	if Ask(req, cfg) {
		t.Error("expected false when input is empty string")
	}
}
