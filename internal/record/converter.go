package record

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/choonkeat/record-tui/internal/logfile"
	"github.com/choonkeat/record-tui/playback"
)

// ConvertSessionToHTML reads a session.log file, strips metadata, and generates HTML output.
//
// This function:
// 1. Reads the session.log file
// 2. Strips the session metadata (header/footer from `script` command)
// 3. Creates a playback frame with the cleaned content
// 4. Generates HTML with xterm.js rendering
// 5. Writes to session.log.html
//
// If timing and input files are found alongside the session log, a table-of-contents
// is generated and embedded in the HTML for navigation.
//
// Returns the path to the generated HTML file, or error if any step fails
func ConvertSessionToHTML(sessionLogPath string) (string, error) {
	// Validate input file exists
	if _, err := os.Stat(sessionLogPath); os.IsNotExist(err) {
		return "", fmt.Errorf("session.log not found: %s", sessionLogPath)
	}

	// Read session.log file (transparently handles .log.gz)
	sessionContent, err := logfile.ReadFile(sessionLogPath)
	if err != nil {
		return "", fmt.Errorf("cannot read session.log: %w", err)
	}

	// Strip session metadata (Script started/done lines from `script` command)
	cleanedContent := playback.StripMetadata(string(sessionContent))
	if cleanedContent == "" {
		return "", fmt.Errorf("session.log appears to be empty after metadata stripping")
	}

	// Create playback frame with all content at timestamp 0.0 (static display)
	frames := []playback.Frame{
		{
			Timestamp: 0.0,
			Content:   cleanedContent,
		},
	}

	// Try to generate TOC from timing/input files
	tocEntries := buildTOC(sessionLogPath, sessionContent)

	// Generate HTML using xterm.js
	opts := playback.Options{TOC: tocEntries}
	htmlContent, err := playback.RenderHTML(frames, opts)
	if err != nil {
		return "", fmt.Errorf("failed to generate HTML: %w", err)
	}

	// Determine output path (same as input but with .html extension)
	outputPath := sessionLogPath + ".html"

	// Write HTML to file
	err = os.WriteFile(outputPath, []byte(htmlContent), 0644)
	if err != nil {
		return "", fmt.Errorf("failed to write HTML file: %w", err)
	}

	return outputPath, nil
}

// ConvertSessionToHTMLWithPath is like ConvertSessionToHTML but allows specifying output path
func ConvertSessionToHTMLWithPath(sessionLogPath string, outputPath string) (string, error) {
	// Validate input file exists
	if _, err := os.Stat(sessionLogPath); os.IsNotExist(err) {
		return "", fmt.Errorf("session.log not found: %s", sessionLogPath)
	}

	// Resolve to absolute paths
	sessionPath, err := filepath.Abs(sessionLogPath)
	if err != nil {
		return "", fmt.Errorf("invalid session path: %w", err)
	}

	outPath, err := filepath.Abs(outputPath)
	if err != nil {
		return "", fmt.Errorf("invalid output path: %w", err)
	}

	// Read session.log (transparently handles .log.gz)
	sessionContent, err := logfile.ReadFile(sessionPath)
	if err != nil {
		return "", fmt.Errorf("cannot read session.log: %w", err)
	}

	// Strip metadata
	cleanedContent := playback.StripMetadata(string(sessionContent))
	if cleanedContent == "" {
		return "", fmt.Errorf("session.log appears to be empty after metadata stripping")
	}

	// Create playback frame
	frames := []playback.Frame{
		{
			Timestamp: 0.0,
			Content:   cleanedContent,
		},
	}

	// Try to generate TOC from timing/input files
	tocEntries := buildTOC(sessionPath, sessionContent)

	// Generate HTML
	opts := playback.Options{TOC: tocEntries}
	htmlContent, err := playback.RenderHTML(frames, opts)
	if err != nil {
		return "", fmt.Errorf("failed to generate HTML: %w", err)
	}

	// Write HTML
	err = os.WriteFile(outPath, []byte(htmlContent), 0644)
	if err != nil {
		return "", fmt.Errorf("failed to write HTML file: %w", err)
	}

	return outPath, nil
}

