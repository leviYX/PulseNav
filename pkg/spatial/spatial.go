// Package spatial 提供面向已校验 WGS84 坐标的纯函数；坐标顺序为经度、纬度。
package spatial

import (
	"encoding/json"
	"fmt"
	"math"
)

const EarthRadiusMeters = 6371008.8 // IUGG 平均地球半径；不是椭球测地线距离。
const degreeTolerance = 1e-10       // 边界容差单位为度，赤道附近约 0.011mm。

// Point 与 GeoJSON position 一致，只接受二维坐标。
type Point [2]float64

// UnmarshalJSON 不沿用固定数组默认的截断/补零行为，防止缺失或多余坐标被接受。
func (p *Point) UnmarshalJSON(data []byte) error {
	var values []*float64
	if err := json.Unmarshal(data, &values); err != nil {
		return err
	}
	if len(values) != 2 || values[0] == nil || values[1] == nil {
		return fmt.Errorf("坐标必须包含两个数值 [经度,纬度]")
	}
	*p = Point{*values[0], *values[1]}
	return p.Validate()
}

func (p Point) Validate() error {
	if math.IsNaN(p[0]) || math.IsNaN(p[1]) || math.IsInf(p[0], 0) || math.IsInf(p[1], 0) || p[0] < -180 || p[0] > 180 || p[1] < -90 || p[1] > 90 {
		return fmt.Errorf("经纬度必须有限且在 [-180,180]、[-90,90] 范围内")
	}
	return nil
}

type Polygon struct {
	Type        string    `json:"type"`
	Coordinates [][]Point `json:"coordinates"`
}

// Validate 限制单环、非自交普通区域围栏。O(n²) 检查由 256 个顶点上限约束。
// 不将跨日期变更线或极区围栏错误地当成经纬度平面多边形。
func (p Polygon) Validate() error {
	if p.Type != "Polygon" || len(p.Coordinates) != 1 {
		return fmt.Errorf("首期只支持无孔洞的 GeoJSON Polygon")
	}
	r := p.Coordinates[0]
	if len(r) < 4 || len(r) > 257 || r[0] != r[len(r)-1] {
		return fmt.Errorf("围栏需闭环，包含 3 至 256 个顶点")
	}
	area := 0.0
	for i := 0; i < len(r)-1; i++ {
		if err := r[i].Validate(); err != nil {
			return err
		}
		if math.Abs(r[i][1]) > 85 || math.Abs(r[i][0]-r[i+1][0]) > 180 {
			return fmt.Errorf("不支持极区或跨日期变更线的围栏")
		}
		if r[i] == r[i+1] {
			return fmt.Errorf("围栏含重复的相邻顶点")
		}
		// 平移到首点再算有向面积，降低大坐标、小围栏的相消误差。
		area += (r[i][0]-r[0][0])*(r[i+1][1]-r[0][1]) - (r[i+1][0]-r[0][0])*(r[i][1]-r[0][1])
		for j := i + 1; j < len(r)-1; j++ {
			if j == i+1 || i == 0 && j == len(r)-2 {
				continue
			}
			if intersects(r[i], r[i+1], r[j], r[j+1]) {
				return fmt.Errorf("围栏不能自交或自触")
			}
		}
		// 相邻边反向重叠同样属于无效几何。
		prev := r[(i+len(r)-2)%(len(r)-1)]
		next := r[i+1]
		if onSegment(prev, r[i], next) || onSegment(next, prev, r[i]) {
			return fmt.Errorf("相邻边不能重叠")
		}
	}
	if math.Abs(area) < 1e-18 {
		return fmt.Errorf("围栏面积过小或退化")
	}
	return nil
}

func CalculateDistance(a, b Point) float64 {
	rad := math.Pi / 180
	dlat := (b[1] - a[1]) * rad
	dlon := (b[0] - a[0]) * rad
	h := math.Pow(math.Sin(dlat/2), 2) + math.Cos(a[1]*rad)*math.Cos(b[1]*rad)*math.Pow(math.Sin(dlon/2), 2)
	// 对跖点附近舍入可能使 h 略大于 1，钳制后避免开方产生 NaN。
	h = math.Max(0, math.Min(1, h))
	return 2 * EarthRadiusMeters * math.Atan2(math.Sqrt(h), math.Sqrt(1-h))
}

