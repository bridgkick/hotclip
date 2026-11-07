//go:build !windows

package clipboard

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
)

var (
	outputMu sync.Mutex
	output   io.Writer = os.Stdout
)

// SetOutput sets the destination writer for OSC 52 sequences (defaults to os.Stdout).
// It returns a restore function that resets the writer to its previous value.
func SetOutput(w io.Writer) func() {
	outputMu.Lock()
	defer outputMu.Unlock()
	prev := output
	output = w
	return func() {
		outputMu.Lock()
		defer outputMu.Unlock()
		output = prev
	}
}

// FormatOSC52 returns the OSC 52 ANSI escape sequence for setting the system clipboard ('c').
func FormatOSC52(text string) string {
	encoded := base64.StdEncoding.EncodeToString([]byte(text))
	return fmt.Sprintf("\x1b]52;c;%s\x07", encoded)
}

// WriteOSC52 writes an OSC 52 escape sequence for text to the specified writer.
// If w is nil, it defaults to os.Stdout.
func WriteOSC52(w io.Writer, text string) error {
	if w == nil {
		w = os.Stdout
	}
	_, err := io.WriteString(w, FormatOSC52(text))
	return err
}

// Copy writes the given text to the system clipboard via an OSC 52 ANSI escape sequence to stdout.
func Copy(text string) error {
	if text == "" {
		return errors.New("clipboard: text is empty")
	}
	outputMu.Lock()
	defer outputMu.Unlock()
	return WriteOSC52(output, text)
}

// CopyRichLink copies a link to the clipboard. On non-Windows platforms, native
// rich clipboard formats (e.g., CF_HTML) are unavailable, so it falls back to
// sending the URL (or title if URL is empty) via the OSC 52 ANSI escape sequence to stdout.
func CopyRichLink(title, url string) error {
	text := url
	if text == "" {
		text = title
	}
	if text == "" {
		return errors.New("clipboard: empty title and url")
	}
	return Copy(text)
}
