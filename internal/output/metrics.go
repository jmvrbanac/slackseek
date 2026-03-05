package output

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/jmvrbanac/slackseek/internal/slack"
)

// ChannelMetrics is the aggregated metrics for a channel.
type ChannelMetrics struct {
	UserCounts    []UserCount     // sorted descending by Count
	ThreadCount   int
	AvgReplyDepth float64
	TopReactions  []ReactionCount // top 5 sorted descending
	HourlyDist    [24]int         // UTC hour → message count
}

// UserCount holds a user's message count.
type UserCount struct {
	DisplayName string
	Count       int
}

// ReactionCount holds a reaction's total usage count.
type ReactionCount struct {
	Name  string
	Total int
}

// ComputeMetrics aggregates messages into ChannelMetrics.
func ComputeMetrics(messages []slack.Message, resolver *slack.Resolver) ChannelMetrics {
	userCounts, reactionTotals, threadCount, totalReplies := accumulateMessages(messages, resolver)
	uc := sortedUserCounts(userCounts)
	rc := topReactions(reactionTotals, 5)
	var hourly [24]int
	for _, m := range messages {
		hourly[m.Time.UTC().Hour()]++
	}
	avgReply := 0.0
	if threadCount > 0 {
		avgReply = float64(totalReplies) / float64(threadCount)
	}
	return ChannelMetrics{
		UserCounts:    uc,
		ThreadCount:   threadCount,
		AvgReplyDepth: avgReply,
		TopReactions:  rc,
		HourlyDist:    hourly,
	}
}

func accumulateMessages(messages []slack.Message, resolver *slack.Resolver) (
	userCounts map[string]int, reactionTotals map[string]int, threadCount, totalReplies int,
) {
	userCounts = make(map[string]int)
	reactionTotals = make(map[string]int)
	for _, m := range messages {
		name := m.UserID
		if resolver != nil {
			name = resolver.UserDisplayName(m.UserID)
		}
		userCounts[name]++
		if m.ThreadTS == "" || m.ThreadTS == m.Timestamp {
			if m.ThreadDepth == 0 {
				threadCount++
			}
		} else {
			totalReplies++
		}
		for _, r := range m.Reactions {
			reactionTotals[r.Name] += r.Count
		}
	}
	return
}

func sortedUserCounts(m map[string]int) []UserCount {
	uc := make([]UserCount, 0, len(m))
	for name, cnt := range m {
		uc = append(uc, UserCount{DisplayName: name, Count: cnt})
	}
	sort.Slice(uc, func(i, j int) bool {
		if uc[i].Count != uc[j].Count {
			return uc[i].Count > uc[j].Count
		}
		return uc[i].DisplayName < uc[j].DisplayName
	})
	return uc
}

func topReactions(m map[string]int, limit int) []ReactionCount {
	rc := make([]ReactionCount, 0, len(m))
	for name, total := range m {
		rc = append(rc, ReactionCount{Name: name, Total: total})
	}
	sort.Slice(rc, func(i, j int) bool {
		if rc[i].Total != rc[j].Total {
			return rc[i].Total > rc[j].Total
		}
		return rc[i].Name < rc[j].Name
	})
	if len(rc) > limit {
		rc = rc[:limit]
	}
	return rc
}

// PrintMetrics formats ChannelMetrics to w.
func PrintMetrics(w io.Writer, fmt Format, m ChannelMetrics) error {
	switch fmt {
	case FormatJSON:
		return printMetricsJSON(w, m)
	default:
		return printMetricsText(w, m)
	}
}

func printMetricsText(w io.Writer, m ChannelMetrics) error {
	fmt.Fprintln(w, "=== Message counts ===")
	for _, uc := range m.UserCounts {
		fmt.Fprintf(w, "%-20s %d\n", uc.DisplayName, uc.Count)
	}

	fmt.Fprintf(w, "\n=== Thread stats ===\n")
	fmt.Fprintf(w, "Thread count: %d  Average replies: %.1f\n", m.ThreadCount, m.AvgReplyDepth)

	fmt.Fprintf(w, "\n=== Top reactions ===\n")
	parts := make([]string, len(m.TopReactions))
	for i, r := range m.TopReactions {
		parts[i] = fmt.Sprintf("%s×%d", r.Name, r.Total)
	}
	fmt.Fprintln(w, strings.Join(parts, "  "))

	fmt.Fprintf(w, "\n=== Messages by hour (UTC) ===\n")
	maxCount := 0
	for _, cnt := range m.HourlyDist {
		if cnt > maxCount {
			maxCount = cnt
		}
	}
	barWidth := 20
	for h, cnt := range m.HourlyDist {
		bar := ""
		if maxCount > 0 && cnt > 0 {
			barLen := cnt * barWidth / maxCount
			if barLen < 1 {
				barLen = 1
			}
			bar = strings.Repeat("█", barLen)
		}
		if cnt > 0 {
			fmt.Fprintf(w, "%02d |%-*s %d\n", h, barWidth, bar, cnt)
		} else {
			fmt.Fprintf(w, "%02d |\n", h)
		}
	}
	return nil
}

type metricsUserJSON struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type metricsThreadsJSON struct {
	Count         int     `json:"count"`
	AvgReplyDepth float64 `json:"avg_reply_depth"`
}

type metricsReactionJSON struct {
	Name  string `json:"name"`
	Total int    `json:"total"`
}

type metricsHourlyJSON struct {
	Hour  int `json:"hour"`
	Count int `json:"count"`
}

type metricsOutputJSON struct {
	Users        []metricsUserJSON     `json:"users"`
	Threads      metricsThreadsJSON    `json:"threads"`
	TopReactions []metricsReactionJSON `json:"top_reactions"`
	Hourly       []metricsHourlyJSON   `json:"hourly"`
}

func printMetricsJSON(w io.Writer, m ChannelMetrics) error {
	users := make([]metricsUserJSON, len(m.UserCounts))
	for i, uc := range m.UserCounts {
		users[i] = metricsUserJSON{Name: uc.DisplayName, Count: uc.Count}
	}
	reactions := make([]metricsReactionJSON, len(m.TopReactions))
	for i, r := range m.TopReactions {
		reactions[i] = metricsReactionJSON(r)
	}
	var hourly []metricsHourlyJSON
	for h, cnt := range m.HourlyDist {
		if cnt > 0 {
			hourly = append(hourly, metricsHourlyJSON{Hour: h, Count: cnt})
		}
	}
	out := metricsOutputJSON{
		Users:        users,
		Threads:      metricsThreadsJSON{Count: m.ThreadCount, AvgReplyDepth: m.AvgReplyDepth},
		TopReactions: reactions,
		Hourly:       hourly,
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
