// Package gpx parses one or more .gpx files into a simple, render-ready Model:
// a flat list of tracks, each with its points (lat/lon/ele/time + per-point
// speed) and summary statistics, plus the overall lat/lon bounds used to
// auto-fit the map. Parsing and geo/speed maths are delegated to
// github.com/tkrajina/gpxgo rather than re-implemented.
package gpx

import (
	"fmt"
	"path/filepath"
	"time"

	gpxgo "github.com/tkrajina/gpxgo/gpx"
)

// Point is a single track point. Has* flags distinguish a genuine zero value
// (e.g. elevation exactly 0 m) from missing data in the source GPX.
type Point struct {
	Lat, Lon float64
	Ele      float64
	HasEle   bool
	Time     time.Time
	HasTime  bool
	Speed    float64 // metres/second, computed from the previous point; 0 if unknown
}

// Stats summarises a track.
type Stats struct {
	Distance float64       // total 2D length, metres
	ElevGain float64       // cumulative uphill, metres
	Duration time.Duration // wall-clock span of the track
	AvgSpeed float64       // moving average, metres/second
	HasTime  bool          // whether the track carried timestamps
}

// Track is one rendered path: all segments of a source <trk> concatenated.
type Track struct {
	Name   string
	Points []Point
	Stats  Stats
}

// Model is the complete parsed input for one generation run.
type Model struct {
	Tracks                         []Track
	MinLat, MinLon, MaxLat, MaxLon float64
	HasBounds                      bool
}

// ParseFiles parses every path in order and returns a combined Model. Each
// <trk> in each file becomes one Track. Files with no track points are skipped
// silently; an unparseable file is a hard error so batch runs fail loudly.
func ParseFiles(paths []string) (Model, error) {
	var m Model
	for _, p := range paths {
		g, err := gpxgo.ParseFile(p)
		if err != nil {
			return Model{}, fmt.Errorf("parse %s: %w", p, err)
		}
		base := filepath.Base(p)
		for ti := range g.Tracks {
			trk := &g.Tracks[ti]
			t := buildTrack(trk, base, ti)
			if len(t.Points) == 0 {
				continue
			}
			m.Tracks = append(m.Tracks, t)
			m.extendBounds(t.Points)
		}
	}
	return m, nil
}

// buildTrack flattens a gpxgo track into our Track, computing per-point speed
// and summary stats.
func buildTrack(trk *gpxgo.GPXTrack, fileBase string, idx int) Track {
	name := trk.Name
	if name == "" {
		// Fall back to the file name, disambiguating multiple tracks per file.
		name = fileBase
		if idx > 0 {
			name = fmt.Sprintf("%s #%d", fileBase, idx+1)
		}
	}

	var pts []Point
	var prev *gpxgo.GPXPoint
	for si := range trk.Segments {
		seg := &trk.Segments[si]
		for pi := range seg.Points {
			gp := &seg.Points[pi]
			p := Point{
				Lat:     gp.Latitude,
				Lon:     gp.Longitude,
				HasTime: !gp.Timestamp.IsZero(),
				Time:    gp.Timestamp,
			}
			if gp.Elevation.NotNull() {
				p.Ele = gp.Elevation.Value()
				p.HasEle = true
			}
			if prev != nil {
				p.Speed = gp.SpeedBetween(prev, false)
			}
			pts = append(pts, p)
			prev = gp
		}
	}

	return Track{Name: name, Points: pts, Stats: statsFor(trk)}
}

// statsFor derives summary statistics using gpxgo's helpers.
func statsFor(trk *gpxgo.GPXTrack) Stats {
	md := trk.MovingData()
	var avg float64
	if md.MovingTime > 0 {
		avg = md.MovingDistance / md.MovingTime
	}
	tb := trk.TimeBounds()
	var dur time.Duration
	hasTime := !tb.StartTime.IsZero() && !tb.EndTime.IsZero()
	if hasTime {
		dur = tb.EndTime.Sub(tb.StartTime)
	}
	return Stats{
		Distance: trk.Length2D(),
		ElevGain: trk.UphillDownhill().Uphill,
		Duration: dur,
		AvgSpeed: avg,
		HasTime:  hasTime,
	}
}

// extendBounds grows the model's bounding box to include the given points.
func (m *Model) extendBounds(pts []Point) {
	for _, p := range pts {
		if !m.HasBounds {
			m.MinLat, m.MaxLat = p.Lat, p.Lat
			m.MinLon, m.MaxLon = p.Lon, p.Lon
			m.HasBounds = true
			continue
		}
		if p.Lat < m.MinLat {
			m.MinLat = p.Lat
		}
		if p.Lat > m.MaxLat {
			m.MaxLat = p.Lat
		}
		if p.Lon < m.MinLon {
			m.MinLon = p.Lon
		}
		if p.Lon > m.MaxLon {
			m.MaxLon = p.Lon
		}
	}
}
