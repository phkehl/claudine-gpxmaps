# gpxmaps

Turn GPX track files into a **single, self-contained HTML map** you can open in any
browser. One small Go binary does both:

- **CLI / batch** — `gpxmaps ride1.gpx ride2.gpx -o trips.html`
- **GUI** — run with no arguments (or `--gui`) for a simple configuration window.

The map is drawn by [Leaflet](https://leafletjs.com) in the browser; this tool only parses
GPX and emits HTML, so it stays tiny and ships as a single binary with no runtime to
install.

---

## What it produces

It accepts GPX **tracks** (`<trk>`), **routes** (`<rte>`) and standalone
**waypoints** (`<wpt>`), in both GPX 1.0 and 1.1 — so track-only, route-only and
waypoint-only files all work. Tracks and routes render as paths; waypoints render as
labelled markers.

A standalone `.html` containing your tracks on an OpenStreetMap basemap, with:

- **Auto-fit** — the map zooms/pans to frame all tracks on load.
- **Per-track colors** — a palette is cycled so overlapping tracks are distinguishable.
- **Legend** — lists tracks with color swatches and checkboxes to toggle each on/off.
- **Stats** — per-track distance, elevation gain, duration and average speed.
- **Start/end markers** — green start, red end per track.
- **Point tooltips** — hover a point to see its timestamp and velocity.

The output is fully self-contained: Leaflet's CSS/JS are embedded inside the file. The
**only** thing fetched from the internet when you open it is the map *tiles* (the basemap
imagery). Everything else works offline.

---

## Usage

### CLI

```
gpxmaps [flags] <file-or-dir.gpx> [more...]
```

Inputs may be files, glob patterns, or directories (scanned non-recursively for `*.gpx`).
Flags and files may appear in any order.

```
# one file
gpxmaps hike.gpx -o hike.html

# many files + a custom title
gpxmaps tracks/*.gpx -o all.html --title "Summer 2024"

# a whole directory
gpxmaps ./gpx-archive -o archive.html

# thin out tooltip markers on huge tracks (keep every 20th point)
gpxmaps bigride.gpx -o big.html --sample 20
```

| Flag | Default | Meaning |
|------|---------|---------|
| `-o`, `--output` | `tracks.html` | Output HTML file |
| `--title` | `GPX Tracks` | Map title / document title |
| `--tile-url` | OSM | Leaflet tile URL template |
| `--sample` | `0` (all) | Keep every Nth point for tooltips (polyline stays full-res) |
| `--markers` | `true` | Start/end markers |
| `--tooltips` | `true` | Per-point time/velocity tooltips |
| `--legend` | `true` | Track legend |
| `--stats` | `true` | Per-track stats in the legend |
| `--gui` | — | Launch the GUI instead |

### GUI

Run `gpxmaps` with no arguments, or `gpxmaps --gui`. Add GPX files, choose the output
path and options, and click **Generate**. The GUI builds the same configuration and calls
the same generation code as the CLI, so output is identical either way.

---

## Building

Requires [Go](https://go.dev) 1.24+.

### Full binary (CLI + GUI)

The GUI uses [Fyne](https://fyne.io), which needs CGO and the system OpenGL/X11
development libraries.

```
# Debian/Ubuntu build dependencies (once):
sudo apt-get install -y libgl1-mesa-dev xorg-dev

go build -o gpxmaps .
```

Cross-compiling the GUI binary to Windows from Linux needs a C cross-toolchain; the
easiest path is [`fyne-cross`](https://github.com/fyne-io/fyne-cross).

### CLI-only binary (no GUI, no CGO)

For servers, batch pipelines, or trivial cross-compilation, build with the `nogui` tag.
This drops the Fyne dependency entirely — pure Go, no C toolchain, no system libraries:

```
go build -tags nogui -o gpxmaps .

# cross-compile to Windows from anywhere:
GOOS=windows GOARCH=amd64 go build -tags nogui -o gpxmaps.exe .
```

In a `nogui` build, passing `--gui` (or running with no arguments) prints a clear error.

### Tests

```
go test ./...
```

---

## Design notes & key decisions

- **Map library: Leaflet** (raster OSM tiles, no API key) — the simplest way to plot
  tracks as polylines with auto-fit, markers and tooltips.
- **Always inline, single file** — every run embeds Leaflet's CSS/JS into the output. There
  is no CDN mode; the result is portable (tiles aside). The default tile basemap is OSM and
  can be changed with `--tile-url`.
- **Vendored libraries** — Leaflet **1.9.4** is committed to the repo under
  `internal/render/assets/` and embedded via `go:embed`, so builds are reproducible and
  never fetch anything at compile time:
  - `leaflet.min.js` / `leaflet.min.css` — inlined into the output.
  - `leaflet.js` (the unminified `leaflet-src.js`) — kept for reference/debugging.
  - (Leaflet ships a single distributed `leaflet.css`, so the min and full CSS are
    identical.)
  - Source: <https://unpkg.com/leaflet@1.9.4/dist/>
- **One pipeline for both modes** — CLI and GUI both populate a `config.Config` and call
  `cli.Generate`, which parses inputs (`internal/gpx`) and renders HTML
  (`internal/render`). The GUI is a thin shell; the two front ends can't diverge.
- **`nogui` build tag** — isolates the Fyne/CGO dependency so a pure-Go, easily
  cross-compiled CLI-only binary is always available.

### Layout

```
main.go                         mode dispatch (no args/--gui → GUI, else CLI)
internal/config/                shared Config struct, defaults, color palette
internal/gpx/                   parse .gpx → Model (tracks, routes, waypoints) via gpxgo
internal/render/                Model + Config → self-contained HTML
internal/render/assets/         vendored, committed Leaflet (min + full)
internal/render/template.gohtml HTML template + Leaflet bootstrap JS
internal/cli/                   flag parsing, input resolution, Generate()
internal/gui/                   Fyne form (gui.go) + nogui stub (gui_nogui.go)
testdata/sample.gpx             test fixture
```

Dependencies: [`tkrajina/gpxgo`](https://github.com/tkrajina/gpxgo) for GPX parsing and
geo/speed maths; [`fyne.io/fyne/v2`](https://fyne.io) for the GUI (excluded in `nogui`
builds).
