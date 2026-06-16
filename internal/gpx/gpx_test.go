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
