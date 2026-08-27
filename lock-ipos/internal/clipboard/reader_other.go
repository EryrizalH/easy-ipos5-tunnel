//go:build !windows

package clipboard

import "errors"

// ReadText is only available in the Windows installer.
func ReadText() (string, error) {
	return "", errors.New("clipboard hanya tersedia pada Windows")
}
