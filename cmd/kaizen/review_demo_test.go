package main

import (
	"context"
	"io"
	"testing"

	"github.com/alexcollie/kaizen/pkg/review"
	"github.com/alexcollie/kaizen/pkg/reviewui"
)

func TestDemoScriptReferencesFixture(t *testing.T) {
	reviewContext, cleanup, err := reviewui.LoadDemoContext()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	files := make(map[string]review.FileDiff)
	for _, file := range reviewContext.Files {
		files[file.Path] = file
	}
	steps := demoScript()
	if steps[len(steps)-1].event.Type != review.EventDone || steps[len(steps)-2].event.Type != review.EventResult {
		t.Fatal("script must end with a result followed by done")
	}

	findings := 0
	for _, step := range steps {
		finding, ok := step.event.Data.(review.Finding)
		if !ok {
			continue
		}
		findings++
		file, exists := files[finding.File]
		if !exists {
			t.Errorf("finding %s references unknown file %s", finding.ID, finding.File)
			continue
		}
		if finding.Line > file.LineCount {
			t.Errorf("finding %s line %d beyond %s (%d lines)", finding.ID, finding.Line, finding.File, file.LineCount)
		}
	}
	if findings < 8 {
		t.Errorf("expected a realistic number of streamed findings, got %d", findings)
	}
}

func TestReplayDemoScriptAssignsSequenceAndCloses(t *testing.T) {
	eventLog := reviewui.NewEventLog()
	steps := demoScript()
	for index := range steps {
		steps[index].delay = 0
	}
	replayDemoScript(context.Background(), eventLog, steps, 1)
	if !eventLog.Closed() || eventLog.LastSeq() != len(steps) {
		t.Fatalf("closed=%v last seq %d, want %d", eventLog.Closed(), eventLog.LastSeq(), len(steps))
	}
}

func TestDemoPosterClassifiesFindings(t *testing.T) {
	reviewContext, cleanup, err := reviewui.LoadDemoContext()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	poster := &demoPoster{output: io.Discard}
	request := review.PostRequest{Summary: "s", Findings: demoFindings()}
	result, err := poster.PostReview(context.Background(), reviewContext.PR, reviewContext.Files, request)
	if err != nil {
		t.Fatal(err)
	}
	// f9 is file-level and f10 sits outside the diff; the rest are on diff lines
	if result.BodyFindings != 2 || result.InlineComments != len(request.Findings)-2 || result.URL == "" {
		t.Errorf("unexpected classification: %+v", result)
	}
}
