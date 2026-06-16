package gpx

import (
	"testing"
	"time"
)

func TestParseSample(t *testing.T) {
	m, err := ParseFiles([]string{"../../testdata/sample.gpx"})
	if err != nil {
		t.Fatalf("ParseFiles: %v", err)
	}
	if len(m.Tracks) != 1 {
		t.Fatalf("tracks = %d, want 1", len(m.Tracks))
	}
	tr := m.Tracks[0]
	if tr.Name != "Test Track" {
		t.Errorf("name = %q, want %q", tr.Name, "Test Track")
	}
	if len(tr.Points) != 4 {
		t.Fatalf("points = %d, want 4", len(tr.Points))
	}

	// Three ~111 m hops along a meridian ≈ 330 m total.
	if d := tr.Stats.Distance; d < 300 || d > 360 {
		t.Errorf("distance = %.1f m, want ~330", d)
	}
	if !tr.Stats.HasTime {
		t.Fatal("HasTime = false, want true")
	}
	if got, want := tr.Stats.Duration, 30*time.Second; got != want {
		t.Errorf("duration = %v, want %v", got, want)
	}
	// Uphill: +10, -5, +10 → 20 m of climb.
	if g := tr.Stats.ElevGain; g < 15 || g > 25 {
		t.Errorf("elev gain = %.1f m, want ~20", g)
	}

	// Bounds span all four points.
	if !m.HasBounds {
		t.Fatal("HasBounds = false")
	}
	if m.MinLat > 47.3769 || m.MaxLat < 47.3799 {
		t.Errorf("lat bounds = [%.4f, %.4f], want to span 47.3769..47.3799", m.MinLat, m.MaxLat)
	}

	// First point has no previous, so no speed; later points do.
	if tr.Points[0].Speed != 0 {
		t.Errorf("point[0] speed = %.2f, want 0", tr.Points[0].Speed)
	}
	if tr.Points[1].Speed <= 0 {
		t.Errorf("point[1] speed = %.2f, want > 0", tr.Points[1].Speed)
	}
}

// TestParseRouteAndWaypoints covers GPX 1.0 files that carry a <rte> route and
// <wpt> waypoints instead of a <trk> (e.g. the classic ExpertGPS fells_loop).
func TestParseRouteAndWaypoints(t *testing.T) {
	m, err := ParseFiles([]string{"../../testdata/route.gpx"})
	if err != nil {
		t.Fatalf("ParseFiles: %v", err)
	}

	// The <rte> becomes a Track.
	if len(m.Tracks) != 1 {
		t.Fatalf("tracks = %d, want 1 (the route)", len(m.Tracks))
	}
	tr := m.Tracks[0]
	if tr.Name != "Fells Loop" {
		t.Errorf("route name = %q, want %q", tr.Name, "Fells Loop")
	}
	if len(tr.Points) != 3 {
		t.Errorf("route points = %d, want 3", len(tr.Points))
	}
	if tr.Stats.Distance <= 0 {
		t.Errorf("route distance = %.1f, want > 0", tr.Stats.Distance)
	}
	// No timestamps on the route, so no duration/speed reported.
	if tr.Stats.HasTime {
		t.Errorf("route HasTime = true, want false (no timestamps)")
	}
	// Elevation: 40->50 (+10), 50->45 (-5) → 10 m climb.
	if g := tr.Stats.ElevGain; g < 8 || g > 12 {
		t.Errorf("route elev gain = %.1f, want ~10", g)
	}

	// The <wpt> becomes a Waypoint.
	if len(m.Waypoints) != 1 {
		t.Fatalf("waypoints = %d, want 1", len(m.Waypoints))
	}
	if m.Waypoints[0].Name != "Trailhead" {
		t.Errorf("waypoint name = %q, want %q", m.Waypoints[0].Name, "Trailhead")
	}

	// Bounds span both the route and the waypoint.
	if !m.HasBounds || m.MaxLat < 42.4389 {
		t.Errorf("bounds should include the waypoint at 42.4389; got max lat %.4f", m.MaxLat)
	}
}
