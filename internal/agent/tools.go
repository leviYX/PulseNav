package agent

import (
	"context"
	"encoding/json"
	"io"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"pulsenav/internal/domain"
	"pulsenav/internal/spatialsvc"
)

// inferTool 对模型参数执行与 HTTP 一致的严格解码；不依赖模型遵守 JSON Schema。
func inferTool[T, D any](name, description string, fn func(context.Context, *T) (*D, error)) (tool.InvokableTool, error) {
	return utils.InferTool(name, description, fn, utils.WithUnmarshalArguments(func(ctx context.Context, arguments string) (any, error) {
		if len(arguments) > 64<<10 {
			return nil, domain.Invalid("工具参数最多 64KiB")
		}
		decoder := json.NewDecoder(strings.NewReader(arguments))
		decoder.DisallowUnknownFields()
		var req *T
		if err := decoder.Decode(&req); err != nil || req == nil {
			return nil, domain.Invalid("工具参数无效")
		}
		if err := decoder.Decode(new(any)); err != io.EOF {
			return nil, domain.Invalid("工具参数必须是单个 JSON 对象")
		}
		return req, nil
	}))
}

type POISearcher interface {
	SearchPOI(context.Context, *domain.SearchRequest) ([]*domain.POI, error)
	Source() string
}

func buildTools(search POISearcher) ([]tool.BaseTool, error) {
	distance, err := inferTool("calculate_distance", "计算两个 WGS84 点的 Haversine 球面近似距离，结果单位米。", spatialsvc.CalculateDistance)
	if err != nil {
		return nil, err
	}
	polygon, err := inferTool("point_in_polygon", "判断 WGS84 点是否在简单围栏内，包含边界，不支持孔洞、极区、跨日期变更线。", spatialsvc.CheckPolygon)
	if err != nil {
		return nil, err
	}
	conversion, err := inferTool("wgs84_to_gcj02", "将 WGS84 转为近似 GCJ02，适用矩形区域外保持原坐标；禁止对已是 GCJ02 的数据重复转换。", spatialsvc.ConvertCoordinates)
	if err != nil {
		return nil, err
	}
	searchTool, err := inferTool("search_poi", "查询 POI，坐标必须为 WGS84。圆形与围栏取交集；分类精确匹配；结果含数据来源 demo/postgis，demo 是合成数据。", func(ctx context.Context, r *domain.SearchRequest) (*domain.FeatureCollection, error) {
		result, err := search.SearchPOI(ctx, r)
		if err != nil {
			return nil, err
		}
		return domain.GeoJSON(result, search.Source()), nil
	})
	if err != nil {
		return nil, err
	}
	return []tool.BaseTool{
		&tracedTool{InvokableTool: distance, name: "calculate_distance"},
		&tracedTool{InvokableTool: polygon, name: "point_in_polygon"},
		&tracedTool{InvokableTool: conversion, name: "wgs84_to_gcj02"},
		&tracedTool{InvokableTool: searchTool, name: "search_poi"},
	}, nil
}
