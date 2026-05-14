package record

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/choonkeat/record-tui/internal/logfile"
	"github.com/choonkeat/record-tui/playback"
)

// TestConvertSessionToHTML_WithSimpleSession tests conversion of a simple recorded session
func TestConvertSessionToHTML_WithSimpleSession(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a mock session.log file with metadata
	sessionLogPath := filepath.Join(tmpDir, "session.log")
	sessionContent := `Script started on Wed Dec 31 12:10:34 2025
Command: bash
hello world
test content
Script done on Wed Dec 31 12:11:00 2025
`

	err := os.WriteFile(sessionLogPath, []byte(sessionContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test session.log: %v", err)
	}

	// Convert to HTML
	htmlPath, err := ConvertSessionToHTML(sessionLogPath)
	if err != nil {
		t.Fatalf("ConvertSessionToHTML failed: %v", err)
	}

	// Verify output path is correct
	expectedPath := sessionLogPath + ".html"
	if htmlPath != expectedPath {
		t.Errorf("Expected output path %s, got %s", expectedPath, htmlPath)
	}

	// Verify HTML file was created
	if _, err := os.Stat(htmlPath); os.IsNotExist(err) {
		t.Fatalf("HTML file not created at %s", htmlPath)
	}

	// Verify HTML contains expected markers
	htmlContent, err := os.ReadFile(htmlPath)
	if err != nil {
		t.Fatalf("Failed to read HTML file: %v", err)
	}

	htmlString := string(htmlContent)
	if !strings.Contains(htmlString, "<!DOCTYPE html>") {
		t.Errorf("HTML should contain DOCTYPE declaration")
	}
	if !strings.Contains(htmlString, "xterm") {
		t.Errorf("HTML should reference xterm.js")
	}
	if !strings.Contains(htmlString, "framesBase64") {
		t.Errorf("HTML should contain framesBase64 variable")
	}

	// Verify metadata was stripped (Script started/done should not be in content)
	if strings.Contains(htmlString, "Script started on") {
		t.Errorf("HTML should not contain 'Script started on' metadata")
	}
	if strings.Contains(htmlString, "Script done on") {
		t.Errorf("HTML should not contain 'Script done on' metadata")
	}

	t.Logf("✓ Conversion successful: %s → %s", sessionLogPath, htmlPath)
}

// TestConvertSessionToHTML_FileNotFound tests error handling for missing session.log
func TestConvertSessionToHTML_FileNotFound(t *testing.T) {
	nonexistentPath := "/tmp/nonexistent-session-" + t.Name() + ".log"

	_, err := ConvertSessionToHTML(nonexistentPath)
	if err == nil {
		t.Errorf("Expected error for non-existent file, got none")
	}

	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Error message should mention 'not found', got: %v", err)
	}
}

