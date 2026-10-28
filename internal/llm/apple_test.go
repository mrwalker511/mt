package llm

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeBridge writes a shell script standing in for mt-apple-bridge and
// returns its path.
func fakeBridge(t *testing.T, script string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), DefaultAppleBridgeName)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestApple_SendsPromptAndReturnsReply(t *testing.T) {
	// The fake bridge echoes the prompt it received on stdin.
	bridge := fakeBridge(t, `printf "INFO: "; cat`)
	got, err := Generate(context.Background(), Config{Provider: "apple", BridgePath: bridge}, "SYSTEM", "open outlook")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"INFO:", "SYSTEM", "User: open outlook", "Response:"} {
		if !strings.Contains(got, want) {
			t.Errorf("reply %q missing %q", got, want)
		}
	}
}

func TestApple_BridgeErrorUsesStderr(t *testing.T) {
	bridge := fakeBridge(t, `echo "error: FoundationModels is not available" >&2; exit 1`)
	_, err := Generate(context.Background(), Config{Provider: "apple", BridgePath: bridge}, "s", "u")
	if err == nil || !strings.Contains(err.Error(), "apple: error: FoundationModels is not available") {
		t.Errorf("err = %v", err)
	}
}

func TestApple_BridgeErrorWithoutStderr(t *testing.T) {
	bridge := fakeBridge(t, `exit 3`)
	_, err := Generate(context.Background(), Config{Provider: "apple", BridgePath: bridge}, "s", "u")
	if err == nil || !strings.Contains(err.Error(), "bridge exited") {
		t.Errorf("err = %v", err)
	}
}

func TestApple_FoundOnPATH(t *testing.T) {
	bridge := fakeBridge(t, `echo "INFO: from PATH"`)
	t.Setenv("PATH", filepath.Dir(bridge))
	got, err := Generate(context.Background(), Config{Provider: "apple"}, "s", "u")
	if err != nil || got != "INFO: from PATH" {
		t.Errorf("got %q, %v", got, err)
	}
}

func TestApple_NotFound(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	_, err := Generate(context.Background(), Config{Provider: "apple"}, "s", "u")
	if err == nil || !strings.Contains(err.Error(), "make apple-bridge") {
		t.Errorf("err = %v", err)
	}
}

func TestApple_CancelReturnsContextError(t *testing.T) {
	bridge := fakeBridge(t, `sleep 30`)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := Generate(ctx, Config{Provider: "apple", BridgePath: bridge}, "s", "u")
	if err != context.DeadlineExceeded || time.Since(start) > 5*time.Second {
		t.Errorf("err = %v after %s", err, time.Since(start))
	}
}
