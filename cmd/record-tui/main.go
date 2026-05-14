package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/choonkeat/record-tui/internal/record"
	"github.com/choonkeat/record-tui/playback"
)

func printUsage() {
	fmt.Fprintf(os.Stderr, `Usage: record-tui [command ...]

Start a terminal recording session and automatically convert to HTML.

Flags:
  -animated    Record timing data and generate animated HTML
  -animate     Alias for -animated
  -convert     Convert an existing session.log to HTML
  -cues        JSON cue file for animated speed changes/pauses
  -streaming   Generate streaming HTML in convert mode

Arguments:
  [command ...]  Command to execute in the recorded session (optional)
                 If omitted, starts an interactive shell

Examples:
  record-tui                  # Start interactive shell recording
  record-tui echo hello       # Record specific command
  record-tui -animated codex  # Record Codex and generate animated HTML
  record-tui /bin/bash        # Record bash session
`)
}

// getRecordingDir creates and returns the recording directory path
// Format: ~/.record-tui/YYYYMMDD-HHMMSS/
func getRecordingDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}

	baseDir := filepath.Join(homeDir, ".record-tui")
	timestamp := time.Now().Format("20060102-150405")
	recordingDir := filepath.Join(baseDir, timestamp)

	// Create directory with permissions 0755
	err = os.MkdirAll(recordingDir, 0755)
	if err != nil {
		return "", fmt.Errorf("cannot create recording directory %s: %w", recordingDir, err)
	}

	return recordingDir, nil
}

// isInteractiveTerminal checks if stdout is connected to a TTY
func isInteractiveTerminal() bool {
	stat, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	// Check if stdout is a character device (terminal)
	return (stat.Mode() & os.ModeCharDevice) != 0
}

// isSSHSession checks if running over SSH
func isSSHSession() bool {
	return os.Getenv("SSH_CLIENT") != ""
}

// openRecordingDir opens the recording directory in the file explorer
// Only opens if running in an interactive terminal and not over SSH
func openRecordingDir(dir string) {
	// Skip if not interactive
	if !isInteractiveTerminal() {
		return
	}
	// Skip if over SSH
	if isSSHSession() {
		return
	}
	// Open directory (silently ignore errors)
	exec.Command("open", dir).Run()
}

func main() {
	convertFlag := flag.String("convert", "", "Convert session.log to HTML (outputs <file>.html)")
	streamingFlag := flag.Bool("streaming", false, "Generate streaming HTML instead (outputs <file>.streaming.html)")
	animatedFlag := flag.Bool("animated", false, "Generate animated HTML with playback controls (outputs <file>.animated.html)")
	animateFlag := flag.Bool("animate", false, "Alias for -animated")
	cuesFlag := flag.String("cues", "", "JSON cue file for animated playback")
	flag.Parse()
	args := flag.Args()
	animated := *animatedFlag || *animateFlag

	if animated && *streamingFlag {
		fmt.Fprintf(os.Stderr, "Error: -animated and -streaming cannot be used together\n")
		os.Exit(1)
	}
	if *cuesFlag != "" && !animated {
		fmt.Fprintf(os.Stderr, "Error: -cues requires -animated\n")
		os.Exit(1)
	}

	var cues []playback.AnimationCue
	if *cuesFlag != "" {
		var err error
		cues, err = loadAnimationCues(*cuesFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Cue file failed: %v\n", err)
			os.Exit(1)
		}
	}

	// Handle conversion mode
	if *convertFlag != "" {
		var htmlPath string
		var err error
		if animated {
			htmlPath, err = record.ConvertSessionToAnimatedHTMLWithCues(*convertFlag, cues)
		} else if *streamingFlag {
			htmlPath, err = record.ConvertSessionToStreamingHTML(*convertFlag, 100000)
		} else {
			htmlPath, err = record.ConvertSessionToHTML(*convertFlag)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Conversion failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "✓ HTML generated: %s\n", htmlPath)

		if !animated {
			// Try to convert HTML to PDF if to-pdf tool is available
			pdfPath := htmlPath[:len(htmlPath)-len(".html")] + ".pdf"
			cmd := exec.Command("to-pdf", htmlPath, pdfPath)
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err == nil {
				fmt.Fprintf(os.Stderr, "✓ PDF generated: %s\n", pdfPath)
			}
		}
		// Silently ignore if to-pdf not found or fails

		os.Exit(0)
	}

	// Setup environment for color recording
	record.SetupRecordingEnvironment()

	// Create recording directory
	recordingDir, err := getRecordingDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to create recording directory: %v\n", err)
		os.Exit(1)
	}

	sessionLogPath := filepath.Join(recordingDir, "session.log")
	sessionTimingPath := filepath.Join(recordingDir, "session.timing")

	// Record the session
	fmt.Fprintf(os.Stderr, "Recording started. Press Ctrl-D to exit.\n")
	if animated {
		err = record.RecordSessionWithTiming(sessionLogPath, sessionTimingPath, args)
	} else {
		err = record.RecordSession(sessionLogPath, args)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Recording failed: %v\n", err)
		os.Exit(1)
	}

	// Verify session.log was created
	if _, err := os.Stat(sessionLogPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: session.log was not created\n")
		os.Exit(1)
	}

	// Convert session.log to HTML
	var htmlPath string
	if animated {
		htmlPath, err = record.ConvertSessionToAnimatedHTMLWithCues(sessionLogPath, cues)
	} else {
		htmlPath, err = record.ConvertSessionToHTML(sessionLogPath)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: HTML conversion failed: %v\n", err)
		fmt.Fprintf(os.Stderr, "Note: session.log was recorded successfully\n")
		// Don't exit - recording was successful even if conversion failed
	} else {
		fmt.Fprintf(os.Stderr, "✓ HTML generated: %s\n", htmlPath)

		if !animated {
			// Try to convert HTML to PDF if to-pdf tool is available
			pdfPath := htmlPath[:len(htmlPath)-len(".html")] + ".pdf"
			cmd := exec.Command("to-pdf", htmlPath, pdfPath)
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err == nil {
				fmt.Fprintf(os.Stderr, "✓ PDF generated: %s\n", pdfPath)
			}
		}
		// Silently ignore if to-pdf not found or fails
	}

	// Success message
	fmt.Fprintf(os.Stderr, "✓ Recording saved to: %s/\n", recordingDir)

	// Open directory in file explorer (interactive terminals only, skip SSH)
	openRecordingDir(recordingDir)

	os.Exit(0)
}

func loadAnimationCues(path string) ([]playback.AnimationCue, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %w", path, err)
	}

	var cues []playback.AnimationCue
	if err := json.Unmarshal(content, &cues); err != nil {
		return nil, fmt.Errorf("cannot parse %s: %w", path, err)
	}
	return cues, nil
}
