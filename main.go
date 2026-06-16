// Command gpxmaps converts GPX track files into a single self-contained HTML
// map. The same binary runs in two modes:
//
//	gpxmaps a.gpx b.gpx -o out.html   # CLI / batch
//	gpxmaps                           # GUI (no args), or: gpxmaps --gui
//
// The map is rendered by Leaflet in the browser; this tool only parses GPX and
// emits HTML, so it stays small and dependency-light.
package main

import (
	"fmt"
	"os"

	"github.com/flip/gpxmaps/internal/cli"
	"github.com/flip/gpxmaps/internal/gui"
)

func main() {
	args := os.Args[1:]
	if wantGUI(args) {
		if err := gui.Run(); err != nil {
			fmt.Fprintln(os.Stderr, "gpxmaps:", err)
			os.Exit(1)
		}
		return
	}
	if err := cli.Run(args); err != nil {
		fmt.Fprintln(os.Stderr, "gpxmaps:", err)
		os.Exit(1)
	}
}

// wantGUI reports whether the GUI should launch: when invoked with no arguments
// at all, or when --gui/-gui is passed explicitly.
func wantGUI(args []string) bool {
	if len(args) == 0 {
		return true
	}
	for _, a := range args {
		if a == "--gui" || a == "-gui" {
			return true
		}
	}
	return false
}
