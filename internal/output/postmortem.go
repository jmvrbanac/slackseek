package output

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/jmvrbanac/slackseek/internal/slack"
)

// IncidentDoc is the structured postmortem document.
type IncidentDoc struct {
	Channel      string
	PeriodFrom   time.Time
	PeriodTo     time.Time
	Participants []string
	Timeline     []TimelineRow
}

// TimelineRow is one row in the incident timeline.
type TimelineRow struct {
	Time    time.Time
	Who     string
	Event   string
	Replies int // > 0 if this was a thread root with replies
}

// BuildIncidentDoc constructs an IncidentDoc from a slice of messages.
// Participants are sorted alphabetically and deduplicated.
// The Timeline contains root messages only, sorted chronologically.
func BuildIncidentDoc(messages []slack.Message, resolver *slack.Resolver) IncidentDoc {
	if len(messages) == 0 {
		return IncidentDoc{}
	}
	roots, replies := GroupByThread(messages)
	channel := messages[0].ChannelName
	if channel == "" && resolver != nil {
		channel = resolver.ChannelName(messages[0].ChannelID)
	}
	return IncidentDoc{
		Channel:      channel,
		PeriodFrom:   messages[0].Time,
		PeriodTo:     messages[len(messages)-1].Time,
		Participants: buildParticipantList(messages, resolver),
		Timeline:     buildTimeline(roots, replies, resolver),
	}
}

func buildParticipantList(messages []slack.Message, resolver *slack.Resolver) []string {
	seen := make(map[string]bool)
	var participants []string
	for _, m := range messages {
		name := m.UserID
		if resolver != nil {
			name = resolver.UserDisplayName(m.UserID)
		}
		if !seen[name] {
			seen[name] = true
			participants = append(participants, name)
		}
	}
	sort.Strings(participants)
	return participants
}

func buildTimeline(roots []slack.Message, replies map[string][]slack.Message, resolver *slack.Resolver) []TimelineRow {
	timeline := make([]TimelineRow, 0, len(roots))
	for _, m := range roots {
		_, _, text := resolveMessageFields(m, resolver)
		who := m.UserID
		if resolver != nil {
			who = resolver.UserDisplayName(m.UserID)
		}
		timeline = append(timeline, TimelineRow{
			Time:    m.Time,
			Who:     who,
			Event:   text,
			Replies: len(replies[m.Timestamp]),
		})
	}
	return timeline
}

// PrintPostmortem formats an IncidentDoc to w.
func PrintPostmortem(w io.Writer, fmt Format, doc IncidentDoc) error {
	switch fmt {
	case FormatJSON:
		return printPostmortemJSON(w, doc)
	default:
		return printPostmortemMarkdown(w, doc)
	}
}

func printPostmortemMarkdown(w io.Writer, doc IncidentDoc) error {
	fmt.Fprintf(w, "# Incident: %s\n\n", doc.Channel)
	fmt.Fprintf(w, "**Period:** %s UTC – %s UTC\n",
		doc.PeriodFrom.UTC().Format("2006-01-02 15:04"),
		doc.PeriodTo.UTC().Format("2006-01-02 15:04"),
	)
	fmt.Fprintf(w, "**Participants:** ")
	for i, p := range doc.Participants {
		if i > 0 {
			fmt.Fprintf(w, ", ")
		}
		fmt.Fprintf(w, "%s", p)
	}
	fmt.Fprintf(w, "\n\n## Timeline\n\n")
	fmt.Fprintf(w, "| Time (UTC)       | Who   | Event                                     |\n")
	fmt.Fprintf(w, "|------------------|-------|-------------------------------------------|\n")
	for _, row := range doc.Timeline {
		event := row.Event
		if len([]rune(event)) > 40 {
			event = string([]rune(event)[:40]) + "…"
		}
		if row.Replies > 0 {
			event = fmt.Sprintf("%s (%d replies)", event, row.Replies)
		}
		fmt.Fprintf(w, "| %s | %-5s | %-41s |\n",
			row.Time.UTC().Format("2006-01-02 15:04"),
			truncate(row.Who, 5),
			event,
		)
	}
	return nil
}

type incidentPeriodJSON struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type timelineRowJSON struct {
	Time    string `json:"time"`
	Who     string `json:"who"`
	Event   string `json:"event"`
	Replies int    `json:"replies"`
}

type incidentDocJSON struct {
	Channel      string              `json:"channel"`
	Period       incidentPeriodJSON  `json:"period"`
	Participants []string            `json:"participants"`
	Timeline     []timelineRowJSON   `json:"timeline"`
}

func printPostmortemJSON(w io.Writer, doc IncidentDoc) error {
	rows := make([]timelineRowJSON, len(doc.Timeline))
	for i, r := range doc.Timeline {
		rows[i] = timelineRowJSON{
			Time:    r.Time.UTC().Format(time.RFC3339),
			Who:     r.Who,
			Event:   r.Event,
			Replies: r.Replies,
		}
	}
	out := incidentDocJSON{
		Channel: doc.Channel,
		Period: incidentPeriodJSON{
			From: doc.PeriodFrom.UTC().Format(time.RFC3339),
			To:   doc.PeriodTo.UTC().Format(time.RFC3339),
		},
		Participants: doc.Participants,
		Timeline:     rows,
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
