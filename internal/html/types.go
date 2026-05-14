package html

// PlaybackFrame represents a single frame of terminal content at a specific timestamp
type PlaybackFrame struct {
	Timestamp float64 `json:"timestamp"` // Time in seconds (cumulative from start)
	Content   string  `json:"content"`   // Terminal content (with ANSI codes preserved)
}

// FooterLink represents a co-branding link in the footer
type FooterLink struct {
	Text string // Display text (e.g., "swe-swe")
	URL  string // Link URL (e.g., "https://github.com/choonkeat/swe-swe")
}

// TOCEntry represents a navigation point in the terminal recording.
type TOCEntry struct {
	Label string `json:"label"` // What the user typed (e.g., "npm test")
	Line  int    `json:"line"`  // Line number in the output (0-indexed)
}

// AnimationMarker represents a clickable point on the animation timeline.
type AnimationMarker struct {
	Label     string  `json:"label"`
	Timestamp float64 `json:"timestamp"`
}

// AnimationCue represents an authored playback action.
type AnimationCue struct {
	Label     string  `json:"label,omitempty"`
	Timestamp float64 `json:"timestamp"`
	Speed     float64 `json:"speed,omitempty"`
	Pause     float64 `json:"pause,omitempty"`
}
