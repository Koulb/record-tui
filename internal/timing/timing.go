// Package timing parses the advanced timing format produced by Linux script(1)
// when both input and output logging are enabled (e.g., --log-in and --log-timing).
//
// The advanced format includes type identifiers:
//
//	O 0.009404 16    # Output: 16 bytes
//	I 0.500000 1     # Input: 1 byte (user keystroke)
//	H 0.000000 0     # Header
//	S 0.000000 0     # Signal
//
// This package also supports the classic format (output only, no type prefix):
//
//	0.009404 16
//	0.440731 35
package timing

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// EntryType identifies the type of a timing entry.
type EntryType byte

const (
	Output EntryType = 'O'
	Input  EntryType = 'I'
	Header EntryType = 'H'
	Signal EntryType = 'S'
)

// Entry represents a single line in a timing file.
type Entry struct {
	Type      EntryType
	Delay     float64 // Seconds since previous entry
	ByteCount int     // Number of bytes in this chunk
}

// Command represents a user command extracted from grouped Input entries.
type Command struct {
	Text             string // What the user typed (e.g., "npm test")
	OutputByteOffset int    // Cumulative output bytes at the point this command was entered
	Timestamp        float64
}

// Parse reads a timing file and returns structured entries.
// Supports both advanced format (with I/O/H/S prefix) and classic format (no prefix, treated as Output).
func Parse(r io.Reader) ([]Entry, error) {
	var entries []Entry
	scanner := bufio.NewScanner(r)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		entry, err := parseLine(line)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNum, err)
		}
		entries = append(entries, entry)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading timing file: %w", err)
	}

	return entries, nil
}

func parseLine(line string) (Entry, error) {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return Entry{}, fmt.Errorf("expected at least 2 fields, got %d: %q", len(fields), line)
	}

	// Detect format: advanced has a single-char type prefix, classic starts with a number
	if len(fields[0]) == 1 && !isDigit(fields[0][0]) {
		typ := EntryType(fields[0][0])

		// H (Header) and S (Signal) entries may have extra metadata fields
		// e.g., "H 0.000000 START_TIME 2026-02-03 08:32:06+00:00"
		// We only care about I and O for command extraction, so parse H/S leniently
		if typ == Header || typ == Signal {
			delay := 0.0
			if len(fields) >= 2 {
				delay, _ = strconv.ParseFloat(fields[1], 64)
			}
			return Entry{Type: typ, Delay: delay, ByteCount: 0}, nil
		}

		// I and O entries: TYPE DELAY BYTECOUNT
		if len(fields) < 3 {
			return Entry{}, fmt.Errorf("advanced format requires 3 fields, got %d: %q", len(fields), line)
		}
		delay, err := strconv.ParseFloat(fields[1], 64)
		if err != nil {
			return Entry{}, fmt.Errorf("invalid delay %q: %w", fields[1], err)
		}
		byteCount, err := strconv.Atoi(fields[2])
		if err != nil {
			return Entry{}, fmt.Errorf("invalid byte count %q: %w", fields[2], err)
		}
		return Entry{Type: typ, Delay: delay, ByteCount: byteCount}, nil
	}

	// Classic format: DELAY BYTECOUNT (treated as Output)
	delay, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return Entry{}, fmt.Errorf("invalid delay %q: %w", fields[0], err)
	}
	byteCount, err := strconv.Atoi(fields[1])
	if err != nil {
		return Entry{}, fmt.Errorf("invalid byte count %q: %w", fields[1], err)
	}
	return Entry{Type: Output, Delay: delay, ByteCount: byteCount}, nil
}

func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

