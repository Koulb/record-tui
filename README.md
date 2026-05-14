# record-tui

Record terminal sessions and convert them to beautiful standalone HTML with xterm.js rendering.

**See [example output](https://record-tui.netlify.app)** - recorded `claude` session with ANSI colors and terminal emulation.

## How I use it

`claude` on my computer is not the claude cli, but this wrapper script

```bash
#/bin/bash

set -xefo pipefail

record-tui ~/.local/bin/claude $*
```

so all claude conversations are recorded. if there were any good ones i'd like to share, i can share the pdf or html.

This is also used as a library in https://github.com/choonkeat/swe-swe to power the view "recordings" feature. 
i.e. all sessions in swe-swe are recorded, good ones can be kept easily.


## Requirements

**macOS only** — requires `script` and `open` commands (both built into macOS).

## Installation

```bash
make install  # Compile and install to ~/bin/record-tui
```

Optional: PDF export support

```bash
make install-pdf-tool  # Enable automatic PDF generation
```

## Usage

Record a terminal session:

```bash
# Interactive shell
record-tui

# Record a specific command
record-tui claude
record-tui npm test
record-tui sh -c "ls -la"

# Record with animated playback controls
record-tui -animated codex
```

Files are saved to `~/.record-tui/YYYYMMDD-HHMMSS/`:
- `session.log` — raw session file
- `session.log.html` — standalone HTML with ANSI colors and terminal emulation
- `session.log.animated.html` — animated HTML replay with play/pause, speed, scrubber, and markers (when `-animated` is used)
- `session.log.pdf` — printable PDF (A4 landscape, requires `make install-pdf-tool`)

Recording stops when:
- Command exits (if you specified one)
- You press **Ctrl-D** or type `exit` (if running interactive shell)

The directory opens automatically in Finder when recording completes (unless over SSH).

## Features

- ✅ **One command**: Records and converts automatically
- ✅ **Standalone HTML**: No external dependencies, works offline after generation
- ✅ **Animated HTML**: Optional replay mode with play/pause, timeline scrubbing, and speed controls
- ✅ **PDF export**: Generate printable PDFs via `make install-pdf-tool` (optional)
- ✅ **Colors preserved**: Full ANSI color support (8 colors + bright variants)
- ✅ **Auto-open**: Directory opens in Finder on completion
- ✅ **Single binary**: No Node.js, npm, or external dependencies (except optional PDF tool)
- ✅ **Instant**: Fast recording and HTML generation (~2s more for PDF)

## What Gets Recorded

Everything typed in the terminal session:
- ✅ Command output with colors
- ✅ Interactive commands (runs fully)
- ✅ Text and code with formatting
- ⚠️ Full-screen TUIs (vim, htop, etc.) display as control sequences

## Examples

```bash
# View all recordings
ls ~/.record-tui/

# Open a specific session
open ~/.record-tui/20251231-144256/session.log.html

# Convert an existing timing-enabled recording to animated HTML
record-tui -convert ~/.record-tui/20251231-144256/session.log -animated

# Convert with authored speed/pause cues
record-tui -convert session.log -animated -cues cues.json
```

## Library Usage

record-tui can be imported as a Go library for generating HTML from terminal recordings:

```bash
go get github.com/choonkeat/record-tui@latest
```

### Embedded Mode (Default)

Best for small recordings (< 1MB). The HTML is self-contained and works offline:

```go
package main

import (
    "os"

    "github.com/choonkeat/record-tui/playback"
)

func main() {
    // Read session.log content (from macOS or Linux `script` command)
    content, _ := os.ReadFile("session.log")

    // Strip script header/footer metadata
    cleaned := playback.StripMetadata(string(content))

    // Create playback frames (single frame for static display)
    frames := []playback.Frame{
        {Timestamp: 0, Content: cleaned},
    }

    // Generate standalone HTML with xterm.js
    html, _ := playback.RenderHTML(frames, playback.Options{
        Title: "My Session", // Optional: custom page title
    })
    os.WriteFile("output.html", []byte(html), 0644)
}
```

### Streaming Mode

Best for large recordings (multi-megabyte). The HTML fetches session data separately and renders progressively:

```go
package main

import (
    "os"

    "github.com/choonkeat/record-tui/playback"
)

func main() {
    // Generate lightweight HTML shell that streams from a URL
    html, _ := playback.RenderStreamingHTML(playback.StreamingOptions{
        Title:   "My Session",
        DataURL: "./session.log", // URL to fetch session data from
    })
    os.WriteFile("output.html", []byte(html), 0644)

    // Serve both files via HTTP:
    // - output.html at /
    // - session.log at /session.log
}
```

**Note:** Streaming mode requires HTTP(S) — won't work with `file://` URLs. The JavaScript handles header/footer stripping and ANSI processing on the fly.

### When to use each mode

| Mode | File Size | Offline Support | Requires Server |
|------|-----------|-----------------|-----------------|
| Embedded (`RenderHTML`) | < 1MB | Yes | No |
| Streaming (`RenderStreamingHTML`) | Any | No | Yes |

Supports both macOS and Linux `script` command output formats.

### Animated Mode

Animated mode consumes timing data and emits output chunks with timestamps:

```go
package main

import (
    "bytes"
    "os"

    "github.com/choonkeat/record-tui/playback"
)

func main() {
    session, _ := os.ReadFile("session.log")
    timing, _ := os.ReadFile("session.timing")

    frames, _ := playback.BuildAnimationFrames(
        bytes.NewReader(timing),
        session,
        playback.AnimationBuildOptions{MaxFPS: 30},
    )
    html, _ := playback.RenderAnimatedHTML(frames, playback.AnimationOptions{
        Title: "Codex Session",
    })
    os.WriteFile("session.animated.html", []byte(html), 0644)
}
```

The CLI records timing itself for `record-tui -animated ...`; it also supports converting pre-existing `session.log` + `session.timing` files.

Cue files let you speed through dull spans or pause at key moments:

```json
[
  { "timestamp": 12.5, "speed": 5, "label": "Speed through install" },
  { "timestamp": 48.0, "pause": 4, "label": "Focus on failing test" },
  { "timestamp": 62.0, "speed": 1, "label": "Back to normal speed" }
]
```

## Development

### Build

```bash
make build   # Compile binary
make test    # Run tests
make clean   # Remove build artifacts
```

### Project Structure

```
cmd/record-tui/    # CLI entry point
playback/          # Public API (import this package)
internal/
├── record/        # Recording and conversion logic
├── html/          # HTML generation with xterm.js
└── session/       # Session log parsing and cleanup
```

## License

MIT
