package llm

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const DefaultAppleBridgeName = "mt-apple-bridge"

func generateApple(ctx context.Context, cfg Config, prompt string) (string, error) {
	bridgePath := cfg.BridgePath
	if bridgePath == "" {
		if exe, err := os.Executable(); err == nil {
			cand := filepath.Join(filepath.Dir(exe), DefaultAppleBridgeName)
			if _, err := os.Stat(cand); err == nil {
				bridgePath = cand
			}
		}
	}
	if bridgePath == "" {
		var err error
		if bridgePath, err = exec.LookPath(DefaultAppleBridgeName); err != nil {
			return "", fmt.Errorf("apple: %s not found — run `make apple-bridge` then place the binary alongside mt or in your PATH", DefaultAppleBridgeName)
		}
	}

	cmd := exec.CommandContext(ctx, bridgePath)
	cmd.Stdin = strings.NewReader(prompt)
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf

	if err := cmd.Run(); err != nil {
		if errBuf.Len() > 0 {
			return "", fmt.Errorf("apple: %s", strings.TrimSpace(errBuf.String()))
		}
		return "", fmt.Errorf("apple: bridge exited: %w", err)
	}
	return strings.TrimSpace(out.String()), nil
}