// IsPointInPolygon 以半开区间处理顶点，水平边不计入交点数；边界先行判定。
// 调用方必须先 Validate。经纬度上的直线边与 PostGIS geometry 边语义一致。
func IsPointInPolygon(point Point, p Polygon) bool {
	if len(p.Coordinates) != 1 {
		return false
	}
	r := p.Coordinates[0]
	inside := false
	for i := 0; i+1 < len(r); i++ {
		a, b := r[i], r[i+1]
		if onSegment(point, a, b) {
			return true
		}
		if (a[1] > point[1]) != (b[1] > point[1]) && point[0] < (b[0]-a[0])*(point[1]-a[1])/(b[1]-a[1])+a[0] {
			inside = !inside
		}
	}
	return inside
}

func cross(a, b, c Point) float64 { return (b[0]-a[0])*(c[1]-a[1]) - (b[1]-a[1])*(c[0]-a[0]) }
func onSegment(p, a, b Point) bool {
	scale := math.Abs(b[0]-a[0]) + math.Abs(b[1]-a[1])
	return math.Abs(cross(a, b, p)) <= degreeTolerance*scale && p[0] >= math.Min(a[0], b[0])-degreeTolerance && p[0] <= math.Max(a[0], b[0])+degreeTolerance && p[1] >= math.Min(a[1], b[1])-degreeTolerance && p[1] <= math.Max(a[1], b[1])+degreeTolerance
}
func intersects(a, b, c, d Point) bool {
	if onSegment(a, c, d) || onSegment(b, c, d) || onSegment(c, a, b) || onSegment(d, a, b) {
		return true
	}
	return (cross(a, b, c) > 0) != (cross(a, b, d) > 0) && (cross(c, d, a) > 0) != (cross(c, d, b) > 0)
}

// WGS84ToGCJ02 使用常见公开近似公式（Krasovsky 椭球参数）。矩形区域判断
// 仅为算法适用范围近似，不代表行政边界；结果不可用于测绘级精度承诺。
func WGS84ToGCJ02(p Point) Point {
	lon, lat := p[0], p[1]
	if lon < 72.004 || lon > 137.8347 || lat < 0.8293 || lat > 55.8271 {
		return p
	}
	x, y := lon-105, lat-35
	dlat := -100 + 2*x + 3*y + .2*y*y + .1*x*y + .2*math.Sqrt(math.Abs(x))
	dlat += (20*math.Sin(6*x*math.Pi) + 20*math.Sin(2*x*math.Pi)) * 2 / 3
	dlat += (20*math.Sin(y*math.Pi) + 40*math.Sin(y*math.Pi/3)) * 2 / 3
	dlat += (160*math.Sin(y*math.Pi/12) + 320*math.Sin(y*math.Pi/30)) * 2 / 3
	dlon := 300 + x + 2*y + .1*x*x + .1*x*y + .1*math.Sqrt(math.Abs(x))
	dlon += (20*math.Sin(6*x*math.Pi) + 20*math.Sin(2*x*math.Pi)) * 2 / 3
	dlon += (20*math.Sin(x*math.Pi) + 40*math.Sin(x*math.Pi/3)) * 2 / 3
	dlon += (150*math.Sin(x*math.Pi/12) + 300*math.Sin(x*math.Pi/30)) * 2 / 3
	const a = 6378245.0
	const ee = .00669342162296594323
	rad := lat * math.Pi / 180
	magic := 1 - ee*math.Pow(math.Sin(rad), 2)
	root := math.Sqrt(magic)
	return Point{lon + dlon/(a/root*math.Cos(rad)*math.Pi)*180, lat + dlat/((a*(1-ee))/(magic*root)*math.Pi)*180}
}
