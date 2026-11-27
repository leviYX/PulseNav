package spatialsvc

import (
	"context"

	"pulsenav/internal/domain"
	"pulsenav/pkg/spatial"
)

type DistanceRequest struct {
	From *spatial.Point `json:"from" jsonschema:"required,description=WGS84 起点 [经度,纬度]"`
	To   *spatial.Point `json:"to" jsonschema:"required,description=WGS84 终点 [经度,纬度]"`
}
type DistanceResult struct {
	Meters float64 `json:"meters"`
	Method string  `json:"method"`
}
type PolygonRequest struct {
	Point   *spatial.Point   `json:"point" jsonschema:"required,description=WGS84 点 [经度,纬度]"`
	Polygon *spatial.Polygon `json:"polygon" jsonschema:"required,description=无孔洞且不跨日期变更线的闭环 GeoJSON Polygon"`
}
type PolygonResult struct {
	Inside           bool `json:"inside"`
	BoundaryIncluded bool `json:"boundary_included"`
}
type ConversionRequest struct {
	Point *spatial.Point `json:"point" jsonschema:"required,description=WGS84 坐标 [经度,纬度]"`
}
type ConversionResult struct {
	Coordinates spatial.Point `json:"coordinates"`
	CRS         string        `json:"crs"`
	Approximate bool          `json:"approximate"`
}

func validatePoint(p *spatial.Point) error {
	if p == nil {
		return domain.Invalid("缺少坐标")
	}
	if err := p.Validate(); err != nil {
		return domain.Invalid(err.Error())
	}
	return nil
}

func CalculateDistance(ctx context.Context, r *DistanceRequest) (*DistanceResult, error) {
	if r == nil {
		return nil, domain.Invalid("缺少参数")
	}
	if err := validatePoint(r.From); err != nil {
		return nil, err
	}
	if err := validatePoint(r.To); err != nil {
		return nil, err
	}
	return &DistanceResult{Meters: spatial.CalculateDistance(*r.From, *r.To), Method: "haversine_sphere"}, nil
}

func CheckPolygon(ctx context.Context, r *PolygonRequest) (*PolygonResult, error) {
	if r == nil || r.Polygon == nil {
		return nil, domain.Invalid("缺少围栏")
	}
	if err := validatePoint(r.Point); err != nil {
		return nil, err
	}
	if err := r.Polygon.Validate(); err != nil {
		return nil, domain.Invalid(err.Error())
	}
	return &PolygonResult{Inside: spatial.IsPointInPolygon(*r.Point, *r.Polygon), BoundaryIncluded: true}, nil
}

func ConvertCoordinates(ctx context.Context, r *ConversionRequest) (*ConversionResult, error) {
	if r == nil {
		return nil, domain.Invalid("缺少参数")
	}
	if err := validatePoint(r.Point); err != nil {
		return nil, err
	}
	return &ConversionResult{Coordinates: spatial.WGS84ToGCJ02(*r.Point), CRS: "GCJ02", Approximate: true}, nil
}
