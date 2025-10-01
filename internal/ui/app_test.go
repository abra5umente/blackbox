package ui

import (
	"strings"
	"testing"
	"time"
)

func TestFormatRecordingTimestamp(t *testing.T) {
	ts := time.Date(2024, 8, 15, 9, 30, 45, 0, time.FixedZone("UTC+2", 2*60*60))
	if got, want := formatRecordingTimestamp(ts), ts.In(time.Local).Format("2006-01-02T15:04:05"); got != want {
		t.Fatalf("formatRecordingTimestamp mismatch: got %q want %q", got, want)
	}
}

func TestBuildRecordingDisplayName(t *testing.T) {
	ts := time.Date(2024, 8, 15, 9, 30, 45, 0, time.UTC)
	if got := buildRecordingDisplayName(" Weekly sync  ", &ts); !containsAll(got, []string{"Weekly sync", "2024"}) {
		t.Fatalf("unexpected display name: %q", got)
	}

	if got := buildRecordingDisplayName("", &ts); got != formatRecordingTimestamp(ts) {
		t.Fatalf("expected timestamp when title empty, got %q", got)
	}

	tooLong := strings.Repeat("あ", maxMeetingTitleRunes+10)
	truncated := buildRecordingDisplayName(tooLong, &ts)
	if runeCount := len([]rune(strings.Split(truncated, " - ")[0])); runeCount != maxMeetingTitleRunes {
		t.Fatalf("expected truncated title length %d, got %d", maxMeetingTitleRunes, runeCount)
	}
}

func TestAugmentPromptForStructuredSummary(t *testing.T) {
	base := "Summarise this meeting.\n"
	augmented := augmentPromptForStructuredSummary(base)
	if base == augmented {
		t.Fatal("expected augmented prompt to differ")
	}
	if !strings.Contains(augmented, "meeting_title") || !strings.Contains(augmented, "summary_markdown") {
		t.Fatalf("augmentPromptForStructuredSummary missing instructions: %q", augmented)
	}
}

func TestStripJSONCodeFence(t *testing.T) {
	cases := []struct {
		name string
		in   string
		out  string
	}{
		{"plain", `{"a":1}`, `{"a":1}`},
		{"fenced", "```json\n{\"a\":1}\n```", `{"a":1}`},
		{"fencedUpper", "```JSON\n{\"a\":1}\n```", `{"a":1}`},
	}

	for _, tc := range cases {
		if got := stripJSONCodeFence(tc.in); got != tc.out {
			t.Fatalf("%s: got %q want %q", tc.name, got, tc.out)
		}
	}
}

func TestParseSummaryPayload(t *testing.T) {
	raw := `{"meeting_title":"Weekly","summary_markdown":"Hi"}`
	payload, err := parseSummaryPayload(raw)
	if err != nil {
		t.Fatalf("parseSummaryPayload failed: %v", err)
	}
	if payload.MeetingTitle != "Weekly" || payload.SummaryMarkdown != "Hi" {
		t.Fatalf("unexpected payload: %+v", payload)
	}

	fenced := "```json\n" + raw + "\n```"
	payload, err = parseSummaryPayload(fenced)
	if err != nil {
		t.Fatalf("parseSummaryPayload fenced failed: %v", err)
	}

	if _, err := parseSummaryPayload("invalid"); err == nil {
		t.Fatal("expected error for invalid payload")
	}
}

func containsAll(haystack string, needles []string) bool {
	for _, n := range needles {
		if !strings.Contains(haystack, n) {
			return false
		}
	}
	return true
}
