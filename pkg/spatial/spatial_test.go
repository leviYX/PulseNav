package spatial

import (
	"encoding/json"
	"math"
	"testing"
)

func TestPointRequiresExactlyTwoNumbers(t *testing.T) {
	for _, raw := range []string{`[]`, `[1]`, `[1,2,3]`, `[null,2]`, `null`, `["1",2]`} {
		var p Point
		if json.Unmarshal([]byte(raw), &p) == nil {
			t.Errorf("accepted %s", raw)
		}
	}
	var p Point
	if err := json.Unmarshal([]byte(`[121,31]`), &p); err != nil || p != (Point{121, 31}) {
		t.Fatalf("got %v %v", p, err)
	}
}

func TestDistance(t *testing.T) {
	for _, tt := range []struct {
		name            string
		a, b            Point
		want, tolerance float64
	}{
		{"same", Point{121, 31}, Point{121, 31}, 0, 1e-8},
		{"equator", Point{0, 0}, Point{1, 0}, 111195.0802, .01},
		{"dateline", Point{179.9, 0}, Point{-179.9, 0}, 22239.016, .01},
		{"antipodes", Point{0, 0}, Point{180, 0}, 20015114.442, .01},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateDistance(tt.a, tt.b)
			if math.IsNaN(got) || math.Abs(got-tt.want) > tt.tolerance {
				t.Fatalf("got %v want %v", got, tt.want)
			}
		})
	}
}

func square() Polygon {
	return Polygon{Type: "Polygon", Coordinates: [][]Point{{{0, 0}, {2, 0}, {2, 2}, {0, 2}, {0, 0}}}}
}

func TestPolygonBoundary(t *testing.T) {
	p := square()
	if err := p.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		p    Point
		want bool
	}{{Point{1, 1}, true}, {Point{0, 0}, true}, {Point{1, 0}, true}, {Point{2, 1}, true}, {Point{3, 1}, false}, {Point{1, 2.001}, false}} {
		if got := IsPointInPolygon(tt.p, p); got != tt.want {
			t.Errorf("%v got %v", tt.p, got)
		}
	}
}

func TestRejectInvalidGeometry(t *testing.T) {
	cases := []Polygon{
		{}, {Type: "Polygon", Coordinates: [][]Point{{{0, 0}, {1, 1}, {0, 0}}}},
		{Type: "Polygon", Coordinates: [][]Point{{{0, 0}, {2, 2}, {0, 2}, {2, 0}, {0, 0}}}},
		{Type: "Polygon", Coordinates: [][]Point{{{179, 0}, {-179, 0}, {-179, 2}, {179, 0}}}},
		{Type: "Polygon", Coordinates: [][]Point{{{0, 0}, {1, 0}, {2, 0}, {0, 0}}}},
		{Type: "Polygon", Coordinates: [][]Point{{{0, 0}, {1, 0}, {1, 1}, {0, 1}}}},
	}
	hole := square()
	hole.Coordinates = append(hole.Coordinates, hole.Coordinates[0])
	cases = append(cases, hole)
	for i, p := range cases {
		if p.Validate() == nil {
			t.Errorf("case %d accepted", i)
		}
	}
	for _, p := range []Point{{181, 0}, {0, 91}, {math.NaN(), 0}, {0, math.Inf(1)}} {
		if p.Validate() == nil {
			t.Errorf("accepted %v", p)
		}
	}
}

func TestGCJConversion(t *testing.T) {
	outside := Point{-74, 40}
	if WGS84ToGCJ02(outside) != outside {
		t.Fatal("outside changed")
	}
	got := WGS84ToGCJ02(Point{116.404, 39.915})
	if math.Abs(got[0]-116.410244499) > 1e-8 || math.Abs(got[1]-39.916404282) > 1e-8 {
		t.Fatalf("got %v", got)
	}
}
