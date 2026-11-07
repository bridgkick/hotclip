//go:build !windows

package clipboard

import (
	"bytes"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

func TestBuildOSC52(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "standard URL",
			input: "https://github.com/bridgkick/hotclip",
		},
		{
			name:  "URL with query params and fragment",
			input: "https://example.com/search?q=test+query&lang=en#heading-1",
		},
		{
			name:  "empty string",
			input: "",
		},
		{
			name:  "unicode characters",
			input: "https://example.com/wiki/Статья_1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			seq := buildOSC52(tc.input)

			if !strings.HasPrefix(seq, "\x1b]52;c;") {
				t.Fatalf("expected OSC 52 prefix, got %q", seq)
			}
			if !strings.HasSuffix(seq, "\x07") {
				t.Fatalf("expected BEL terminator, got %q", seq)
			}

			payload := strings.TrimPrefix(seq, "\x1b]52;c;")
			payload = strings.TrimSuffix(payload, "\x07")

			decoded, err := base64.StdEncoding.DecodeString(payload)
			if err != nil {
				t.Fatalf("failed to decode base64 payload %q: %v", payload, err)
			}

			if string(decoded) != tc.input {
				t.Errorf("expected decoded payload %q, got %q", tc.input, string(decoded))
			}
		})
	}
}

func TestCopyRichLink_Success(t *testing.T) {
	origStdout := stdout
	defer func() { stdout = origStdout }()

	var buf bytes.Buffer
	stdout = &buf

	title := "GitHub"
	url := "https://github.com/bridgkick/hotclip"

	if err := CopyRichLink(title, url); err != nil {
		t.Fatalf("unexpected error from CopyRichLink: %v", err)
	}

	expected := buildOSC52(url)
	if buf.String() != expected {
		t.Errorf("expected stdout %q, got %q", expected, buf.String())
	}
}

type errWriter struct {
	err error
}

func (e *errWriter) Write(p []byte) (n int, err error) {
	return 0, e.err
}

func TestCopyRichLink_WriterError(t *testing.T) {
	origStdout := stdout
	defer func() { stdout = origStdout }()

	expectedErr := errors.New("simulated stdout write failure")
	stdout = &errWriter{err: expectedErr}

	err := CopyRichLink("Title", "https://example.com")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, expectedErr) && !strings.Contains(err.Error(), expectedErr.Error()) {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}
}
