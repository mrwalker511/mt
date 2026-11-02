package llm

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const DefaultAppleBridgeName = "mt-apple-bridge"

// findBridge returns the mt-apple-bridge to run: bridge_path if set, else the
// one next to the mt binary, else the first on PATH.
func findBridge(cfg Config) (string, error) {
	if cfg.BridgePath != "" {
		if _, err := os.Stat(cfg.BridgePath); err != nil {
			return "", fmt.Errorf("apple: bridge_path %s: %w", cfg.BridgePath, err)
		}
		return cfg.BridgePath, nil
	}
	if exe, err := os.Executable(); err == nil {
		cand := filepath.Join(filepath.Dir(exe), DefaultAppleBridgeName)
		if _, err := os.Stat(cand); err == nil {
			return cand, nil
		}
	}
	path, err := exec.LookPath(DefaultAppleBridgeName)
	if err != nil {
		return "", fmt.Errorf("apple: %s not found; run `make apple-bridge` then place the binary alongside mt or in your PATH", DefaultAppleBridgeName)
	}
	return path, nil
}

func generateApple(ctx context.Context, cfg Config, systemPrompt, userMessage string) (string, error) {
	prompt := systemPrompt + "\nUser: " + userMessage + "\nResponse:"
	bridgePath, err := findBridge(cfg)
	if err != nil {
		return "", err
	}

	cmd := exec.CommandContext(ctx, bridgePath)
	cmd.Stdin = strings.NewReader(prompt)
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	// After a cancel, don't wait for children of the bridge that still hold its pipes.
	cmd.WaitDelay = 2 * time.Second

	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		if errBuf.Len() > 0 {
			return "", fmt.Errorf("apple: %s", strings.TrimSpace(errBuf.String()))
		}
		return "", fmt.Errorf("apple: bridge exited: %w", err)
	}
	return strings.TrimSpace(out.String()), nil
}
