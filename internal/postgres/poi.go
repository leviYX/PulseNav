package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"pulsenav/internal/domain"
)

// Querier 使连接池与测试事务都能注入，不要求业务依赖具体连接类型。
type Querier interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}
type Repository struct{ db Querier }

func NewRepository(db Querier) *Repository { return &Repository{db: db} }
func (*Repository) Source() string         { return "postgis" }

func (r *Repository) SearchPOI(ctx context.Context, q *domain.SearchRequest) ([]*domain.POI, error) {
	args := make([]any, 0, 6)
	where := make([]string, 0, 3)
	distance := "NULL::double precision"
	order := "id"
	bind := func(v any) string { args = append(args, v); return fmt.Sprintf("$%d", len(args)) }
	if q.Center != nil {
		lon, lat, radius := bind((*q.Center)[0]), bind((*q.Center)[1]), bind(q.RadiusMeters)
		center := "ST_SetSRID(ST_MakePoint(" + lon + "::float8," + lat + "::float8),4326)::geography"
		// geography 的 ST_DWithin 使用米，并与 location 上的 GiST 索引匹配。
		where = append(where, "ST_DWithin(location,"+center+","+radius+"::float8)")
		distance = "ST_Distance(location," + center + ")"
		order = "distance_meters, id"
	}
	if q.Polygon != nil {
		encoded, err := json.Marshal(q.Polygon)
		if err != nil {
			return nil, domain.Invalid("围栏无法编码")
		}
		poly := bind(string(encoded))
		// geometry 表达式索引与查询相同，ST_Covers 包含顶点与线段边界。
		where = append(where, "ST_Covers(ST_SetSRID(ST_GeomFromGeoJSON("+poly+"::text),4326),location::geometry)")
	}
	if q.Category != "" {
		where = append(where, "category = "+bind(q.Category)+"::text")
	}
	limit := bind(q.Limit)
	sql := "SELECT id,name,category,ST_X(location::geometry),ST_Y(location::geometry)," + distance + " AS distance_meters FROM poi WHERE " + strings.Join(where, " AND ") + " ORDER BY " + order + " LIMIT " + limit + "::int"
	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, domain.Unavailable(err)
	}
	defer rows.Close()
	out := make([]*domain.POI, 0)
	for rows.Next() {
		p := new(domain.POI)
		if err = rows.Scan(&p.ID, &p.Name, &p.Category, &p.Location[0], &p.Location[1], &p.DistanceMeters); err != nil {
			return nil, domain.Unavailable(err)
		}
		out = append(out, p)
	}
	if err = rows.Err(); err != nil {
		return nil, domain.Unavailable(err)
	}
	return out, nil
}
