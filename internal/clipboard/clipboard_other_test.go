//go:build !windows

package clipboard

import (
	"bytes"
	"encoding/base64"
	"errors"
	"strings"
	"sync"
	"testing"
)

type failWriter struct{}

func (f failWriter) Write(p []byte) (n int, err error) {
	return 0, errors.New("simulated write failure")
}

func parseOSC52(t *testing.T, raw string) string {
	t.Helper()
	const prefix = "\x1b]52;c;"
	const suffix = "\x07"

	if !strings.HasPrefix(raw, prefix) {
		t.Fatalf("expected prefix %q, got %q", prefix, raw)
	}
	if !strings.HasSuffix(raw, suffix) {
		t.Fatalf("expected suffix %q, got %q", suffix, raw)
	}

	payload := raw[len(prefix) : len(raw)-len(suffix)]
	decoded, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		t.Fatalf("failed to decode base64 payload %q: %v", payload, err)
	}
	return string(decoded)
}

func TestFormatOSC52(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "standard url",
			input: "https://example.com/docs?page=1&sort=desc#intro",
		},
		{
			name:  "unicode text",
			input: "https://example.com/🚀/слово/测试",
		},
		{
			name:  "simple title",
			input: "My Awesome Bookmark",
		},
		{
			name:  "empty string",
			input: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			seq := FormatOSC52(tc.input)
			got := parseOSC52(t, seq)
			if got != tc.input {
				t.Fatalf("expected %q, got %q", tc.input, got)
			}
		})
	}
}

func TestCopyRichLink_PrefersURL(t *testing.T) {
	var buf bytes.Buffer
	restore := SetOutput(&buf)
	defer restore()

	title := "GitHub - bridgkick/hotclip"
	url := "https://github.com/bridgkick/hotclip"

	if err := CopyRichLink(title, url); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	got := parseOSC52(t, buf.String())
	if got != url {
		t.Fatalf("expected URL %q in clipboard, got %q", url, got)
	}
}

func TestCopyRichLink_FallbackToTitle(t *testing.T) {
	var buf bytes.Buffer
	restore := SetOutput(&buf)
	defer restore()

	title := "Link without URL"
	url := ""

	if err := CopyRichLink(title, url); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	got := parseOSC52(t, buf.String())
	if got != title {
		t.Fatalf("expected title %q in clipboard, got %q", title, got)
	}
}

func TestCopyRichLink_EmptyBoth(t *testing.T) {
	var buf bytes.Buffer
	restore := SetOutput(&buf)
	defer restore()

	if err := CopyRichLink("", ""); err == nil {
		t.Fatal("expected error when both title and url are empty, got nil")
	}

	if buf.Len() != 0 {
		t.Fatalf("expected nothing written to buffer, got %q", buf.String())
	}
}

func TestCopy_Success(t *testing.T) {
	var buf bytes.Buffer
	restore := SetOutput(&buf)
	defer restore()

	input := "https://golang.org"
	if err := Copy(input); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	got := parseOSC52(t, buf.String())
	if got != input {
		t.Fatalf("expected %q, got %q", input, got)
	}
}

func TestCopy_EmptyText(t *testing.T) {
	var buf bytes.Buffer
	restore := SetOutput(&buf)
	defer restore()

	if err := Copy(""); err == nil {
		t.Fatal("expected error on empty text, got nil")
	}

	if buf.Len() != 0 {
		t.Fatalf("expected nothing written to buffer, got %q", buf.String())
	}
}

func TestWriteOSC52_CustomWriter(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteOSC52(&buf, "custom writer test"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	got := parseOSC52(t, buf.String())
	if got != "custom writer test" {
		t.Fatalf("expected 'custom writer test', got %q", got)
	}
}

func TestWriteOSC52_ErrorWriter(t *testing.T) {
	err := WriteOSC52(failWriter{}, "test")
	if err == nil {
		t.Fatal("expected error from failWriter, got nil")
	}
	if !strings.Contains(err.Error(), "simulated write failure") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestSetOutput_Restore(t *testing.T) {
	var buf1 bytes.Buffer
	var buf2 bytes.Buffer

	restore1 := SetOutput(&buf1)
	_ = Copy("to-buf1")
	if parseOSC52(t, buf1.String()) != "to-buf1" {
		t.Fatalf("buf1 did not receive payload")
	}

	restore2 := SetOutput(&buf2)
	_ = Copy("to-buf2")
	if parseOSC52(t, buf2.String()) != "to-buf2" {
		t.Fatalf("buf2 did not receive payload")
	}

	restore2()
	buf1.Reset()
	_ = Copy("back-to-buf1")
	if parseOSC52(t, buf1.String()) != "back-to-buf1" {
		t.Fatalf("buf1 did not receive payload after restore2")
	}

	restore1()
}

func TestConcurrentCopy(t *testing.T) {
	var buf bytes.Buffer
	restore := SetOutput(&buf)
	defer restore()

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			_ = Copy("concurrent test")
			_ = CopyRichLink("Title", "https://example.com")
		}(i)
	}
	wg.Wait()
}
