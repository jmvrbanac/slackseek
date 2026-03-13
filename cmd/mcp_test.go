package cmd

import (
	"testing"
)

// TestMCPCommandRegistered verifies that addMCPCmd registers a "mcp" subcommand
// on the parent with a "serve" child.
func TestMCPCommandRegistered(t *testing.T) {
	root := NewRootCmd()
	addMCPCmd(root, func() error { return nil })

	mcpCmd, _, err := root.Find([]string{"mcp"})
	if err != nil || mcpCmd == nil || mcpCmd.Use != "mcp" {
		t.Fatalf("expected 'mcp' command, got cmd=%v err=%v", mcpCmd, err)
	}
}

// TestMCPServeCommandRegistered verifies the "serve" child of "mcp" is present.
func TestMCPServeCommandRegistered(t *testing.T) {
	root := NewRootCmd()
	addMCPCmd(root, func() error { return nil })

	serveCmd, _, err := root.Find([]string{"mcp", "serve"})
	if err != nil || serveCmd == nil || serveCmd.Use != "serve" {
		t.Fatalf("expected 'mcp serve' command, got cmd=%v err=%v", serveCmd, err)
	}
}

// TestMCPServeRunsServeFn verifies that executing "mcp serve" calls the
// injected serve function.
func TestMCPServeRunsServeFn(t *testing.T) {
	called := false
	root := NewRootCmd()
	addMCPCmd(root, func() error {
		called = true
		return nil
	})
	outBuf := new(noopWriter)
	root.SetOut(outBuf)
	root.SetErr(outBuf)
	root.SetArgs([]string{"mcp", "serve"})
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("expected serveFn to be called, it was not")
	}
}

// noopWriter discards all output.
type noopWriter struct{}

func (n *noopWriter) Write(p []byte) (int, error) { return len(p), nil }
