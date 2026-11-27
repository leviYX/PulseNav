import type { FeatureCollection, Point, Polygon } from 'geojson'

export type Coordinate = [number, number]
export type POIProperties = { name: string; category: string; distance_meters?: number }
export type POICollection = FeatureCollection<Point, POIProperties> & { source: string }
export type SearchRequest = {
  center: Coordinate
  radius_meters: number
  category?: string
  limit: number
  polygon?: Polygon
}
export type Health = { status: string; poi_source: string; agent_enabled: boolean }
export type ToolResult = { name: string; result: unknown; arguments?: unknown; elapsed_ms?: number }
export type AgentResult = { answer: string; tool_results: ToolResult[] }
export type Execution = {
  id: number
  name: string
  input: unknown
  result?: unknown
  elapsed: number
  status: 'success' | 'error' | 'canceled'
  error?: string
}
export type ChatMessage = {
  id: number
  role: 'user' | 'assistant'
  content: string
  error?: boolean
}
