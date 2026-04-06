//go:build !windows
// +build !windows

package appcore

func detectRecentAuthFailure(serviceName string) (bool, error) {
	return false, nil
}
