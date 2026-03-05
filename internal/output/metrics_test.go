package output

import (
	"strings"
	"testing"
	"time"

	"github.com/jmvrbanac/slackseek/internal/slack"
)

// T040: metrics unit tests

func makeMetricsMessages() []slack.Message {
	t1 := time.Date(2026, 2, 25, 14, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 2, 25, 14, 5, 0, 0, time.UTC)
	t3 := time.Date(2026, 2, 25, 15, 0, 0, 0, time.UTC)
	return []slack.Message{
		{
			Timestamp: "100.000000", Time: t1, UserID: "alice",
			Reactions: []slack.Reaction{{Name: "thumbsup", Count: 3}, {Name: "fire", Count: 2}},
		},
		{
			Timestamp: "101.000000", Time: t1, UserID: "alice",
		},
		{
			Timestamp: "102.000000", Time: t2, UserID: "bob",
			Reactions: []slack.Reaction{{Name: "thumbsup", Count: 5}},
		},
		{
			Timestamp: "103.000000", Time: t3, UserID: "carol",
			ThreadTS: "100.000000", ThreadDepth: 1,
		},
	}
}

func TestComputeMetrics_UserCountsSortedDescending(t *testing.T) {
	msgs := makeMetricsMessages()
	m := ComputeMetrics(msgs, nil)

	if len(m.UserCounts) == 0 {
		t.Fatal("expected user counts")
	}
	for i := 1; i < len(m.UserCounts); i++ {
		if m.UserCounts[i].Count > m.UserCounts[i-1].Count {
			t.Errorf("user counts not sorted descending at index %d", i)
		}
	}
	// alice has 2 messages
	if m.UserCounts[0].DisplayName != "alice" || m.UserCounts[0].Count != 2 {
		t.Errorf("expected alice with 2 messages first, got %+v", m.UserCounts[0])
	}
}

func TestComputeMetrics_TopReactionsTop5(t *testing.T) {
	t1 := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	msgs := []slack.Message{}
	for i := 0; i < 10; i++ {
		msgs = append(msgs, slack.Message{
			Timestamp: "1.000000",
			Time:      t1,
			UserID:    "u",
			Reactions: []slack.Reaction{
				{Name: strings.Repeat("r", i+1), Count: 10 - i},
			},
		})
	}
	m := ComputeMetrics(msgs, nil)
	if len(m.TopReactions) > 5 {
		t.Errorf("expected at most 5 top reactions, got %d", len(m.TopReactions))
	}
}

func TestComputeMetrics_HourlyDistribution(t *testing.T) {
	t14 := time.Date(2026, 2, 25, 14, 0, 0, 0, time.UTC)
	t15 := time.Date(2026, 2, 25, 15, 0, 0, 0, time.UTC)
	msgs := []slack.Message{
		{Timestamp: "1.0", Time: t14, UserID: "u"},
		{Timestamp: "2.0", Time: t14, UserID: "u"},
		{Timestamp: "3.0", Time: t15, UserID: "u"},
	}
	m := ComputeMetrics(msgs, nil)
	if m.HourlyDist[14] != 2 {
		t.Errorf("expected 2 messages at hour 14, got %d", m.HourlyDist[14])
	}
	if m.HourlyDist[15] != 1 {
		t.Errorf("expected 1 message at hour 15, got %d", m.HourlyDist[15])
	}
}

func TestPrintMetrics_TextContainsSections(t *testing.T) {
	msgs := makeMetricsMessages()
	m := ComputeMetrics(msgs, nil)

	var sb strings.Builder
	if err := PrintMetrics(&sb, FormatText, m); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := sb.String()

	for _, section := range []string{"Message counts", "Thread stats", "Top reactions", "Messages by hour"} {
		if !strings.Contains(out, section) {
			t.Errorf("expected section %q in metrics text output, got:\n%s", section, out)
		}
	}
}

func TestPrintMetrics_JSONContainsRequiredKeys(t *testing.T) {
	msgs := makeMetricsMessages()
	m := ComputeMetrics(msgs, nil)

	var sb strings.Builder
	if err := PrintMetrics(&sb, FormatJSON, m); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := sb.String()

	for _, key := range []string{`"users"`, `"threads"`, `"top_reactions"`, `"hourly"`} {
		if !strings.Contains(out, key) {
			t.Errorf("expected JSON key %q in metrics output, got:\n%s", key, out)
		}
	}
}