// ExtractCommands groups Input entries into user commands.
// inputContent is the raw bytes from the session.input file (with metadata stripped).
// Commands are identified by input terminated with \r or \n.
// Single control characters, arrow keys, and tab completions are filtered out.
//
// In real terminal recordings, each keystroke is typically a separate I entry
// followed by an O entry (the echo). This function accumulates input bytes
// across I/O pairs, splitting commands at \r or \n boundaries in the input stream.
func ExtractCommands(entries []Entry, inputContent []byte) []Command {
	var commands []Command
	var inputOffset int  // position in inputContent
	var outputOffset int // cumulative output bytes
	var currentTime float64
	var currentInput []byte // accumulating current command's input
	var commandOutputOffset int
	var commandTimestamp float64

	for _, e := range entries {
		currentTime += e.Delay

		switch e.Type {
		case Output:
			outputOffset += e.ByteCount

		case Input:
			if len(currentInput) == 0 {
				commandOutputOffset = outputOffset
				commandTimestamp = currentTime
			}
			end := inputOffset + e.ByteCount
			if end > len(inputContent) {
				end = len(inputContent)
			}
			if inputOffset < len(inputContent) {
				chunk := inputContent[inputOffset:end]
				for _, b := range chunk {
					currentInput = append(currentInput, b)
					// Split on \r or \n — this ends a command
					if b == '\r' || b == '\n' {
						cmd := finalizeCommand(currentInput, commandOutputOffset, commandTimestamp)
						if cmd != nil {
							commands = append(commands, *cmd)
						}
						currentInput = nil
					}
				}
			}
			inputOffset = end
		}
	}

	// Handle trailing input (no terminator)
	if len(currentInput) > 0 {
		cmd := finalizeCommand(currentInput, commandOutputOffset, commandTimestamp)
		if cmd != nil {
			commands = append(commands, *cmd)
		}
	}

	return commands
}

// finalizeCommand processes accumulated input bytes into a Command.
// Returns nil if the input should be filtered (control chars, arrows, etc.).
func finalizeCommand(input []byte, outputOffset int, timestamp float64) *Command {
	// Must end with \r or \n to be a command
	if len(input) == 0 {
		return nil
	}

	lastByte := input[len(input)-1]
	if lastByte != '\r' && lastByte != '\n' {
		return nil
	}

	// Trim the trailing \r or \n
	text := strings.TrimRight(string(input), "\r\n")

	// Filter out empty commands
	if text == "" {
		return nil
	}

	// Filter out single control characters (Ctrl-C = 0x03, etc.)
	if len(text) == 1 && text[0] < 0x20 {
		return nil
	}

	// Filter out pure escape sequences (arrow keys, etc.)
	if isOnlyEscapeSequences(text) {
		return nil
	}

	// Filter out single tab (tab completion)
	if text == "\t" {
		return nil
	}

	// Strip escape sequences from the label text
	text = stripEscapeSequences(text)

	// Re-check after stripping
	if text == "" {
		return nil
	}

	return &Command{
		Text:             text,
		OutputByteOffset: outputOffset,
		Timestamp:        timestamp,
	}
}

