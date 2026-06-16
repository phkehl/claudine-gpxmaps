// Package render turns a parsed gpx.Model plus a config.Config into a single,
// self-contained HTML document. Leaflet's CSS/JS are embedded from vendored
// files (see assets/) and inlined into every output, so the result depends on
// the network only for map tiles.
package render

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
	"math"
	"time"

	"github.com/flip/gpxmaps/internal/config"
	"github.com/flip/gpxmaps/internal/gpx"
)

// Vendored Leaflet assets, committed to the repo and embedded at build time.
// The minified variants are what we inline; the full variants live alongside
// for debugging (see assets/leaflet.js, assets/leaflet.css).
//
//go:embed assets/leaflet.min.css
var leafletCSS string

//go:embed assets/leaflet.min.js
var leafletJS string

//go:embed template.gohtml
var pageTemplate string

// jsPoint is a sampled point carrying tooltip data (timestamp + velocity).
type jsPoint struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
	T   string  `json:"t"` // formatted timestamp, "" if absent
	V   string  `json:"v"` // formatted velocity, "" if absent
}

type jsStats struct {
	Distance string `json:"distance"`
	ElevGain string `json:"elevGain"`
	Duration string `json:"duration"`
	AvgSpeed string `json:"avgSpeed"`
	HasTime  bool   `json:"hasTime"`
}

type jsTrack struct {
	Name   string       `json:"name"`
	Color  string       `json:"color"`
	Coords [][2]float64 `json:"coords"`
	Points []jsPoint    `json:"points"`
	Stats  jsStats      `json:"stats"`
}

type jsWaypoint struct {
	Lat  float64 `json:"lat"`
	Lon  float64 `json:"lon"`
	Name string  `json:"name"`
	Desc string  `json:"desc"`
}

type jsModel struct {
	Title           string         `json:"title"`
	TileURL         string         `json:"tileUrl"`
	TileAttribution string         `json:"tileAttribution"`
	ShowMarkers     bool           `json:"showMarkers"`
	ShowTooltips    bool           `json:"showTooltips"`
	ShowLegend      bool           `json:"showLegend"`
	ShowStats       bool           `json:"showStats"`
	Bounds          *[2][2]float64 `json:"bounds"`
	Tracks          []jsTrack      `json:"tracks"`
	Waypoints       []jsWaypoint   `json:"waypoints"`
}

// templateData is what the Go template renders. The asset/data fields are typed
// as trusted (template.CSS/JS) because they are vendored or JSON-marshalled by
// us, not user free-text.
type templateData struct {
	Title      string
	LeafletCSS template.CSS
	LeafletJS  template.JS
	DataJSON   template.JS
}

var tmpl = template.Must(template.New("page").Parse(pageTemplate))

// HTML renders the model to a complete HTML document as a byte slice.
func HTML(m gpx.Model, cfg config.Config) ([]byte, error) {
	view := buildView(m, cfg)
	data, err := json.Marshal(view)
	if err != nil {
		return nil, fmt.Errorf("marshal view model: %w", err)
	}
	td := templateData{
		Title:      cfg.Title,
		LeafletCSS: template.CSS(leafletCSS),
		LeafletJS:  template.JS(leafletJS),
		DataJSON:   template.JS(data),
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, td); err != nil {
		return nil, fmt.Errorf("execute template: %w", err)
	}
	return buf.Bytes(), nil
}

// buildView converts the parsed model + config into the JSON-friendly view the
// browser script consumes, assigning colours and applying point sampling.
func buildView(m gpx.Model, cfg config.Config) jsModel {
	out := jsModel{
		Title:           cfg.Title,
		TileURL:         cfg.TileURL,
		TileAttribution: cfg.TileAttribution,
		ShowMarkers:     cfg.ShowMarkers,
		ShowTooltips:    cfg.ShowTooltips,
		ShowLegend:      cfg.ShowLegend,
		ShowStats:       cfg.ShowStats,
	}
	if m.HasBounds {
		out.Bounds = &[2][2]float64{{m.MinLat, m.MinLon}, {m.MaxLat, m.MaxLon}}
	}
	for i, t := range m.Tracks {
		jt := jsTrack{
			Name:   t.Name,
			Color:  cfg.ColorFor(i),
			Coords: coordsOf(t.Points),
			Stats:  formatStats(t.Stats),
		}
		if cfg.ShowTooltips {
			jt.Points = sampledPoints(t.Points, cfg.Sample)
		}
		out.Tracks = append(out.Tracks, jt)
	}
	for _, w := range m.Waypoints {
		out.Waypoints = append(out.Waypoints, jsWaypoint{
			Lat: w.Lat, Lon: w.Lon, Name: w.Name, Desc: w.Desc,
		})
	}
	return out
}

// coordsOf returns the full-resolution polyline coordinates.
func coordsOf(pts []gpx.Point) [][2]float64 {
	out := make([][2]float64, len(pts))
	for i, p := range pts {
		out[i] = [2]float64{p.Lat, p.Lon}
	}
	return out
}

// sampledPoints keeps every Nth point (plus always the first and last) so large
// tracks don't emit thousands of tooltip markers. sample <= 1 keeps all points.
func sampledPoints(pts []gpx.Point, sample int) []jsPoint {
	if len(pts) == 0 {
		return nil
	}
	if sample < 1 {
		sample = 1
	}
	var out []jsPoint
	last := len(pts) - 1
	for i, p := range pts {
		if i != 0 && i != last && i%sample != 0 {
			continue
		}
		jp := jsPoint{Lat: p.Lat, Lon: p.Lon}
		if p.HasTime {
			jp.T = p.Time.Format("2006-01-02 15:04:05")
		}
		if p.HasTime && i != 0 {
			// Velocity is only meaningful once we have a previous timed point.
			jp.V = fmt.Sprintf("%.1f km/h", p.Speed*3.6)
		}
		out = append(out, jp)
	}
	return out
}

// formatStats renders human-readable stat strings (units done here so the
// browser script stays trivial).
func formatStats(s gpx.Stats) jsStats {
	js := jsStats{HasTime: s.HasTime}
	if s.Distance >= 1000 {
		js.Distance = fmt.Sprintf("%.2f km", s.Distance/1000)
	} else {
		js.Distance = fmt.Sprintf("%.0f m", s.Distance)
	}
	js.ElevGain = fmt.Sprintf("%.0f m", math.Round(s.ElevGain))
	if s.HasTime {
		js.Duration = formatDuration(s.Duration)
		js.AvgSpeed = fmt.Sprintf("%.1f km/h", s.AvgSpeed*3.6)
	}
	return js
}

// formatDuration renders a duration as H:MM:SS.
func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	mn := d / time.Minute
	d -= mn * time.Minute
	s := d / time.Second
	return fmt.Sprintf("%d:%02d:%02d", h, mn, s)
}
