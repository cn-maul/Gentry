//go:build !windows

package web

import "fmt"

func restartWindows(_, _ string) error {
	return fmt.Errorf("Windows updater is unavailable on this platform")
}