// ConvertSessionToStreamingHTML generates streaming HTML that fetches session data via JavaScript.
// Unlike ConvertSessionToHTML which embeds all content, this generates lightweight HTML (~15KB)
// that streams content from the log file. The HTML must be served via HTTP (not file://).
//
// maxRows specifies the initial viewport size before auto-resize (e.g., 100000).
// Output is written to session.log.streaming.html
func ConvertSessionToStreamingHTML(sessionLogPath string, maxRows uint32) (string, error) {
	// Validate input file exists
	if _, err := os.Stat(sessionLogPath); os.IsNotExist(err) {
		return "", fmt.Errorf("session.log not found: %s", sessionLogPath)
	}

	// Try to generate TOC from timing/input files
	var tocEntries []playback.TOCEntry
	sessionContent, err := logfile.ReadFile(sessionLogPath)
	if err == nil {
		tocEntries = buildTOC(sessionLogPath, sessionContent)
	}

	// Generate streaming HTML that references the log file
	logFileName := filepath.Base(sessionLogPath)
	htmlContent, err := playback.RenderStreamingHTML(playback.StreamingOptions{
		Title:   logFileName,
		DataURL: "./" + logFileName,
		MaxRows: maxRows,
		TOC:     tocEntries,
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate streaming HTML: %w", err)
	}

	// Output path: session.log.streaming.html
	outputPath := sessionLogPath + ".streaming.html"

	// Write HTML to file
	err = os.WriteFile(outputPath, []byte(htmlContent), 0644)
	if err != nil {
		return "", fmt.Errorf("failed to write HTML file: %w", err)
	}

	return outputPath, nil
}

// ConvertSessionToAnimatedHTML reads session.log plus session.timing and generates
// an animated HTML replay with playback controls.
func ConvertSessionToAnimatedHTML(sessionLogPath string) (string, error) {
	return ConvertSessionToAnimatedHTMLWithCues(sessionLogPath, nil)
}

// ConvertSessionToAnimatedHTMLWithCues is like ConvertSessionToAnimatedHTML but
// embeds authored speed/pause cues into the playback UI.
func ConvertSessionToAnimatedHTMLWithCues(sessionLogPath string, cues []playback.AnimationCue) (string, error) {
	if _, err := os.Stat(sessionLogPath); os.IsNotExist(err) {
		return "", fmt.Errorf("session.log not found: %s", sessionLogPath)
	}

	sessionContent, err := logfile.ReadFile(sessionLogPath)
	if err != nil {
		return "", fmt.Errorf("cannot read session.log: %w", err)
	}

	timingPath := logfile.CompanionPath(sessionLogPath, ".timing")
	timingBytes, err := os.ReadFile(timingPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("session.timing not found: %s", timingPath)
		}
		return "", fmt.Errorf("cannot read session.timing: %w", err)
	}

	frames, err := playback.BuildAnimationFrames(bytes.NewReader(timingBytes), sessionContent)
	if err != nil {
		return "", fmt.Errorf("failed to build animation frames: %w", err)
	}
	if len(frames) == 0 {
		return "", fmt.Errorf("no animation frames generated from %s", timingPath)
	}

	var markers []playback.AnimationMarker
	inputPath := logfile.CompanionPath(sessionLogPath, ".input")
	if inputBytes, err := os.ReadFile(inputPath); err == nil {
		markers, _ = playback.BuildAnimationMarkers(bytes.NewReader(timingBytes), inputBytes)
	}

	htmlContent, err := playback.RenderAnimatedHTML(frames, playback.AnimationOptions{
		Title:   filepath.Base(sessionLogPath),
		Markers: markers,
		Cues:    cues,
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate animated HTML: %w", err)
	}

	outputPath := sessionLogPath + ".animated.html"
	err = os.WriteFile(outputPath, []byte(htmlContent), 0644)
	if err != nil {
		return "", fmt.Errorf("failed to write animated HTML file: %w", err)
	}

	return outputPath, nil
}

// buildTOC attempts to build TOC entries from timing and input files alongside the session log.
// Returns nil if timing or input files are not found or cannot be parsed.
//
// Expected file naming convention:
//   - session.log      → session.timing, session.input
//   - session-UUID.log → session-UUID.timing, session-UUID.input
func buildTOC(sessionLogPath string, sessionContent []byte) []playback.TOCEntry {
	timingPath := logfile.CompanionPath(sessionLogPath, ".timing")
	inputPath := logfile.CompanionPath(sessionLogPath, ".input")

	timingFile, err := os.Open(timingPath)
	if err != nil {
		return nil
	}
	defer timingFile.Close()

	inputBytes, err := os.ReadFile(inputPath)
	if err != nil {
		return nil
	}

	return playback.BuildTOC(timingFile, inputBytes, bytes.NewReader(sessionContent))
}
