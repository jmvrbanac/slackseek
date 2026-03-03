package slack_test

import (
	"context"
	"os"
	"testing"

	"github.com/jmvrbanac/slackseek/internal/slack"
)

func TestSearchIntegration(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("set INTEGRATION=1 to run")
	}

	token := os.Getenv("SLACK_TOKEN")
	if token == "" {
		t.Fatal("SLACK_TOKEN environment variable must be set for integration tests")
	}

	client := slack.NewClient(token, "", nil)
	results, err := client.SearchMessages(context.Background(), "test", 5)
	if err != nil {
		t.Fatalf("SearchMessages() failed: %v", err)
	}

	t.Logf("SearchMessages returned %d results", len(results))
}
