import type { Coordinate, SearchRequest } from './types'

export function circleRing(center: Coordinate, meters: number): Coordinate[] {
  const rad = Math.PI / 180,
    latitude = center[1] * rad,
    longitude = center[0] * rad,
    angular = meters / 6371008.8
  const points: Coordinate[] = []
  for (let i = 0; i < 96; i++) {
    const bearing = (i / 96) * Math.PI * 2
    const lat = Math.asin(
      Math.sin(latitude) * Math.cos(angular) +
        Math.cos(latitude) * Math.sin(angular) * Math.cos(bearing),
    )
    const lon =
      longitude +
      Math.atan2(
        Math.sin(bearing) * Math.sin(angular) * Math.cos(latitude),
        Math.cos(angular) - Math.sin(latitude) * Math.sin(lat),
      )
    // 仅用于地图渲染：保持相对于中心的连续经度，避免跨 ±180° 时连成横跨全球的边。
    points.push([lon / rad, lat / rad])
  }
  points.push([...points[0]])
  return points
}
export function closeRing(points: Coordinate[]): Coordinate[] {
  if (points.length < 3) throw new Error('围栏至少需要 3 个顶点')
  return [...points.map((p) => [...p] as Coordinate), [...points[0]]]
}
export function makeSearch(
  center: Coordinate,
  radius: number,
  category: string,
  polygon: Coordinate[],
): SearchRequest {
  if (
    center.some((v) => !Number.isFinite(v)) ||
    Math.abs(center[0]) > 180 ||
    Math.abs(center[1]) > 90
  )
    throw new Error('请输入有效的 WGS84 经度和纬度')
  if (!Number.isFinite(radius) || radius <= 0 || radius > 50000)
    throw new Error('半径必须大于 0 且不超过 50000 米')
  return {
    center,
    radius_meters: radius,
    category: category || undefined,
    limit: 100,
    ...(polygon.length
      ? { polygon: { type: 'Polygon' as const, coordinates: [closeRing(polygon)] } }
      : {}),
  }
}
export async function postJSON<T>(url: string, body: unknown, signal?: AbortSignal): Promise<T> {
  const response = await fetch(url, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
    signal,
  })
  let value
  try {
    value = await response.json()
  } catch {
    throw new Error(`服务返回了无法解析的响应（HTTP ${response.status}）`)
  }
  if (!response.ok) throw new Error(value?.error?.message || `请求失败（HTTP ${response.status}）`)
  return value as T
}
export function formatDistance(meters?: number) {
  return meters == null
    ? '—'
    : meters < 1000
      ? `${Math.round(meters)} m`
      : `${(meters / 1000).toFixed(2)} km`
}
export function isPOICollection(value: unknown): value is import('./types').POICollection {
  if (!value || typeof value !== 'object') return false
  const v = value as import('./types').POICollection
  return (
    v.type === 'FeatureCollection' &&
    Array.isArray(v.features) &&
    typeof v.source === 'string' &&
    v.features.every(
      (f) =>
        f?.geometry?.type === 'Point' &&
        Array.isArray(f.geometry.coordinates) &&
        f.geometry.coordinates.length === 2 &&
        f.geometry.coordinates.every(Number.isFinite) &&
        typeof f.properties?.name === 'string',
    )
  )
}
