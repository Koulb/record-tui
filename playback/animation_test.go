package playback

import (
	"strings"
	"testing"
)

func TestBuildAnimationFrames_UsesOutputTiming(t *testing.T) {
	frames, err := BuildAnimationFrames(
		strings.NewReader("O 0.100000 3\nO 0.200000 2\n"),
		[]byte("hello"),
		AnimationBuildOptions{MaxFPS: 0},
	)
	if err != nil {
		t.Fatalf("BuildAnimationFrames failed: %v", err)
	}

	if len(frames) != 2 {
		t.Fatalf("expected 2 frames, got %d", len(frames))
	}
	if frames[0].Timestamp != 0.1 || frames[0].Content != "hel" {
		t.Fatalf("first frame = (%v, %q), want (0.1, %q)", frames[0].Timestamp, frames[0].Content, "hel")
	}
	if frames[1].Timestamp != 0.3 || frames[1].Content != "lo" {
		t.Fatalf("second frame = (%v, %q), want (0.3, %q)", frames[1].Timestamp, frames[1].Content, "lo")
	}
}

func TestBuildAnimationFrames_StripsSessionMetadata(t *testing.T) {
	session := "Script started on Wed Dec 31 12:10:34 2025\n" +
		"Command: bash\n" +
		"hello\n" +
		"Script done on Wed Dec 31 12:11:22 2025\n"

	frames, err := BuildAnimationFrames(
		strings.NewReader("O 0.100000 2\nO 0.200000 4\n"),
		[]byte(session),
		AnimationBuildOptions{MaxFPS: 0},
	)
	if err != nil {
		t.Fatalf("BuildAnimationFrames failed: %v", err)
	}

	got := frames[0].Content + frames[1].Content
	if got != "hello" {
		t.Fatalf("combined frame content = %q, want %q", got, "hello")
	}
}

func TestBuildAnimationFrames_CoalescesFramesByMaxFPS(t *testing.T) {
	frames, err := BuildAnimationFrames(
		strings.NewReader("O 0.001000 1\nO 0.001000 1\nO 0.100000 1\n"),
		[]byte("abc"),
		AnimationBuildOptions{MaxFPS: 30},
	)
	if err != nil {
		t.Fatalf("BuildAnimationFrames failed: %v", err)
	}

	if len(frames) != 2 {
		t.Fatalf("expected 2 coalesced frames, got %d", len(frames))
	}
	if frames[0].Content != "ab" {
		t.Fatalf("first coalesced frame content = %q, want %q", frames[0].Content, "ab")
	}
	if frames[1].Content != "c" {
		t.Fatalf("final frame content = %q, want %q", frames[1].Content, "c")
	}
}

func TestBuildAnimationMarkers_ExtractsCommandTimestamps(t *testing.T) {
	markers, err := BuildAnimationMarkers(
		strings.NewReader("O 0.100000 5\nI 0.500000 3\nO 0.010000 3\n"),
		[]byte("ls\r"),
	)
	if err != nil {
		t.Fatalf("BuildAnimationMarkers failed: %v", err)
	}

	if len(markers) != 1 {
		t.Fatalf("expected 1 marker, got %d", len(markers))
	}
	if markers[0].Label != "ls" {
		t.Fatalf("marker label = %q, want %q", markers[0].Label, "ls")
	}
	if markers[0].Timestamp != 0.6 {
		t.Fatalf("marker timestamp = %v, want 0.6", markers[0].Timestamp)
	}
}

func TestRenderAnimatedHTML_Basic(t *testing.T) {
	html, err := RenderAnimatedHTML(
		[]Frame{{Timestamp: 0.1, Content: "hello"}},
		AnimationOptions{
			Title: "Animated Session",
			Markers: []AnimationMarker{
				{Label: "ls", Timestamp: 0.1},
			},
			Cues: []AnimationCue{
				{Label: "Pause here", Timestamp: 0.1, Pause: 1},
			},
		},
	)
	if err != nil {
		t.Fatalf("RenderAnimatedHTML failed: %v", err)
	}

	for _, want := range []string{
		"<title>Animated Session</title>",
		"framesBase64",
		"animationMarkers",
		"id=\"animation-play\"",
		"id=\"animation-speed\"",
		"id=\"animation-timeline\"",
		"ls",
		"Pause here",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("animated HTML should contain %q", want)
		}
	}
}
