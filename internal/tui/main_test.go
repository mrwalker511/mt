package tui

import (
	"os"
	"testing"
)

// TestMain trusts every folder by default, so config tests can use a ./mt.yaml
// in a temporary folder. The trust tests switch back to the real check.
func TestMain(m *testing.M) {
	dirIsTrusted = func(string) bool { return true }
	os.Exit(m.Run())
}
