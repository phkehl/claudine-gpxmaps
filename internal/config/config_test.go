package config

import "testing"

func TestOutputName(t *testing.T) {
	cases := []struct {
		in   []string
		want string
	}{
		{nil, "tracks.html"},
		{[]string{"ride.gpx"}, "ride.html"},
		{[]string{"a.gpx", "b.gpx"}, "a_b.html"},
		{[]string{"trips/ride1.gpx", "trips/ride2.gpx"}, "ride1_ride2.html"},
	}
	for _, c := range cases {
		if got := OutputName(c.in); got != c.want {
			t.Errorf("OutputName(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}