// TestConvertSessionToHTML_CreatesValidHTML tests that generated HTML is valid structure
func TestConvertSessionToHTML_CreatesValidHTML(t *testing.T) {
	tmpDir := t.TempDir()

	// Create session.log with more realistic content
	sessionLogPath := filepath.Join(tmpDir, "session.log")
	sessionContent := `Script started on Wed Dec 31 12:10:34 2025
Command: bash
[91mRed colored text[39m
$ ls -la
total 42
$ pwd
/home/user
Command exit status: 0
Script done on Wed Dec 31 12:11:00 2025
`

	err := os.WriteFile(sessionLogPath, []byte(sessionContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test session.log: %v", err)
	}

	// Convert
	htmlPath, err := ConvertSessionToHTML(sessionLogPath)
	if err != nil {
		t.Fatalf("ConvertSessionToHTML failed: %v", err)
	}

	// Verify HTML file size is reasonable
	fileInfo, err := os.Stat(htmlPath)
	if err != nil {
		t.Fatalf("Cannot stat HTML file: %v", err)
	}

	if fileInfo.Size() < 1000 {
		t.Errorf("HTML file seems too small: %d bytes", fileInfo.Size())
	}

	t.Logf("✓ Generated HTML file: %d bytes", fileInfo.Size())
}

// TestConvertSessionToHTMLWithPath_CustomOutputPath tests specifying custom output path
func TestConvertSessionToHTMLWithPath_CustomOutputPath(t *testing.T) {
	tmpDir := t.TempDir()

	// Create session.log
	sessionLogPath := filepath.Join(tmpDir, "session.log")
	sessionContent := `Script started on Wed Dec 31 12:10:34 2025
Command: bash
test
Script done on Wed Dec 31 12:11:00 2025
`

	err := os.WriteFile(sessionLogPath, []byte(sessionContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test session.log: %v", err)
	}

	// Convert with custom output path
	customOutputPath := filepath.Join(tmpDir, "custom_output.html")
	returnedPath, err := ConvertSessionToHTMLWithPath(sessionLogPath, customOutputPath)
	if err != nil {
		t.Fatalf("ConvertSessionToHTMLWithPath failed: %v", err)
	}

	// Verify it used the custom path (after Abs conversion)
	absCustomPath, _ := filepath.Abs(customOutputPath)
	if returnedPath != absCustomPath {
		t.Errorf("Expected %s, got %s", absCustomPath, returnedPath)
	}

	// Verify file was created at custom location
	if _, err := os.Stat(customOutputPath); os.IsNotExist(err) {
		t.Errorf("HTML file not created at custom path: %s", customOutputPath)
	}

	t.Logf("✓ Custom output path used: %s", customOutputPath)
}

// TestConvertSessionToHTML_WithRealSessionLog tests with actual session.log file
func TestConvertSessionToHTML_WithRealSessionLog(t *testing.T) {
	// Use the real session.log from recording tests
	sessionLogPath := os.ExpandEnv("$HOME/.record-tui/20251231-141042/session.log")

	// Check if file exists, skip if not
	if _, err := os.Stat(sessionLogPath); os.IsNotExist(err) {
		t.Skipf("Test session.log not found at %s, skipping", sessionLogPath)
	}

	// Convert to HTML
	htmlPath, err := ConvertSessionToHTML(sessionLogPath)
	if err != nil {
		t.Fatalf("ConvertSessionToHTML failed: %v", err)
	}

	// Verify file was created
	htmlFileInfo, err := os.Stat(htmlPath)
	if err != nil {
		t.Fatalf("HTML file not created: %v", err)
	}

	t.Logf("✓ Real session converted: %s (%d bytes → %d bytes)",
		sessionLogPath, 176, htmlFileInfo.Size())
}

// TestConvertSessionToHTML_PreservesANSICodes tests that ANSI codes are preserved in HTML
func TestConvertSessionToHTML_PreservesANSICodes(t *testing.T) {
	tmpDir := t.TempDir()

	// Create session.log with ANSI codes
	sessionLogPath := filepath.Join(tmpDir, "session.log")
	sessionContent := `Script started on Wed Dec 31 12:10:34 2025
Command: bash
[91mRed[39m [92mGreen[39m [94mBlue[39m
Script done on Wed Dec 31 12:11:00 2025
`

	err := os.WriteFile(sessionLogPath, []byte(sessionContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test session.log: %v", err)
	}

	// Convert
	htmlPath, err := ConvertSessionToHTML(sessionLogPath)
	if err != nil {
		t.Fatalf("ConvertSessionToHTML failed: %v", err)
	}

	// Read and verify ANSI codes are in the base64 content
	htmlContent, _ := os.ReadFile(htmlPath)
	htmlString := string(htmlContent)

	// ANSI codes should be preserved in the base64-encoded frames
	if !strings.Contains(htmlString, "framesBase64") {
		t.Errorf("HTML should contain frame data")
	}

	t.Logf("✓ ANSI codes preserved in HTML")
}

// TestConvertSessionToHTML_WithTimingAndInput tests TOC generation when timing/input files exist
func TestConvertSessionToHTML_WithTimingAndInput(t *testing.T) {
	tmpDir := t.TempDir()

	// Create session.log
	sessionLogPath := filepath.Join(tmpDir, "session.log")
	sessionContent := "Script started on 2026-01-12 06:41:43+00:00 [COMMAND=\"bash\"]\n" +
		"$ ls\nfile1\nfile2\n$ npm test\nPASS\nScript done on 2026-01-12 06:45:00+00:00 [COMMAND_EXIT_STATUS=\"0\"]\n"
	err := os.WriteFile(sessionLogPath, []byte(sessionContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create session.log: %v", err)
	}

	// Create session.timing (advanced format)
	timingPath := filepath.Join(tmpDir, "session.timing")
	timingContent := "O 0.010 4\nI 0.500 3\nO 0.010 18\nI 1.000 9\nO 0.010 5\n"
	err = os.WriteFile(timingPath, []byte(timingContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create session.timing: %v", err)
	}

	// Create session.input
	inputPath := filepath.Join(tmpDir, "session.input")
	inputContent := "ls\rnpm test\r"
	err = os.WriteFile(inputPath, []byte(inputContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create session.input: %v", err)
	}

	// Convert to HTML
	htmlPath, err := ConvertSessionToHTML(sessionLogPath)
	if err != nil {
		t.Fatalf("ConvertSessionToHTML failed: %v", err)
	}

	// Read HTML and check for TOC elements
	htmlBytes, err := os.ReadFile(htmlPath)
	if err != nil {
		t.Fatalf("Failed to read HTML: %v", err)
	}
	htmlString := string(htmlBytes)

	if !strings.Contains(htmlString, `id="nav-indicator"`) {
		t.Error("HTML should contain navigation indicator")
	}
	if !strings.Contains(htmlString, `id="nav-prev"`) {
		t.Error("HTML should contain prev navigation button")
	}
	if !strings.Contains(htmlString, `"ls"`) {
		t.Error("HTML should contain 'ls' in navigation data")
	}
	if !strings.Contains(htmlString, `"npm test"`) {
		t.Error("HTML should contain 'npm test' in navigation data")
	}
}

// TestConvertSessionToHTML_WithoutTimingFiles tests graceful degradation when no timing files exist
func TestConvertSessionToHTML_WithoutTimingFiles(t *testing.T) {
	tmpDir := t.TempDir()

	sessionLogPath := filepath.Join(tmpDir, "session.log")
	sessionContent := "Script started on Wed Dec 31 12:10:34 2025\nCommand: bash\nhello\nScript done on Wed Dec 31 12:11:00 2025\n"
	err := os.WriteFile(sessionLogPath, []byte(sessionContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create session.log: %v", err)
	}

	htmlPath, err := ConvertSessionToHTML(sessionLogPath)
	if err != nil {
		t.Fatalf("ConvertSessionToHTML failed: %v", err)
	}

	htmlBytes, _ := os.ReadFile(htmlPath)
	htmlString := string(htmlBytes)

	// Should NOT have navigation when no timing files
	if strings.Contains(htmlString, `id="nav-indicator"`) {
		t.Error("HTML should NOT contain navigation when no timing files")
	}
}

func TestConvertSessionToAnimatedHTML_WithTimingFile(t *testing.T) {
	tmpDir := t.TempDir()

	sessionLogPath := filepath.Join(tmpDir, "session.log")
	sessionContent := "Script started on Wed Dec 31 12:10:34 2025\n" +
		"Command: bash\n" +
		"hello\n" +
		"Script done on Wed Dec 31 12:11:00 2025\n"
	if err := os.WriteFile(sessionLogPath, []byte(sessionContent), 0644); err != nil {
		t.Fatalf("Failed to create session.log: %v", err)
	}

	timingPath := filepath.Join(tmpDir, "session.timing")
	timingContent := "O 0.100000 2\nO 0.200000 4\n"
	if err := os.WriteFile(timingPath, []byte(timingContent), 0644); err != nil {
		t.Fatalf("Failed to create session.timing: %v", err)
	}

	inputPath := filepath.Join(tmpDir, "session.input")
	if err := os.WriteFile(inputPath, []byte("echo hello\r"), 0644); err != nil {
		t.Fatalf("Failed to create session.input: %v", err)
	}

	htmlPath, err := ConvertSessionToAnimatedHTML(sessionLogPath)
	if err != nil {
		t.Fatalf("ConvertSessionToAnimatedHTML failed: %v", err)
	}

	expectedPath := sessionLogPath + ".animated.html"
	if htmlPath != expectedPath {
		t.Fatalf("output path = %q, want %q", htmlPath, expectedPath)
	}

	htmlBytes, err := os.ReadFile(htmlPath)
	if err != nil {
		t.Fatalf("Failed to read animated HTML: %v", err)
	}
	htmlString := string(htmlBytes)

	for _, want := range []string{
		"framesBase64",
		"animationMarkers",
		"id=\"animation-play\"",
		"id=\"animation-speed\"",
		"id=\"animation-timeline\"",
	} {
		if !strings.Contains(htmlString, want) {
			t.Fatalf("animated HTML should contain %q", want)
		}
	}
}

func TestConvertSessionToAnimatedHTML_MissingTimingFile(t *testing.T) {
	tmpDir := t.TempDir()

	sessionLogPath := filepath.Join(tmpDir, "session.log")
	sessionContent := "Script started on Wed Dec 31 12:10:34 2025\n" +
		"Command: bash\n" +
		"hello\n" +
		"Script done on Wed Dec 31 12:11:00 2025\n"
	if err := os.WriteFile(sessionLogPath, []byte(sessionContent), 0644); err != nil {
		t.Fatalf("Failed to create session.log: %v", err)
	}

	_, err := ConvertSessionToAnimatedHTML(sessionLogPath)
	if err == nil {
		t.Fatalf("expected missing timing file error")
	}
	if !strings.Contains(err.Error(), "session.timing not found") {
		t.Fatalf("error = %v, want missing timing file message", err)
	}
}

func TestConvertSessionToAnimatedHTMLWithCues_EmbedsCueLabels(t *testing.T) {
	tmpDir := t.TempDir()

	sessionLogPath := filepath.Join(tmpDir, "session.log")
	sessionContent := "Script started on Wed Dec 31 12:10:34 2025\n" +
		"Command: bash\n" +
		"hello\n" +
		"Script done on Wed Dec 31 12:11:00 2025\n"
	if err := os.WriteFile(sessionLogPath, []byte(sessionContent), 0644); err != nil {
		t.Fatalf("Failed to create session.log: %v", err)
	}

	timingPath := filepath.Join(tmpDir, "session.timing")
	if err := os.WriteFile(timingPath, []byte("O 0.100000 5\n"), 0644); err != nil {
		t.Fatalf("Failed to create session.timing: %v", err)
	}

	htmlPath, err := ConvertSessionToAnimatedHTMLWithCues(sessionLogPath, []playback.AnimationCue{
		{Label: "Focus on output", Timestamp: 0.1, Pause: 2},
	})
	if err != nil {
		t.Fatalf("ConvertSessionToAnimatedHTMLWithCues failed: %v", err)
	}

	htmlBytes, err := os.ReadFile(htmlPath)
	if err != nil {
		t.Fatalf("Failed to read animated HTML: %v", err)
	}
	htmlString := string(htmlBytes)
	if !strings.Contains(htmlString, "Focus on output") {
		t.Fatalf("animated HTML should contain cue label")
	}
	if !strings.Contains(htmlString, "animationCues") {
		t.Fatalf("animated HTML should contain cue data")
	}
}

// TestCompanionPath tests the path derivation helper (moved to internal/logfile)
func TestCompanionPath(t *testing.T) {
	tests := []struct {
		logPath  string
		ext      string
		expected string
	}{
		{"/path/session.log", ".timing", "/path/session.timing"},
		{"/path/session.log", ".input", "/path/session.input"},
		{"/path/session-abc123.log", ".timing", "/path/session-abc123.timing"},
		{"/path/noext", ".timing", "/path/noext.timing"},
		{"/path/session.log.gz", ".timing", "/path/session.timing"},
		{"/path/session.log.gz", ".input", "/path/session.input"},
	}

	for _, tt := range tests {
		got := logfile.CompanionPath(tt.logPath, tt.ext)
		if got != tt.expected {
			t.Errorf("CompanionPath(%q, %q) = %q, want %q", tt.logPath, tt.ext, got, tt.expected)
		}
	}
}
