package poi

import (
	"context"
	"sort"

	"pulsenav/internal/domain"
	"pulsenav/pkg/spatial"
)

// DemoRepository 的私有记录在构造后不再修改；每次查询复制结果，无需读锁。
// 这是合成数据，只用于演示，球面距离不能用于替代 PostGIS 椭球距离精度验收。
type DemoRepository struct{ records []domain.POI }

func NewDemoRepository() *DemoRepository {
	return &DemoRepository{records: []domain.POI{
		{ID: "demo-cafe-1", Name: "演示咖啡 A（合成数据）", Category: "cafe", Location: spatial.Point{121.4737, 31.2304}},
		{ID: "demo-cafe-2", Name: "演示咖啡 B（合成数据）", Category: "cafe", Location: spatial.Point{121.476, 31.232}},
		{ID: "demo-shop-1", Name: "演示商店（合成数据）", Category: "shop", Location: spatial.Point{121.474, 31.231}},
	}}
}
func (*DemoRepository) Source() string { return "demo" }
func (r *DemoRepository) SearchPOI(ctx context.Context, q *domain.SearchRequest) ([]*domain.POI, error) {
	out := make([]*domain.POI, 0)
	for _, record := range r.records {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		p := record
		if q.Category != "" && q.Category != p.Category {
			continue
		}
		if q.Center != nil {
			distance := spatial.CalculateDistance(*q.Center, p.Location)
			if distance > q.RadiusMeters {
				continue
			}
			p.DistanceMeters = &distance
		}
		if q.Polygon != nil && !spatial.IsPointInPolygon(p.Location, *q.Polygon) {
			continue
		}
		out = append(out, &p)
	}
	sort.Slice(out, func(i, j int) bool {
		if q.Center != nil && *out[i].DistanceMeters != *out[j].DistanceMeters {
			return *out[i].DistanceMeters < *out[j].DistanceMeters
		}
		return out[i].ID < out[j].ID
	})
	if len(out) > q.Limit {
		out = out[:q.Limit]
	}
	return out, nil
}
