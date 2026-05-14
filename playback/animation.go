package playback

import (
	"io"
	"math"

	"github.com/choonkeat/record-tui/internal/html"
	"github.com/choonkeat/record-tui/internal/session"
	"github.com/choonkeat/record-tui/internal/timing"
)

const defaultAnimationFPS = 30.0

// BuildAnimationFrames converts script timing data plus session output into
// timed output chunks suitable for RenderAnimatedHTML.
func BuildAnimationFrames(timingReader io.Reader, sessionContent []byte, opts ...AnimationBuildOptions) ([]Frame, error) {
	entries, err := timing.Parse(timingReader)
	if err != nil {
		return nil, err
	}

	maxFPS := defaultAnimationFPS
	if len(opts) > 0 {
		maxFPS = opts[0].MaxFPS
	}

	cleanContent := []byte(session.StripMetadataOnly(string(sessionContent)))
	if len(cleanContent) == 0 {
		cleanContent = []byte(session.StripMetadata(string(sessionContent)))
	}

	var frames []Frame
	var offset int
	var currentTime float64
	var pending []byte
	var pendingStart float64
	var pendingLast float64

	frameInterval := 0.0
	if maxFPS > 0 {
		frameInterval = 1.0 / maxFPS
	}

	flushPending := func() {
		if len(pending) == 0 {
			return
		}
		frames = append(frames, Frame{
			Timestamp: roundSeconds(pendingLast),
			Content:   string(pending),
		})
		pending = nil
		pendingStart = 0
		pendingLast = 0
	}

	for _, entry := range entries {
		currentTime += entry.Delay
		if entry.Type != timing.Output {
			continue
		}
		if entry.ByteCount <= 0 || offset >= len(cleanContent) {
			continue
		}

		end := offset + entry.ByteCount
		if end > len(cleanContent) {
			end = len(cleanContent)
		}
		chunk := cleanContent[offset:end]
		offset = end
		if len(chunk) == 0 {
			continue
		}

		if frameInterval <= 0 {
			frames = append(frames, Frame{Timestamp: roundSeconds(currentTime), Content: string(chunk)})
			continue
		}

		if len(pending) > 0 && currentTime-pendingStart >= frameInterval {
			flushPending()
		}
		if len(pending) == 0 {
			pendingStart = currentTime
		}
		pending = append(pending, chunk...)
		pendingLast = currentTime
	}

	flushPending()

	if offset < len(cleanContent) {
		frames = append(frames, Frame{
			Timestamp: roundSeconds(currentTime),
			Content:   string(cleanContent[offset:]),
		})
	}

	return frames, nil
}

// BuildAnimationMarkers extracts command labels and timestamps from input timing data.
func BuildAnimationMarkers(timingReader io.Reader, inputContent []byte) ([]AnimationMarker, error) {
	entries, err := timing.Parse(timingReader)
	if err != nil {
		return nil, err
	}

	commands := timing.ExtractCommands(entries, []byte(session.StripMetadataOnly(string(inputContent))))
	markers := make([]AnimationMarker, 0, len(commands))
	for _, command := range commands {
		markers = append(markers, AnimationMarker{
			Label:     command.Text,
			Timestamp: roundSeconds(command.Timestamp),
		})
	}
	return markers, nil
}

// RenderAnimatedHTML generates standalone HTML with timed terminal playback controls.
func RenderAnimatedHTML(frames []Frame, opts ...AnimationOptions) (string, error) {
	internalFrames := make([]html.PlaybackFrame, len(frames))
	for i, frame := range frames {
		internalFrames[i] = html.PlaybackFrame{
			Timestamp: frame.Timestamp,
			Content:   frame.Content,
		}
	}

	title := "Terminal"
	var footerLink html.FooterLink
	var markers []html.AnimationMarker
	var cues []html.AnimationCue
	if len(opts) > 0 {
		if opts[0].Title != "" {
			title = opts[0].Title
		}
		footerLink = html.FooterLink{
			Text: opts[0].FooterLink.Text,
			URL:  opts[0].FooterLink.URL,
		}
		for _, marker := range opts[0].Markers {
			markers = append(markers, html.AnimationMarker{
				Label:     marker.Label,
				Timestamp: marker.Timestamp,
			})
		}
		for _, cue := range opts[0].Cues {
			cues = append(cues, html.AnimationCue{
				Label:     cue.Label,
				Timestamp: cue.Timestamp,
				Speed:     cue.Speed,
				Pause:     cue.Pause,
			})
		}
	}

	return html.RenderAnimatedPlaybackHTML(internalFrames, title, footerLink, markers, cues)
}

func roundSeconds(value float64) float64 {
	return math.Round(value*1_000_000) / 1_000_000
}
