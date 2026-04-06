//go:build windows

package clipboard

import (
	"fmt"
	"html"
	"runtime"
	"syscall"
	"time"
	"unsafe"
)

const (
	cfUnicodetext = 13
	gmemMoveable  = 0x0002
)

var (
	user32           = syscall.MustLoadDLL("user32")
	openClipboard    = user32.MustFindProc("OpenClipboard")
	closeClipboard   = user32.MustFindProc("CloseClipboard")
	emptyClipboard   = user32.MustFindProc("EmptyClipboard")
	setClipboardData = user32.MustFindProc("SetClipboardData")
	registerFormat   = user32.MustFindProc("RegisterClipboardFormatW")

	kernel32     = syscall.NewLazyDLL("kernel32")
	globalAlloc  = kernel32.NewProc("GlobalAlloc")
	globalFree   = kernel32.NewProc("GlobalFree")
	globalLock   = kernel32.NewProc("GlobalLock")
	globalUnlock = kernel32.NewProc("GlobalUnlock")
	lstrcpy      = kernel32.NewProc("lstrcpyW")
)

// waitOpenClipboard opens the clipboard, retrying for up to one second.
func waitOpenClipboard() error {
	limit := time.Now().Add(time.Second)
	for time.Now().Before(limit) {
		r, _, err := openClipboard.Call(0)
		if r != 0 {
			return nil
		}
		_ = err
		time.Sleep(time.Millisecond)
	}
	return fmt.Errorf("clipboard: timeout opening clipboard")
}

// setUTF16 writes a UTF-16 string to the clipboard in the given format.
func setUTF16(format uintptr, text string) error {
	data := syscall.StringToUTF16(text)
	size := uintptr(len(data) * int(unsafe.Sizeof(data[0])))

	h, _, err := globalAlloc.Call(gmemMoveable, size)
	if h == 0 {
		return fmt.Errorf("clipboard: GlobalAlloc: %w", err)
	}
	defer func() {
		if h != 0 {
			globalFree.Call(h)
		}
	}()

	l, _, err := globalLock.Call(h)
	if l == 0 {
		return fmt.Errorf("clipboard: GlobalLock: %w", err)
	}

	r, _, err := lstrcpy.Call(l, uintptr(unsafe.Pointer(&data[0])))
	if r == 0 {
		return fmt.Errorf("clipboard: lstrcpy: %w", err)
	}

	globalUnlock.Call(h)

	r, _, err = setClipboardData.Call(format, h)
	if r == 0 {
		return fmt.Errorf("clipboard: SetClipboardData: %w", err)
	}
	h = 0 // ownership transferred
	return nil
}

// setRawBytes writes raw bytes to the clipboard in the given format.
func setRawBytes(format uintptr, data []byte) error {
	size := uintptr(len(data))

	h, _, err := globalAlloc.Call(gmemMoveable, size)
	if h == 0 {
		return fmt.Errorf("clipboard: GlobalAlloc: %w", err)
	}
	defer func() {
		if h != 0 {
			globalFree.Call(h)
		}
	}()

	l, _, err := globalLock.Call(h)
	if l == 0 {
		return fmt.Errorf("clipboard: GlobalLock: %w", err)
	}

	dst := unsafe.Slice((*byte)(unsafe.Pointer(l)), len(data))
	copy(dst, data)

	globalUnlock.Call(h)

	r, _, err := setClipboardData.Call(format, h)
	if r == 0 {
		return fmt.Errorf("clipboard: SetClipboardData: %w", err)
	}
	h = 0 // ownership transferred
	return nil
}

// buildCFHTML constructs a CF_HTML clipboard payload for a hyperlink.
func buildCFHTML(title, url string) []byte {
	safeTitle := html.EscapeString(title)
	safeURL := html.EscapeString(url)
	fragment := fmt.Sprintf(`<a href="%s">%s</a>`, safeURL, safeTitle)

	// The header has fixed-width offset fields (10 digits each).
	// Build a template header to measure its byte length.
	headerTmpl := "Version:0.9\r\nStartHTML:%010d\r\nEndHTML:%010d\r\nStartFragment:%010d\r\nEndFragment:%010d\r\n"
	header := fmt.Sprintf(headerTmpl, 0, 0, 0, 0)
	headerLen := len(header)

	prefix := "<html><body>\r\n<!--StartFragment-->"
	suffix := "<!--EndFragment-->\r\n</body></html>"

	startHTML := headerLen
	startFragment := headerLen + len(prefix)
	endFragment := startFragment + len(fragment)
	endHTML := endFragment + len(suffix)

	header = fmt.Sprintf(headerTmpl, startHTML, endHTML, startFragment, endFragment)

	buf := make([]byte, 0, endHTML+1)
	buf = append(buf, header...)
	buf = append(buf, prefix...)
	buf = append(buf, fragment...)
	buf = append(buf, suffix...)
	buf = append(buf, 0) // null terminator
	return buf
}

// CopyRichLink copies a URL to the clipboard in both plain text and HTML formats.
// Plain text paste yields the raw URL; rich text paste yields a clickable hyperlink.
func CopyRichLink(title, url string) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := waitOpenClipboard(); err != nil {
		return err
	}
	defer closeClipboard.Call()

	r, _, err := emptyClipboard.Call(0)
	if r == 0 {
		return fmt.Errorf("clipboard: EmptyClipboard: %w", err)
	}

	// Register CF_HTML format.
	formatName, _ := syscall.UTF16PtrFromString("HTML Format")
	htmlFmt, _, _ := registerFormat.Call(uintptr(unsafe.Pointer(formatName)))

	// Set plain text (URL only).
	if err := setUTF16(cfUnicodetext, url); err != nil {
		return err
	}

	// Set HTML (clickable hyperlink).
	if htmlFmt != 0 {
		if err := setRawBytes(htmlFmt, buildCFHTML(title, url)); err != nil {
			return err
		}
	}

	return nil
}
