package tokens_test

import (
	"os"
	"testing"

	"github.com/jmvrbanac/slackseek/internal/tokens"
)

func TestExtractIntegration(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("set INTEGRATION=1 to run")
	}

	result, err := tokens.DefaultExtract()
	if err != nil {
		t.Fatalf("DefaultExtract() failed: %v", err)
	}

	if len(result.Workspaces) == 0 {
		t.Fatal("expected at least one workspace, got zero")
	}

	for _, ws := range result.Workspaces {
		if ws.Token == "" {
			t.Errorf("workspace %q has empty Token", ws.Name)
		}
	}
}
