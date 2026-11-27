-- 由部署者执行，服务进程不自动迁移或持有 DDL 权限。
CREATE EXTENSION IF NOT EXISTS postgis;

CREATE TABLE IF NOT EXISTS poi (
    id text PRIMARY KEY,
    name text NOT NULL,
    category text NOT NULL,
    location geography(Point, 4326) NOT NULL,
    CONSTRAINT poi_location_valid CHECK (
        NOT ST_IsEmpty(location::geometry)
        AND ST_IsValid(location::geometry)
    )
);

CREATE INDEX IF NOT EXISTS poi_location_geography_gist ON poi USING gist (location);
CREATE INDEX IF NOT EXISTS poi_location_geometry_gist ON poi USING gist ((location::geometry));
CREATE INDEX IF NOT EXISTS poi_category_idx ON poi (category);
