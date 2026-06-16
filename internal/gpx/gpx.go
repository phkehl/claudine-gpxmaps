// Package gpx parses one or more .gpx files into a simple, render-ready Model:
// a flat list of paths (tracks AND routes), each with its points
// (lat/lon/ele/time + per-point speed) and summary statistics, plus any
// standalone waypoints and the overall lat/lon bounds used to auto-fit the map.
// Parsing and geo/speed maths are delegated to github.com/tkrajina/gpxgo rather
// than re-implemented.
package gpx

import (
	"fmt"
	"path/filepath"
	"time"

	gpxgo "github.com/tkrajina/gpxgo/gpx"
)

// Point is a single path point. Has* flags distinguish a genuine zero value
// (e.g. elevation exactly 0 m) from missing data in the source GPX.
type Point struct {
	Lat, Lon float64
	Ele      float64
	HasEle   bool
	Time     time.Time
	HasTime  bool
	Speed    float64 // metres/second, computed from the previous point; 0 if unknown
}

// Stats summarises a path.
type Stats struct {
	Distance float64       // total 2D length, metres
	ElevGain float64       // cumulative uphill, metres
	Duration time.Duration // wall-clock span (only when timestamps increase)
	AvgSpeed float64       // average speed over the duration, metres/second
	HasTime  bool          // whether the path carried usable timestamps
}

// Track is one rendered path. It comes from either a GPX <trk> (its segments
// concatenated) or a GPX <rte> route; both are just an ordered list of points
// as far as the map is concerned.
type Track struct {
	Name   string
	Points []Point
	Stats  Stats
}

// Waypoint is a standalone point of interest (GPX <wpt>), rendered as a marker
// rather than part of a path.
type Waypoint struct {
	Lat, Lon float64
	Name     string
	Desc     string
}

// Model is the complete parsed input for one generation run.
type Model struct {
	Tracks                         []Track
	Waypoints                      []Waypoint
	MinLat, MinLon, MaxLat, MaxLon float64
	HasBounds                      bool
}

// ParseFiles parses every path in order and returns a combined Model. Every
// <trk> and every <rte> becomes a Track, and every <wpt> becomes a Waypoint, so
// the tool handles track-only, route-only and waypoint-only files alike. Empty
// paths are skipped silently; an unparseable file is a hard error so batch runs
// fail loudly.
func ParseFiles(paths []string) (Model, error) {
	var m Model
	for _, p := range paths {
		g, err := gpxgo.ParseFile(p)
		if err != nil {
			return Model{}, fmt.Errorf("parse %s: %w", p, err)
		}
		base := filepath.Base(p)

		// <trk> tracks: concatenate every segment's points.
		for ti := range g.Tracks {
			trk := &g.Tracks[ti]
			var gpts []*gpxgo.GPXPoint
			for si := range trk.Segments {
				seg := &trk.Segments[si]
				for pi := range seg.Points {
					gpts = append(gpts, &seg.Points[pi])
				}
			}
			m.addTrack(fallbackName(trk.Name, base, "track", ti), gpts)
		}

		// <rte> routes: an ordered list of points, treated like a track.
		for ri := range g.Routes {
			rte := &g.Routes[ri]
			gpts := make([]*gpxgo.GPXPoint, 0, len(rte.Points))
			for pi := range rte.Points {
				gpts = append(gpts, &rte.Points[pi])
			}
			m.addTrack(fallbackName(rte.Name, base, "route", ri), gpts)
		}

		// <wpt> waypoints: standalone markers.
		for wi := range g.Waypoints {
			wp := &g.Waypoints[wi]
			m.Waypoints = append(m.Waypoints, Waypoint{
				Lat:  wp.Latitude,
				Lon:  wp.Longitude,
				Name: wp.Name,
				Desc: wp.Description,
			})
			m.extendBoundsLatLon(wp.Latitude, wp.Longitude)
		}
	}
	return m, nil
}

// addTrack converts gpxgo points into a Track and appends it (plus its bounds)
// to the model. Tracks with no points are dropped.
func (m *Model) addTrack(name string, gpts []*gpxgo.GPXPoint) {
	if len(gpts) == 0 {
		return
	}
	pts := make([]Point, 0, len(gpts))
	var prev *gpxgo.GPXPoint
	for _, gp := range gpts {
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
	m.Tracks = append(m.Tracks, Track{Name: name, Points: pts, Stats: computeStats(pts)})
	m.extendBounds(pts)
}

// fallbackName uses the source element's name if present, otherwise derives a
// readable, unambiguous one from the file and element kind/index.
func fallbackName(name, fileBase, kind string, idx int) string {
	if name != "" {
		return name
	}
	return fmt.Sprintf("%s (%s %d)", fileBase, kind, idx+1)
}

// computeStats derives summary statistics directly from the flattened points,
// so it works identically for tracks and routes. Distance uses the haversine
// helper from gpxgo; duration/average speed are reported only when timestamps
// are present and strictly increasing (routes often carry unrelated times).
func computeStats(pts []Point) Stats {
	var dist, gain float64
	for i := 1; i < len(pts); i++ {
		dist += gpxgo.HaversineDistance(pts[i-1].Lat, pts[i-1].Lon, pts[i].Lat, pts[i].Lon)
		if pts[i].HasEle && pts[i-1].HasEle {
			if d := pts[i].Ele - pts[i-1].Ele; d > 0 {
				gain += d
			}
		}
	}

	var first, last time.Time
	for _, p := range pts {
		if !p.HasTime {
			continue
		}
		if first.IsZero() {
			first = p.Time
		}
		last = p.Time
	}

	s := Stats{Distance: dist, ElevGain: gain}
	if !first.IsZero() && last.After(first) {
		s.HasTime = true
		s.Duration = last.Sub(first)
		if secs := s.Duration.Seconds(); secs > 0 {
			s.AvgSpeed = dist / secs
		}
	}
	return s
}

// extendBounds grows the model's bounding box to include the given points.
func (m *Model) extendBounds(pts []Point) {
	for _, p := range pts {
		m.extendBoundsLatLon(p.Lat, p.Lon)
	}
}

// extendBoundsLatLon grows the model's bounding box to include one coordinate.
func (m *Model) extendBoundsLatLon(lat, lon float64) {
	if !m.HasBounds {
		m.MinLat, m.MaxLat = lat, lat
		m.MinLon, m.MaxLon = lon, lon
		m.HasBounds = true
		return
	}
	if lat < m.MinLat {
		m.MinLat = lat
	}
	if lat > m.MaxLat {
		m.MaxLat = lat
	}
	if lon < m.MinLon {
		m.MinLon = lon
	}
	if lon > m.MaxLon {
		m.MaxLon = lon
	}
}
