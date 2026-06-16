// Package cli implements the command-line / batch front end. It parses flags
// into a config.Config, resolves the positional inputs (files, globs and
// directories), then runs the shared parse+render pipeline.
package cli

import (
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/flip/gpxmaps/internal/config"
	"github.com/flip/gpxmaps/internal/gpx"
	"github.com/flip/gpxmaps/internal/render"
)

// Run executes the CLI with the given args (os.Args[1:]). It returns a non-nil
// error on any failure so main can set the exit code.
func Run(args []string) error {
	cfg := config.Default()
	cfg.Output = "" // when left empty, derived from the input file names below

	var serve bool
	var addr string

	fs := flag.NewFlagSet("gpxmaps", flag.ContinueOnError)
	fs.StringVar(&cfg.Output, "o", "", "output HTML file (default: derived from input names)")
	fs.StringVar(&cfg.Output, "output", "", "output HTML file (alias of -o)")
	fs.StringVar(&cfg.Title, "title", cfg.Title, "map title")
	fs.StringVar(&cfg.TileURL, "tile-url", cfg.TileURL, "Leaflet tile URL template")
	fs.StringVar(&cfg.GoogleAPIKey, "google-key", cfg.GoogleAPIKey, "Google Maps API key (enables Google base layers, default Roadmap)")
	fs.IntVar(&cfg.Sample, "sample", cfg.Sample, "keep every Nth point for tooltips (0/1 = all)")
	fs.BoolVar(&cfg.ShowMarkers, "markers", cfg.ShowMarkers, "show start/end markers")
	fs.BoolVar(&cfg.ShowTooltips, "tooltips", cfg.ShowTooltips, "show per-point time/velocity tooltips")
	fs.BoolVar(&cfg.ShowLegend, "legend", cfg.ShowLegend, "show track legend")
	fs.BoolVar(&cfg.ShowStats, "stats", cfg.ShowStats, "show per-track stats in legend")
	fs.BoolVar(&serve, "serve", false, "after generating, serve the file over HTTP")
	fs.StringVar(&addr, "addr", ":8080", "listen address for --serve")
	// --gui is consumed by main for mode dispatch; declare it so flag parsing
	// of a CLI invocation that also passes inputs doesn't error.
	fs.Bool("gui", false, "launch the GUI instead of batch mode")

	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: gpxmaps [flags] <file-or-dir.gpx> [more...]\n\n")
		fmt.Fprintf(fs.Output(), "Generates a single self-contained HTML map from GPX tracks/routes/waypoints.\n")
		fmt.Fprintf(fs.Output(), "With no arguments (or --gui) the configuration GUI opens instead.\n")
		fmt.Fprintf(fs.Output(), "Use --serve to view the result in a browser over HTTP.\n\n")
		fmt.Fprintf(fs.Output(), "Flags:\n")
		fs.PrintDefaults()
	}

	// Go's flag package stops at the first non-flag argument. Parse in a loop so
	// flags and file arguments may be freely interleaved (e.g. both
	// `gpxmaps -o x.html a.gpx` and `gpxmaps a.gpx -o x.html` work).
	var positional []string
	rest := args
	for len(rest) > 0 {
		if err := fs.Parse(rest); err != nil {
			return err
		}
		na := fs.Args()
		if len(na) == 0 {
			break
		}
		positional = append(positional, na[0])
		rest = na[1:]
	}

	inputs, err := resolveInputs(positional)
	if err != nil {
		return err
	}
	if len(inputs) == 0 {
		fs.Usage()
		return fmt.Errorf("no .gpx input files found")
	}
	cfg.Inputs = inputs
	if cfg.Output == "" {
		cfg.Output = config.OutputName(inputs)
	}

	if err := Generate(cfg); err != nil {
		return err
	}
	fmt.Printf("Wrote %s (%d input file(s))\n", cfg.Output, len(inputs))

	if serve {
		return serveFile(addr, cfg.Output)
	}
	return nil
}

// ServeURL returns the human-facing URL for a listen address, e.g. ":8080" ->
// "http://localhost:8080/".
func ServeURL(addr string) string {
	if strings.HasPrefix(addr, ":") {
		return "http://localhost" + addr + "/"
	}
	return "http://" + addr + "/"
}

// fileServer returns a handler that serves the file returned by currentFile()
// for every request (so the served content tracks the latest generation).
func fileServer(currentFile func() string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, currentFile())
	})
	return mux
}

// StartServer binds addr and serves currentFile() in a background goroutine,
// returning the URL to open. It fails fast if the address can't be bound. Used
// by the GUI, which must not block; the CLI uses the blocking serveFile.
func StartServer(addr string, currentFile func() string) (string, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return "", err
	}
	go http.Serve(ln, fileServer(currentFile))
	return ServeURL(addr), nil
}

// serveFile serves the generated HTML over HTTP until the process is
// interrupted. Every path returns the same file, so http://host:port/ just
// works; map tiles are still fetched by the browser directly from the tile
// server.
func serveFile(addr, file string) error {
	fmt.Printf("Serving %s at %s (Ctrl+C to stop)\n", file, ServeURL(addr))
	return http.ListenAndServe(addr, fileServer(func() string { return file }))
}

// Generate runs the shared pipeline: parse the configured inputs, render HTML,
// write the output file. Exported so the GUI can call the exact same path.
func Generate(cfg config.Config) error {
	m, err := gpx.ParseFiles(cfg.Inputs)
	if err != nil {
		return err
	}
	if len(m.Tracks) == 0 && len(m.Waypoints) == 0 {
		return fmt.Errorf("no tracks, routes, or waypoints found in the input file(s)")
	}
	html, err := render.HTML(m, cfg)
	if err != nil {
		return err
	}
	if err := os.WriteFile(cfg.Output, html, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", cfg.Output, err)
	}
	return nil
}

// resolveInputs expands the positional arguments into a de-duplicated, sorted
// list of .gpx files. Each argument may be a file, a glob pattern, or a
// directory (scanned non-recursively for *.gpx).
func resolveInputs(args []string) ([]string, error) {
	seen := map[string]struct{}{}
	var out []string
	add := func(p string) {
		if _, ok := seen[p]; ok {
			return
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}

	for _, a := range args {
		info, statErr := os.Stat(a)
		switch {
		case statErr == nil && info.IsDir():
			matches, _ := filepath.Glob(filepath.Join(a, "*.gpx"))
			for _, m := range matches {
				add(m)
			}
		case statErr == nil:
			add(a)
		default:
			// Not an existing path: treat as a glob pattern.
			matches, err := filepath.Glob(a)
			if err != nil {
				return nil, fmt.Errorf("bad pattern %q: %w", a, err)
			}
			for _, m := range matches {
				if strings.EqualFold(filepath.Ext(m), ".gpx") {
					add(m)
				}
			}
		}
	}
	sort.Strings(out)
	return out, nil
}
