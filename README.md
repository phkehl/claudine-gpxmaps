**Warning -- This is AI generated code**
(purpose: me playing with Claudine...)

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

### Google Maps layers

Pass a Google Maps API key (CLI `--google-key`, or the **Google API key** field in the
GUI) to add Google base layers — **Roadmap** (the default when a key is set), **Satellite**,
**Hybrid** and **Terrain** — via the [Leaflet GoogleMutant](https://github.com/Leaflet/Leaflet.GoogleMutant)
plugin, alongside OpenStreetMap in a layer switcher. With a key the output additionally
loads Google's Maps JavaScript API from `maps.googleapis.com` (Google's API can't be
inlined). The key is embedded in the HTML, so use a key restricted by HTTP referrer.
Without a key, nothing Google-related is included and the file stays fully self-contained.

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

# no -o: the output name is derived from the inputs -> ride1_ride2.html
gpxmaps ride1.gpx ride2.gpx

# generate and immediately view it in your browser
gpxmaps hike.gpx --serve            # http://localhost:8080/

# thin out tooltip markers on huge tracks (keep every 20th point)
gpxmaps bigride.gpx -o big.html --sample 20
```

If `-o`/`--output` is omitted, the output filename is built from the input file base names
(without extension) joined with underscores, e.g. `ride1.gpx ride2.gpx` → `ride1_ride2.html`.

| Flag | Default | Meaning |
|------|---------|---------|
| `-o`, `--output` | derived from inputs | Output HTML file |
| `--title` | `GPX Tracks` | Map title / document title |
| `--tile-url` | OSM | Leaflet tile URL template |
| `--google-key` | — | Google Maps API key; adds Google base layers (default Roadmap) |
| `--sample` | `0` (all) | Keep every Nth point for tooltips (polyline stays full-res) |
| `--markers` | `true` | Start/end markers |
| `--tooltips` | `true` | Per-point time/velocity tooltips |
| `--legend` | `true` | Track legend |
| `--stats` | `true` | Per-track stats in the legend |
| `--serve` | `false` | After generating, serve the file over HTTP |
| `--addr` | `:8080` | Listen address for `--serve` |
| `--gui` | — | Launch the GUI instead |

### GUI

Run `gpxmaps` with no arguments, or `gpxmaps --gui`. Add GPX files, choose the output
path and options, and click **Generate**. The GUI builds the same configuration and calls
the same generation code as the CLI, so output is identical either way.

Manage the file list with **Add GPX…** (a native dialog with Ctrl/Shift multi-select),
**Add folder…** (every `.gpx` in a directory at once), **Clear** (empties the list and
resets the output name), **Sort by path** / **Sort by name**, and **Up** / **Down** to
reorder the selected file.

> Multi-select uses [`ncruces/zenity`](https://github.com/ncruces/zenity): native on
> Windows; on Linux it needs the `zenity` tool installed (`sudo apt-get install zenity`),
> and falls back to Fyne's single-file picker if it's missing.

Tick **Serve over HTTP after generating** (with an optional listen address) to start a
local server on Generate; the window then shows a clickable link to open the map in your
browser. The output filename auto-fills from the input files until you set your own.

All settings (title, tile URL, Google key, sample, serve options and the feature
checkboxes) are **remembered across runs** via Fyne preferences; only the GPX file list
and the output filename reset each time.

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
  - `googlemutant.js` — the Leaflet GoogleMutant plugin (v0.14.0), inlined only when a
    Google API key is supplied.
  - Source: <https://unpkg.com/leaflet@1.9.4/dist/> and
    <https://unpkg.com/leaflet.gridlayer.googlemutant@0.14.0/dist/>
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
geo/speed maths; [`fyne.io/fyne/v2`](https://fyne.io) for the GUI and
[`ncruces/zenity`](https://github.com/ncruces/zenity) for the native multi-select dialog
(both excluded in `nogui` builds).
