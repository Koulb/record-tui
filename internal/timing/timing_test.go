package timing

import (
	"strings"
	"testing"
)

func TestParse_AdvancedFormat(t *testing.T) {
	input := `O 0.009404 16
I 0.500000 1
O 0.001234 1
I 0.200000 4
O 0.001000 5
H 0.000000 0
S 0.100000 0
`
	entries, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(entries) != 7 {
		t.Fatalf("expected 7 entries, got %d", len(entries))
	}

	// Check first entry
	if entries[0].Type != Output || entries[0].ByteCount != 16 {
		t.Errorf("entry 0: got type=%c bytes=%d, want type=O bytes=16", entries[0].Type, entries[0].ByteCount)
	}

	// Check input entry
	if entries[1].Type != Input || entries[1].ByteCount != 1 {
		t.Errorf("entry 1: got type=%c bytes=%d, want type=I bytes=1", entries[1].Type, entries[1].ByteCount)
	}

	// Check header entry
	if entries[5].Type != Header {
		t.Errorf("entry 5: got type=%c, want type=H", entries[5].Type)
	}

	// Check signal entry
	if entries[6].Type != Signal {
		t.Errorf("entry 6: got type=%c, want type=S", entries[6].Type)
	}
}

func TestParse_ClassicFormat(t *testing.T) {
	input := `0.009404 16
0.440731 35
`
	entries, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	// Classic format entries are treated as Output
	if entries[0].Type != Output || entries[0].ByteCount != 16 {
		t.Errorf("entry 0: got type=%c bytes=%d, want type=O bytes=16", entries[0].Type, entries[0].ByteCount)
	}
	if entries[1].Type != Output || entries[1].ByteCount != 35 {
		t.Errorf("entry 1: got type=%c bytes=%d, want type=O bytes=35", entries[1].Type, entries[1].ByteCount)
	}
}

func TestParse_EmptyLines(t *testing.T) {
	input := `
O 0.009404 16

I 0.500000 1

`
	entries, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
}

func TestParse_InvalidLine(t *testing.T) {
	input := `O 0.009404 16
garbage
`
	_, err := Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for invalid line")
	}
}

func TestExtractCommands_SimpleCommand(t *testing.T) {
	// Simulate: some output, then user types "ls\r", then output
	entries := []Entry{
		{Type: Output, Delay: 0.01, ByteCount: 20},  // prompt
		{Type: Input, Delay: 0.5, ByteCount: 1},     // 'l'
		{Type: Input, Delay: 0.1, ByteCount: 1},     // 's'
		{Type: Input, Delay: 0.2, ByteCount: 1},     // '\r'
		{Type: Output, Delay: 0.01, ByteCount: 100}, // command output
	}
	inputContent := []byte("ls\r")

	commands := ExtractCommands(entries, inputContent)
	if len(commands) != 1 {
		t.Fatalf("expected 1 command, got %d", len(commands))
	}
	if commands[0].Text != "ls" {
		t.Errorf("got text %q, want %q", commands[0].Text, "ls")
	}
	if commands[0].OutputByteOffset != 20 {
		t.Errorf("got offset %d, want 20", commands[0].OutputByteOffset)
	}
}

func TestExtractCommands_MultipleCommands(t *testing.T) {
	entries := []Entry{
		{Type: Output, Delay: 0.01, ByteCount: 20},  // prompt
		{Type: Input, Delay: 0.5, ByteCount: 3},     // "ls\r"
		{Type: Output, Delay: 0.01, ByteCount: 50},  // ls output
		{Type: Output, Delay: 0.01, ByteCount: 20},  // next prompt
		{Type: Input, Delay: 1.0, ByteCount: 9},     // "npm test\r"
		{Type: Output, Delay: 0.01, ByteCount: 200}, // npm output
	}
	inputContent := []byte("ls\rnpm test\r")

	commands := ExtractCommands(entries, inputContent)
	if len(commands) != 2 {
		t.Fatalf("expected 2 commands, got %d", len(commands))
	}
	if commands[0].Text != "ls" {
		t.Errorf("cmd 0: got %q, want %q", commands[0].Text, "ls")
	}
	if commands[0].OutputByteOffset != 20 {
		t.Errorf("cmd 0: got offset %d, want 20", commands[0].OutputByteOffset)
	}
	if commands[1].Text != "npm test" {
		t.Errorf("cmd 1: got %q, want %q", commands[1].Text, "npm test")
	}
	if commands[1].OutputByteOffset != 90 {
		t.Errorf("cmd 1: got offset %d, want 90", commands[1].OutputByteOffset)
	}
}

