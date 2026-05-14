package html

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func TestRenderAnimatedPlaybackHTML_EmbedsFramesAndControls(t *testing.T) {
	frames := []PlaybackFrame{
		{Timestamp: 0.1, Content: "hello"},
		{Timestamp: 0.3, Content: " world"},
	}
	markers := []AnimationMarker{
		{Label: "echo hello", Timestamp: 0.1},
	}
	cues := []AnimationCue{
		{Label: "Slow failure", Timestamp: 0.2, Speed: 0.5, Pause: 2},
	}

	html, err := RenderAnimatedPlaybackHTML(frames, "Animated", FooterLink{}, markers, cues)
	if err != nil {
		t.Fatalf("RenderAnimatedPlaybackHTML failed: %v", err)
	}

	for _, want := range []string{
		"framesBase64",
		"animationMarkers",
		"animationCues",
		"id=\"animation-play\"",
		"id=\"animation-speed\"",
		"id=\"animation-timeline\"",
		"id=\"animation-time\"",
		"function playFrom",
		"echo hello",
		"Slow failure",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("animated HTML should contain %q", want)
		}
	}

	startMarker := "const framesBase64 = '"
	startIdx := strings.Index(html, startMarker)
	if startIdx == -1 {
		t.Fatalf("could not find framesBase64")
	}
	startIdx += len(startMarker)
	endIdx := strings.Index(html[startIdx:], "'")
	if endIdx == -1 {
		t.Fatalf("could not find framesBase64 terminator")
	}

	decoded, err := base64.StdEncoding.DecodeString(html[startIdx : startIdx+endIdx])
	if err != nil {
		t.Fatalf("could not decode frames: %v", err)
	}
	var decodedFrames []PlaybackFrame
	if err := json.Unmarshal(decoded, &decodedFrames); err != nil {
		t.Fatalf("could not unmarshal frames: %v", err)
	}
	if len(decodedFrames) != 2 {
		t.Fatalf("expected 2 frames, got %d", len(decodedFrames))
	}
	if decodedFrames[0].Content != "hello" {
		t.Fatalf("first frame content = %q, want %q", decodedFrames[0].Content, "hello")
	}
}

func TestRenderAnimatedPlaybackHTML_EscapesTitleAndMarkers(t *testing.T) {
	html, err := RenderAnimatedPlaybackHTML(
		[]PlaybackFrame{{Timestamp: 0.1, Content: "hello"}},
		"<script>alert(1)</script>",
		FooterLink{},
		[]AnimationMarker{{Label: "<img src=x>", Timestamp: 0.1}},
		nil,
	)
	if err != nil {
		t.Fatalf("RenderAnimatedPlaybackHTML failed: %v", err)
	}

	if strings.Contains(html, "<script>alert") {
		t.Fatalf("title should be escaped")
	}
	if strings.Contains(html, "<img src=x>") {
		t.Fatalf("marker label should be JSON-encoded, not emitted as raw HTML")
	}
	if !strings.Contains(html, "&lt;script&gt;") {
		t.Fatalf("escaped title missing")
	}
}

func TestRenderAnimatedPlaybackHTML_EncodesNilMarkersAndCuesAsArrays(t *testing.T) {
	html, err := RenderAnimatedPlaybackHTML(
		[]PlaybackFrame{{Timestamp: 0.1, Content: "hello"}},
		"Animated",
		FooterLink{},
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("RenderAnimatedPlaybackHTML failed: %v", err)
	}

	for _, constName := range []string{"animationMarkersBase64", "animationCuesBase64"} {
		encoded := extractBase64Const(t, html, constName)
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			t.Fatalf("could not decode %s: %v", constName, err)
		}
		if string(decoded) != "[]" {
			t.Fatalf("%s decoded to %q, want []", constName, string(decoded))
		}
	}
}

func extractBase64Const(t *testing.T, html string, constName string) string {
	t.Helper()
	startMarker := "const " + constName + " = '"
	startIdx := strings.Index(html, startMarker)
	if startIdx == -1 {
		t.Fatalf("could not find %s", constName)
	}
	startIdx += len(startMarker)
	endIdx := strings.Index(html[startIdx:], "'")
	if endIdx == -1 {
		t.Fatalf("could not find %s terminator", constName)
	}
	return html[startIdx : startIdx+endIdx]
}
