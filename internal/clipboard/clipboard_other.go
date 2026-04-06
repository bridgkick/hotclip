//go:build !windows

package clipboard

import "errors"

// CopyRichLink is not supported on non-Windows platforms.
func CopyRichLink(title, url string) error {
	return errors.New("rich clipboard not supported on this platform")
}