func TestExtractCommands_FilterControlChars(t *testing.T) {
	// Single Ctrl-C followed by \r should be filtered
	entries := []Entry{
		{Type: Output, Delay: 0.01, ByteCount: 20},
		{Type: Input, Delay: 0.5, ByteCount: 2}, // Ctrl-C + \r
		{Type: Output, Delay: 0.01, ByteCount: 10},
	}
	inputContent := []byte{0x03, '\r'}

	commands := ExtractCommands(entries, inputContent)
	if len(commands) != 0 {
		t.Fatalf("expected 0 commands (Ctrl-C filtered), got %d", len(commands))
	}
}

func TestExtractCommands_FilterArrowKeys(t *testing.T) {
	// Arrow key sequence ESC[A followed by \r
	entries := []Entry{
		{Type: Output, Delay: 0.01, ByteCount: 20},
		{Type: Input, Delay: 0.5, ByteCount: 4}, // ESC[A + \r
		{Type: Output, Delay: 0.01, ByteCount: 10},
	}
	inputContent := []byte("\x1b[A\r")

	commands := ExtractCommands(entries, inputContent)
	if len(commands) != 0 {
		t.Fatalf("expected 0 commands (arrow key filtered), got %d", len(commands))
	}
}

func TestExtractCommands_FilterTerminalOSCReplies(t *testing.T) {
	inputContent := []byte("\x1b]10;rgb:cdcd/d6d6/f4f4\x1b\\\x1b]11;rgb:1e1e/1e1e/2e2e\x1b\\\r")
	entries := []Entry{
		{Type: Output, Delay: 0.01, ByteCount: 20},
		{Type: Input, Delay: 0.5, ByteCount: len(inputContent)},
	}

	commands := ExtractCommands(entries, inputContent)
	if len(commands) != 0 {
		t.Fatalf("expected 0 commands (OSC replies filtered), got %d: %#v", len(commands), commands)
	}
}

func TestExtractCommands_NoTerminator(t *testing.T) {
	// Input without \r or \n is not a command (e.g., partial typing)
	entries := []Entry{
		{Type: Output, Delay: 0.01, ByteCount: 20},
		{Type: Input, Delay: 0.5, ByteCount: 2},
	}
	inputContent := []byte("ls")

	commands := ExtractCommands(entries, inputContent)
	if len(commands) != 0 {
		t.Fatalf("expected 0 commands (no terminator), got %d", len(commands))
	}
}

func TestExtractCommands_InterleavedIO(t *testing.T) {
	// Real-world pattern: each keystroke is I(1 byte) followed by O(1 byte echo)
	entries := []Entry{
		{Type: Output, Delay: 0.01, ByteCount: 75},   // prompt
		{Type: Input, Delay: 4.0, ByteCount: 1},      // 'l'
		{Type: Output, Delay: 0.001, ByteCount: 1},   // echo 'l'
		{Type: Input, Delay: 0.1, ByteCount: 1},      // 's'
		{Type: Output, Delay: 0.001, ByteCount: 1},   // echo 's'
		{Type: Input, Delay: 0.2, ByteCount: 1},      // '\r'
		{Type: Output, Delay: 0.001, ByteCount: 100}, // command output
		{Type: Input, Delay: 1.0, ByteCount: 1},      // 'p'
		{Type: Output, Delay: 0.001, ByteCount: 1},   // echo 'p'
		{Type: Input, Delay: 0.1, ByteCount: 1},      // 'w'
		{Type: Output, Delay: 0.001, ByteCount: 1},   // echo 'w'
		{Type: Input, Delay: 0.1, ByteCount: 1},      // 'd'
		{Type: Output, Delay: 0.001, ByteCount: 1},   // echo 'd'
		{Type: Input, Delay: 0.2, ByteCount: 1},      // '\r'
		{Type: Output, Delay: 0.001, ByteCount: 50},  // pwd output
	}
	inputContent := []byte("ls\rpwd\r")

	commands := ExtractCommands(entries, inputContent)
	if len(commands) != 2 {
		t.Fatalf("expected 2 commands, got %d", len(commands))
	}
	if commands[0].Text != "ls" {
		t.Errorf("cmd 0: got %q, want %q", commands[0].Text, "ls")
	}
	if commands[1].Text != "pwd" {
		t.Errorf("cmd 1: got %q, want %q", commands[1].Text, "pwd")
	}
}

func TestExtractCommands_EmptyEnter(t *testing.T) {
	// Just pressing Enter (empty command)
	entries := []Entry{
		{Type: Output, Delay: 0.01, ByteCount: 20},
		{Type: Input, Delay: 0.5, ByteCount: 1},
		{Type: Output, Delay: 0.01, ByteCount: 10},
	}
	inputContent := []byte("\r")

	commands := ExtractCommands(entries, inputContent)
	if len(commands) != 0 {
		t.Fatalf("expected 0 commands (empty enter), got %d", len(commands))
	}
}
