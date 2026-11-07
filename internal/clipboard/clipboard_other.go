//go:build !windows

package clipboard

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"
)

// stdout is the output destination for OSC 52 escape sequences (defaults to os.Stdout).
// It can be replaced in tests to capture or mock output.
var stdout io.Writer = os.Stdout

// buildOSC52 constructs the OSC 52 ANSI escape sequence to copy text to the clipboard.
// Format: OSC 52 ; Pc ; Pd BEL (\x1b]52;c;<base64>\x07)
// where 'c' selects the system clipboard and Pd is the base64-encoded string.
func buildOSC52(text string) string {
	b64 := base64.StdEncoding.EncodeToString([]byte(text))
	return fmt.Sprintf("\x1b]52;c;%s\x07", b64)
}

// CopyRichLink copies a URL to the system clipboard on non-Windows platforms using
// the OSC 52 ANSI escape sequence sent to stdout.
// Rich text (HTML) clipboard is not supported natively across terminals on non-Windows,
// so the plain URL is copied as a fallback.
func CopyRichLink(title, url string) error {
	seq := buildOSC52(url)
	_, err := fmt.Fprint(stdout, seq)
	return err
}
