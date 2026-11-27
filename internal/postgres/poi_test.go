package postgres

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"pulsenav/internal/domain"
	"pulsenav/internal/poi"
	"pulsenav/pkg/spatial"
)

func TestPostGISSearch(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to run real PostGIS integration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal("cannot connect to test database")
	}
	defer conn.Close(context.Background())
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	// 使用事务级独立 schema，测试结束回滚，不改已有业务表。
	schema := pgx.Identifier{"pulsegeo_test_" + time.Now().Format("150405000000000")}.Sanitize()
	if _, err = tx.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, "SET LOCAL search_path TO "+schema+", public"); err != nil {
		t.Fatal(err)
	}
	sql, err := os.ReadFile(filepath.Join("..", "..", "migrations", "001_poi.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, string(sql)); err != nil {
		t.Fatal(err)
	}
	_, err = tx.Exec(ctx, `INSERT INTO poi (id,name,category,location) VALUES
 ('a','center','cafe',ST_SetSRID(ST_MakePoint(121.4737,31.2304),4326)::geography),
 ('b','nearby','cafe',ST_SetSRID(ST_MakePoint(121.476,31.232),4326)::geography),
 ('c','shop','shop',ST_SetSRID(ST_MakePoint(121.4737,31.2304),4326)::geography)`)
	if err != nil {
		t.Fatal(err)
	}
	svc := poi.NewService(NewRepository(tx))
	p := spatial.Point{121.4737, 31.2304}
	got, err := svc.SearchPOI(ctx, &domain.SearchRequest{Center: &p, RadiusMeters: 1000, Category: "cafe"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != "a" || got[1].ID != "b" {
		t.Fatalf("unexpected result: %+v", got)
	}
	poly := spatial.Polygon{Type: "Polygon", Coordinates: [][]spatial.Point{{p, {121.474, 31.2304}, {121.474, 31.231}, p}}}
	got, err = svc.SearchPOI(ctx, &domain.SearchRequest{Center: &p, RadiusMeters: 1000, Category: "cafe", Polygon: &poly})
	if err != nil || len(got) != 1 || got[0].ID != "a" {
		t.Fatalf("boundary/intersection: %+v %v", got, err)
	}
	got, err = svc.SearchPOI(ctx, &domain.SearchRequest{Polygon: &poly, Category: "' OR 1=1 --"})
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("empty/injection: %+v %v", got, err)
	}
	canceled, stop := context.WithCancel(ctx)
	stop()
	_, err = svc.SearchPOI(canceled, &domain.SearchRequest{Center: &p, RadiusMeters: 1000})
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
