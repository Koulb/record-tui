package record

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/choonkeat/record-tui/internal/logfile"
	"github.com/creack/pty"
	"golang.org/x/term"
)

// RecordSession executes the `script` command to record a terminal session.
// The `script` command reads from stdin and writes terminal output to a file.
//
// Args:
//   - outputPath: Path to the session.log file to create
//   - args: Command and arguments to execute within the session
//     If empty, script will use the default shell
//
// Returns error if script command fails or cannot be executed
func RecordSession(outputPath string, args []string) error {
	// Build command: script <outputPath> [additional args]
	cmdArgs := []string{outputPath}
	cmdArgs = append(cmdArgs, args...)

	cmd := exec.Command("script", cmdArgs...)

	// Inherit stdin/stdout/stderr so user can interact with the recorded session
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Execute script command
	err := cmd.Run()
	if err != nil {
		// script returns exit code 0 normally, so any error is a real problem
		return fmt.Errorf("script command failed: %w", err)
	}

	return nil
}

// RecordSessionWithTiming records a terminal session and writes timing/input data.
func RecordSessionWithTiming(outputPath string, timingPath string, args []string) error {
	cmd, commandLabel := commandFromArgs(args)

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("cannot create recording directory: %w", err)
	}

	logFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("cannot create session.log: %w", err)
	}
	defer logFile.Close()

	timingFile, err := os.Create(timingPath)
	if err != nil {
		return fmt.Errorf("cannot create session.timing: %w", err)
	}
	defer timingFile.Close()

	inputPath := logfile.CompanionPath(outputPath, ".input")
	inputFile, err := os.Create(inputPath)
	if err != nil {
		return fmt.Errorf("cannot create session.input: %w", err)
	}
	defer inputFile.Close()

	logWriter := bufio.NewWriter(logFile)
	timingWriter := bufio.NewWriter(timingFile)
	inputWriter := bufio.NewWriter(inputFile)
	defer logWriter.Flush()
	defer timingWriter.Flush()
	defer inputWriter.Flush()

	startedAt := time.Now()
	fmt.Fprintf(logWriter, "Script started on %s\n", startedAt.Format(time.ANSIC))
	fmt.Fprintf(logWriter, "Command: %s\n", commandLabel)
	fmt.Fprintf(timingWriter, "H 0.000000 START_TIME %s\n", startedAt.Format(time.RFC3339))
	fmt.Fprintf(timingWriter, "H 0.000000 COMMAND %s\n", commandLabel)
	fmt.Fprintf(timingWriter, "H 0.000000 OUTPUT_LOG %s\n", outputPath)
	fmt.Fprintf(timingWriter, "H 0.000000 INPUT_LOG %s\n", inputPath)

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return fmt.Errorf("cannot start command in pty: %w", err)
	}
	defer ptmx.Close()

	stdinFD := int(os.Stdin.Fd())
	if term.IsTerminal(stdinFD) {
		oldState, err := term.MakeRaw(stdinFD)
		if err == nil {
			defer term.Restore(stdinFD, oldState)
		}
		_ = pty.InheritSize(os.Stdin, ptmx)
	}

	sigwinch := make(chan os.Signal, 1)
	signal.Notify(sigwinch, syscall.SIGWINCH)
	defer signal.Stop(sigwinch)
	go func() {
		for range sigwinch {
			_ = pty.InheritSize(os.Stdin, ptmx)
		}
	}()

	var timingMu sync.Mutex
	lastEvent := time.Now()
	writeTiming := func(kind byte, byteCount int) {
		timingMu.Lock()
		defer timingMu.Unlock()
		now := time.Now()
		delay := now.Sub(lastEvent).Seconds()
		if delay < 0 {
			delay = 0
		}
		fmt.Fprintf(timingWriter, "%c %.6f %d\n", kind, delay, byteCount)
		_ = timingWriter.Flush()
		lastEvent = now
	}

	outputDone := make(chan struct{})
	go func() {
		defer close(outputDone)
		buf := make([]byte, 32*1024)
		for {
			n, readErr := ptmx.Read(buf)
			if n > 0 {
				chunk := buf[:n]
				_, _ = os.Stdout.Write(chunk)
				_, _ = logWriter.Write(chunk)
				_ = logWriter.Flush()
				writeTiming('O', n)
			}
			if readErr != nil {
				if readErr != io.EOF {
					return
				}
				return
			}
		}
	}()

	go func() {
		buf := make([]byte, 4096)
		for {
			n, readErr := os.Stdin.Read(buf)
			if n > 0 {
				chunk := buf[:n]
				_, _ = ptmx.Write(chunk)
				_, _ = inputWriter.Write(chunk)
				_ = inputWriter.Flush()
				writeTiming('I', n)
			}
			if readErr != nil {
				return
			}
		}
	}()

	waitErr := cmd.Wait()
	_ = ptmx.Close()
	<-outputDone

	exitStatus := 0
	if waitErr != nil {
		exitStatus = 1
		if exitErr, ok := waitErr.(*exec.ExitError); ok {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				exitStatus = status.ExitStatus()
			}
		}
	}

	finishedAt := time.Now()
	fmt.Fprintf(logWriter, "\nCommand exit status: %d\n", exitStatus)
	fmt.Fprintf(logWriter, "Script done on %s\n", finishedAt.Format(time.ANSIC))
	fmt.Fprintf(timingWriter, "H 0.000000 COMMAND_EXIT_STATUS %d\n", exitStatus)

	if waitErr != nil {
		return fmt.Errorf("recorded command exited with status %d: %w", exitStatus, waitErr)
	}

	return nil
}

func commandFromArgs(args []string) (*exec.Cmd, string) {
	if len(args) == 0 {
		shell := os.Getenv("SHELL")
		if shell == "" {
			shell = "/bin/sh"
		}
		return exec.Command(shell), shell
	}
	return exec.Command(args[0], args[1:]...), commandLabel(args)
}

func commandLabel(args []string) string {
	if len(args) == 0 {
		return ""
	}
	label := args[0]
	for _, arg := range args[1:] {
		label += " " + arg
	}
	return label
}

// RecordSessionDetailed is like RecordSession but returns more info about execution
// Returns: exit code, error
func RecordSessionDetailed(outputPath string, args []string) (int, error) {
	cmdArgs := []string{outputPath}
	cmdArgs = append(cmdArgs, args...)

	cmd := exec.Command("script", cmdArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				return status.ExitStatus(), err
			}
		}
		return 1, err
	}

	return 0, nil
}
