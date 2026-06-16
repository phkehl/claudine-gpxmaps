//go:build nogui

// Package gui — headless stub. Built when the `nogui` build tag is set, which
// drops the Fyne dependency (and its CGO/OpenGL requirements) for CLI-only,
// easily cross-compiled binaries. Calling Run here is a clear error.
package gui

import "errors"

// Run reports that this build has no GUI.
func Run() error {
	return errors.New("this build has no GUI (compiled with -tags nogui); pass GPX files for batch mode")
}
