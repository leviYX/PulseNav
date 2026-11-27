package poi

import (
	"context"
	"errors"
	"math"
	"testing"

	"pulsenav/internal/domain"
	"pulsenav/pkg/spatial"
)

func TestSearchIntersectionAndStableOrder(t *testing.T) {
	svc := NewService(NewDemoRepository())
	center := spatial.Point{121.4737, 31.2304}
	polygon := spatial.Polygon{Type: "Polygon", Coordinates: [][]spatial.Point{{{121.473, 31.23}, {121.474, 31.23}, {121.474, 31.231}, {121.473, 31.231}, {121.473, 31.23}}}}
	got, err := svc.SearchPOI(context.Background(), &domain.SearchRequest{Center: &center, RadiusMeters: 1000, Polygon: &polygon, Category: "cafe"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "demo-cafe-1" {
		t.Fatalf("got %+v", got)
	}
	// 仅围栏查询有稳定 ID 排序；空结果必须是空数组。
	got, err = svc.SearchPOI(context.Background(), &domain.SearchRequest{Polygon: &polygon, Category: "absent"})
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("got %v %v", got, err)
	}
}

func TestSearchRejectsInvalidAndCancellation(t *testing.T) {
	svc := NewService(NewDemoRepository())
	p := spatial.Point{0, 0}
	for _, req := range []*domain.SearchRequest{nil, {}, {Center: &p}, {RadiusMeters: 10}, {Center: &p, RadiusMeters: 50001}, {Center: &p, RadiusMeters: math.NaN()}, {Center: &p, RadiusMeters: 10, Limit: 101}} {
		_, err := svc.SearchPOI(context.Background(), req)
		if !errors.Is(err, domain.ErrInvalidArgument) {
			t.Errorf("got %v", err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := svc.SearchPOI(ctx, &domain.SearchRequest{Center: &p, RadiusMeters: 10})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
}

func TestDemoReturnsIndependentResults(t *testing.T) {
	svc := NewService(NewDemoRepository())
	p := spatial.Point{121.4737, 31.2304}
	req := &domain.SearchRequest{Center: &p, RadiusMeters: 1000}
	a, _ := svc.SearchPOI(context.Background(), req)
	a[0].Name = "mutated"
	b, _ := svc.SearchPOI(context.Background(), req)
	if b[0].Name == "mutated" {
		t.Fatal("shared mutable result")
	}
	if req.Limit != 0 {
		t.Fatal("request mutated")
	}
}
