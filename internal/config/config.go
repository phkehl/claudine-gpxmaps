// Package config holds the shared options that drive HTML generation. Both the
// CLI and the GUI build a Config and hand it to the renderer, so the two front
// ends can never diverge in what they produce.
package config

// DefaultTileURL is the OpenStreetMap raster tile template used when the user
// does not override it. Tiles are fetched by the browser at view time and are
// the only part of the output that requires an internet connection.
const DefaultTileURL = "https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"

// DefaultTileAttribution is shown in the map's attribution control. OSM's tile
// usage policy requires attribution.
const DefaultTileAttribution = `&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors`

// DefaultPalette is cycled across tracks so overlapping tracks are
// distinguishable. Colours are chosen to be readable on the OSM basemap.
var DefaultPalette = []string{
	"#e6194b", "#3cb44b", "#4363d8", "#f58231", "#911eb4",
	"#46f0f0", "#f032e6", "#bcf60c", "#fabebe", "#008080",
	"#9a6324", "#800000", "#808000", "#000075", "#e6beff",
}

// Config is the complete set of knobs for one generation run.
type Config struct {
	// Inputs is the list of resolved .gpx file paths to include.
	Inputs []string

	// Output is the path of the .html file to write.
	Output string

	// Title is the document <title> and the on-map heading.
	Title string

	// TileURL is the Leaflet tile-layer URL template.
	TileURL string

	// TileAttribution is the attribution HTML for the tile layer.
	TileAttribution string

	// Sample, when > 1, keeps only every Nth point for markers/tooltips. The
	// full track polyline is always drawn at full resolution; sampling only
	// thins the interactive point markers so large tracks stay responsive. 0
	// or 1 means keep every point.
	Sample int

	// ShowMarkers toggles start/end markers per track.
	ShowMarkers bool

	// ShowTooltips toggles per-point tooltips (timestamp + velocity).
	ShowTooltips bool

	// ShowLegend toggles the track legend / layer-toggle control.
	ShowLegend bool

	// ShowStats toggles distance/elevation/duration/speed in the legend.
	ShowStats bool

	// Palette is the ordered list of track colours to cycle through.
	Palette []string
}

// Default returns a Config populated with sensible defaults. Callers override
// fields from CLI flags or GUI widgets before rendering.
func Default() Config {
	return Config{
		Output:          "tracks.html",
		Title:           "GPX Tracks",
		TileURL:         DefaultTileURL,
		TileAttribution: DefaultTileAttribution,
		Sample:          0,
		ShowMarkers:     true,
		ShowTooltips:    true,
		ShowLegend:      true,
		ShowStats:       true,
		Palette:         DefaultPalette,
	}
}

// ColorFor returns the palette colour for the i-th track, cycling if there are
// more tracks than colours.
func (c Config) ColorFor(i int) string {
	pal := c.Palette
	if len(pal) == 0 {
		pal = DefaultPalette
	}
	return pal[i%len(pal)]
}