// stripEscapeSequences removes ANSI escape sequences from a string,
// keeping only printable text. Simulates a simple line editor to handle
// cursor movement (arrow keys), backspace (0x7F/DEL), readline word
// movement (ESC b/f), and readline editing commands (Ctrl+K/W/U/A/E).
func stripEscapeSequences(s string) string {
	var buf []byte
	cursor := 0
	i := 0
	for i < len(s) {
		if s[i] == 0x1b {
			// Parse ESC sequence
			i++
			if i >= len(s) {
				break
			}
			if s[i] == '[' {
				// CSI sequence: ESC [ params final
				i++
				paramStart := i
				for i < len(s) && s[i] >= 0x30 && s[i] <= 0x3f {
					i++
				}
				param := s[paramStart:i]
				// Skip intermediate bytes (0x20-0x2f)
				for i < len(s) && s[i] >= 0x20 && s[i] <= 0x2f {
					i++
				}
				if i < len(s) {
					final := s[i]
					i++
					switch final {
					case 'D': // Cursor left
						n := 1
						if param != "" {
							n, _ = strconv.Atoi(param)
							if n == 0 {
								n = 1
							}
						}
						cursor -= n
						if cursor < 0 {
							cursor = 0
						}
					case 'C': // Cursor right
						n := 1
						if param != "" {
							n, _ = strconv.Atoi(param)
							if n == 0 {
								n = 1
							}
						}
						cursor += n
						if cursor > len(buf) {
							cursor = len(buf)
						}
					case 'H', 'f': // Cursor home / position
						cursor = 0
					case '~': // Special keys
						switch param {
						case "3": // Delete key
							if cursor < len(buf) {
								buf = append(buf[:cursor], buf[cursor+1:]...)
							}
						}
						// 200~ and 201~ are bracketed paste markers - just ignore
					}
				}
			} else if s[i] == 'b' {
				// ESC b = word backward
				i++
				for cursor > 0 && (cursor-1 >= len(buf) || buf[cursor-1] == ' ') {
					cursor--
				}
				for cursor > 0 && cursor-1 < len(buf) && buf[cursor-1] != ' ' {
					cursor--
				}
			} else if s[i] == 'f' {
				// ESC f = word forward
				i++
				for cursor < len(buf) && buf[cursor] != ' ' {
					cursor++
				}
				for cursor < len(buf) && buf[cursor] == ' ' {
					cursor++
				}
			} else if s[i] == 'd' {
				// ESC d = kill word forward
				i++
				end := cursor
				for end < len(buf) && buf[end] == ' ' {
					end++
				}
				for end < len(buf) && buf[end] != ' ' {
					end++
				}
				buf = append(buf[:cursor], buf[end:]...)
			} else if s[i] == 0x7f {
				// ESC DEL = kill word backward
				i++
				start := cursor
				for start > 0 && buf[start-1] == ' ' {
					start--
				}
				for start > 0 && buf[start-1] != ' ' {
					start--
				}
				buf = append(buf[:start], buf[cursor:]...)
				cursor = start
			} else if s[i] == 0x1b {
				// Double ESC - don't consume the second ESC, let the loop handle it
			} else if isStringSequenceIntroducer(s[i]) {
				i = skipStringSequence(s, i+1)
			} else {
				// Other ESC + single char (e.g., ESC O for SS3) - skip
				i++
			}
		} else if s[i] == 0x7f || s[i] == 0x08 {
			// DEL or BS: delete previous character
			if cursor > 0 {
				cursor--
				if cursor < len(buf) {
					buf = append(buf[:cursor], buf[cursor+1:]...)
				}
			}
			i++
		} else if s[i] == 0x01 {
			// Ctrl+A: beginning of line
			cursor = 0
			i++
		} else if s[i] == 0x05 {
			// Ctrl+E: end of line
			cursor = len(buf)
			i++
		} else if s[i] == 0x0b {
			// Ctrl+K: kill to end of line
			if cursor < len(buf) {
				buf = buf[:cursor]
			}
			i++
		} else if s[i] == 0x15 {
			// Ctrl+U: kill to beginning of line
			buf = buf[cursor:]
			cursor = 0
			i++
		} else if s[i] == 0x17 {
			// Ctrl+W: kill previous word
			start := cursor
			for start > 0 && start-1 < len(buf) && buf[start-1] == ' ' {
				start--
			}
			for start > 0 && start-1 < len(buf) && buf[start-1] != ' ' {
				start--
			}
			buf = append(buf[:start], buf[cursor:]...)
			cursor = start
			i++
		} else if s[i] < 0x20 {
			// Skip other control characters
			i++
		} else {
			// Printable character: overwrite at cursor position
			if cursor < len(buf) {
				buf[cursor] = s[i]
			} else {
				buf = append(buf, s[i])
			}
			cursor++
			i++
		}
	}
	return string(buf)
}

// isOnlyEscapeSequences returns true if the string contains only ANSI escape sequences
// and control characters (no printable text).
func isOnlyEscapeSequences(s string) bool {
	i := 0
	for i < len(s) {
		if s[i] == 0x1b {
			// Skip ESC and the sequence that follows
			i++
			if i < len(s) && s[i] == '[' {
				i++
				// Skip parameter bytes and intermediate bytes
				for i < len(s) && s[i] >= 0x20 && s[i] <= 0x3f {
					i++
				}
				// Skip final byte
				if i < len(s) {
					i++
				}
			} else if i < len(s) && isStringSequenceIntroducer(s[i]) {
				i = skipStringSequence(s, i+1)
			} else if i < len(s) {
				i++
			}
		} else if s[i] < 0x20 {
			// Control character
			i++
		} else {
			// Printable character found
			return false
		}
	}
	return true
}

func isStringSequenceIntroducer(b byte) bool {
	return b == ']' || b == 'P' || b == '^' || b == '_' || b == 'X'
}

func skipStringSequence(s string, i int) int {
	for i < len(s) {
		if s[i] == '\a' {
			return i + 1
		}
		if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '\\' {
			return i + 2
		}
		i++
	}
	return i
}
