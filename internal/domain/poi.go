package domain

import (
	"math"
	"strings"

	"pulsenav/pkg/spatial"
)

type SearchRequest struct {
	Center       *spatial.Point   `json:"center,omitempty" jsonschema:"description=WGS84 中心坐标 [经度,纬度]，必须与 radius_meters 同时提供"`
	RadiusMeters float64          `json:"radius_meters,omitempty" jsonschema:"description=搜索半径，单位米，大于零且最多 50000"`
	Polygon      *spatial.Polygon `json:"polygon,omitempty" jsonschema:"description=可选 GeoJSON 简单 Polygon，与圆形取交集"`
	Category     string           `json:"category,omitempty" jsonschema:"description=可选 POI 分类精确匹配，例如 cafe"`
	Limit        int              `json:"limit,omitempty" jsonschema:"description=结果数，默认 20，最大 100"`
}

func (r *SearchRequest) Normalize() (*SearchRequest, error) {
	if r == nil {
		return nil, Invalid("请求不能为空")
	}
	q := *r
	if math.IsNaN(q.RadiusMeters) || math.IsInf(q.RadiusMeters, 0) || q.RadiusMeters < 0 || q.RadiusMeters > 50000 {
		return nil, Invalid("半径必须在 (0,50000] 米内")
	}
	if (q.Center == nil) != (q.RadiusMeters == 0) {
		return nil, Invalid("中心点与正数半径必须成对提供")
	}
	if q.Center == nil && q.Polygon == nil {
		return nil, Invalid("至少提供圆形或围栏条件")
	}
	if q.Center != nil {
		if err := q.Center.Validate(); err != nil {
			return nil, Invalid(err.Error())
		}
	}
	if q.Polygon != nil {
		if err := q.Polygon.Validate(); err != nil {
			return nil, Invalid(err.Error())
		}
	}
	if q.Limit < 0 || q.Limit > 100 {
		return nil, Invalid("limit 必须在 1 至 100 内")
	}
	if q.Limit == 0 {
		q.Limit = 20
	}
	q.Category = strings.TrimSpace(q.Category)
	if len(q.Category) > 128 {
		return nil, Invalid("分类过长")
	}
	return &q, nil
}

type POI struct {
	ID             string
	Name           string
	Category       string
	Location       spatial.Point
	DistanceMeters *float64
}

type PointGeometry struct {
	Type        string        `json:"type"`
	Coordinates spatial.Point `json:"coordinates"`
}
type POIProperties struct {
	Name           string   `json:"name"`
	Category       string   `json:"category"`
	DistanceMeters *float64 `json:"distance_meters,omitempty"`
}
type Feature struct {
	Type       string        `json:"type"`
	ID         string        `json:"id"`
	Geometry   PointGeometry `json:"geometry"`
	Properties POIProperties `json:"properties"`
}
type FeatureCollection struct {
	Type     string    `json:"type"`
	Features []Feature `json:"features"`
	Source   string    `json:"source"`
}

func GeoJSON(pois []*POI, source string) *FeatureCollection {
	out := &FeatureCollection{Type: "FeatureCollection", Features: make([]Feature, 0, len(pois)), Source: source}
	for _, p := range pois {
		out.Features = append(out.Features, Feature{Type: "Feature", ID: p.ID, Geometry: PointGeometry{Type: "Point", Coordinates: p.Location}, Properties: POIProperties{Name: p.Name, Category: p.Category, DistanceMeters: p.DistanceMeters}})
	}
	return out
}
