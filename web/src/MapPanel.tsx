import { useEffect, useRef, useState } from 'react'
import { Crosshair, Layers, Map as MapIcon, Minus, Plus, TriangleAlert } from 'lucide-react'
import type { GeoJSONSource, Map as GLMap, StyleSpecification } from 'maplibre-gl'
import type { FeatureCollection } from 'geojson'
import 'maplibre-gl/dist/maplibre-gl.css'
import workerUrl from 'maplibre-gl/dist/maplibre-gl-worker.mjs?worker&url'
import { circleRing, closeRing } from './lib'
import type { Coordinate, POICollection } from './types'

type Props = {
  center: Coordinate
  radius: number
  polygon: Coordinate[]
  drawing: boolean
  pois: POICollection | null
  selected: string | null
  focus: { point: Coordinate; revision: number } | null
  onPick: (point: Coordinate) => void
  onSelect: (id: string) => void
}
const empty = (): FeatureCollection => ({ type: 'FeatureCollection', features: [] })
const style = (tiles: boolean): StyleSpecification => ({
  version: 8,
  sources: tiles
    ? {
        osm: {
          type: 'raster',
          tiles: ['https://tile.openstreetmap.org/{z}/{x}/{y}.png'],
          tileSize: 256,
          maxzoom: 19,
          attribution:
            '© <a href="https://www.openstreetmap.org/copyright" target="_blank" rel="noopener">OpenStreetMap</a> contributors',
        },
      }
    : {},
  layers: [
    { id: 'background', type: 'background', paint: { 'background-color': '#182321' } },
    ...(tiles
      ? [
          {
            id: 'basemap',
            type: 'raster' as const,
            source: 'osm',
            paint: { 'raster-saturation': -0.72, 'raster-brightness-max': 0.75 },
          },
        ]
      : []),
  ],
})

function validCoordinate(p: Coordinate) {
  return p.every(Number.isFinite) && Math.abs(p[0]) <= 180 && Math.abs(p[1]) <= 85
}
export default function MapPanel(input: Props) {
  // 表单编辑值不能直接成为地图状态；MapLibre 使用 Mercator，只呈现有效的普通区域坐标。
  const validCenter = useRef<Coordinate>(validCoordinate(input.center) ? input.center : [0, 0])
  if (validCoordinate(input.center)) validCenter.current = input.center
  const props = { ...input, center: validCenter.current }
  const container = useRef<HTMLDivElement>(null),
    mapRef = useRef<GLMap | null>(null)
  const latest = useRef(props)
  latest.current = props
  const [ready, setReady] = useState(false),
    [fallback, setFallback] = useState(false)
  const [fallbackCenter, setFallbackCenter] = useState<Coordinate>(props.center)
  const [tiles, setTiles] = useState(true),
    [tileError, setTileError] = useState(false)
  const [zoom, setZoom] = useState(13.8)

  useEffect(() => {
    let disposed = false
    let observer: ResizeObserver | undefined
    let map: GLMap | undefined
    void import('maplibre-gl')
      .then((gl) => {
        if (disposed || !container.current) return
        try {
          // MapLibre 6 的默认相对 worker URL 在 Vite 哈希分包后不再成立。
          // 显式让 Vite 构建 worker 及其依赖，并在创建地图前设置同源 URL。
          gl.setWorkerUrl(workerUrl)
          map = new gl.Map({
            container: container.current,
            style: style(true),
            center: latest.current.center,
            zoom: 13.8,
            minZoom: 2,
            maxZoom: 19,
            attributionControl: { compact: true },
            renderWorldCopies: false,
          })
          mapRef.current = map
          map.on('error', (e) => {
            if (disposed) return
            if ('sourceId' in e && e.sourceId === 'osm') setTileError(true)
            else if (/worker|webgl/i.test(e.error?.message || '')) {
              map?.remove()
              mapRef.current = null
              setFallback(true)
            }
          })
          map.on('zoom', () => {
            if (!disposed && map) setZoom(map.getZoom())
          })
          map.on('click', (e) => {
            if (!map?.getLayer('pois')) return
            if (!latest.current.drawing) {
              const hit = map.queryRenderedFeatures(e.point, { layers: ['pois'] })[0]
              if (hit?.id != null) {
                latest.current.onSelect(String(hit.id))
                return
              }
            }
            latest.current.onPick([
              Number(e.lngLat.lng.toFixed(6)),
              Number(e.lngLat.lat.toFixed(6)),
            ])
          })
          map.on('load', () => {
            if (!map || disposed) return
            for (const name of ['grid', 'radius', 'fence', 'vertices', 'center', 'pois'])
              map.addSource(name, { type: 'geojson', data: empty() })
            map.addLayer({
              id: 'grid-lines',
              type: 'line',
              source: 'grid',
              paint: { 'line-color': '#466158', 'line-width': 0.5, 'line-opacity': 0.3 },
            })
            map.addLayer({
              id: 'radius-fill',
              type: 'fill',
              source: 'radius',
              paint: { 'fill-color': '#abd97d', 'fill-opacity': 0.12 },
            })
            map.addLayer({
              id: 'radius-line',
              type: 'line',
              source: 'radius',
              paint: { 'line-color': '#b8f26b', 'line-width': 1.5, 'line-dasharray': [4, 3] },
            })
            map.addLayer({
              id: 'fence-fill',
              type: 'fill',
              source: 'fence',
              filter: ['==', '$type', 'Polygon'],
              paint: { 'fill-color': '#7dbcea', 'fill-opacity': 0.14 },
            })
            map.addLayer({
              id: 'fence-line',
              type: 'line',
              source: 'fence',
              paint: { 'line-color': '#8bccff', 'line-width': 2 },
            })
            map.addLayer({
              id: 'vertices',
              type: 'circle',
              source: 'vertices',
              paint: {
                'circle-color': '#8bccff',
                'circle-radius': 4,
                'circle-stroke-color': '#122325',
                'circle-stroke-width': 2,
              },
            })
            map.addLayer({
              id: 'pois',
              type: 'circle',
              source: 'pois',
              paint: {
                'circle-color': '#d3fc97',
                'circle-radius': 7,
                'circle-stroke-color': '#24362a',
                'circle-stroke-width': 3,
              },
            })
            map.addLayer({
              id: 'center',
              type: 'circle',
              source: 'center',
              paint: {
                'circle-color': '#fff',
                'circle-radius': 4,
                'circle-stroke-color': '#425c3c',
                'circle-stroke-width': 3,
              },
            })
            setReady(true)
          })
          observer = new ResizeObserver(() => map?.resize())
          observer.observe(container.current)
        } catch {
          setFallback(true)
        }
      })
      .catch(() => {
        if (!disposed) setFallback(true)
      })
    return () => {
      disposed = true
      observer?.disconnect()
      map?.remove()
      mapRef.current = null
    }
  }, [])

  useEffect(() => {
    const map = mapRef.current
    if (!ready || !map) return
    const set = (id: string, value: FeatureCollection) =>
      (map.getSource(id) as GeoJSONSource)?.setData(value)
    set('radius', {
      type: 'FeatureCollection',
      features: [
        {
          type: 'Feature',
          properties: {},
          geometry: { type: 'Polygon', coordinates: [circleRing(props.center, props.radius)] },
        },
      ],
    })
    set('center', {
      type: 'FeatureCollection',
      features: [
        { type: 'Feature', properties: {}, geometry: { type: 'Point', coordinates: props.center } },
      ],
    })
    set('pois', props.pois || empty())
    set(
      'fence',
      props.polygon.length < 2
        ? empty()
        : {
            type: 'FeatureCollection',
            features: [
              {
                type: 'Feature',
                properties: {},
                geometry:
                  props.polygon.length < 3
                    ? { type: 'LineString', coordinates: props.polygon }
                    : { type: 'Polygon', coordinates: [closeRing(props.polygon)] },
              },
            ],
          },
    )
    set('vertices', {
      type: 'FeatureCollection',
      features: props.polygon.map((p) => ({
        type: 'Feature',
        properties: {},
        geometry: { type: 'Point', coordinates: p },
      })),
    })
    const [lon, lat] = props.center
    const lines: FeatureCollection = empty()
    for (let i = -10; i <= 10; i++) {
      const x = Math.floor(lon * 100) / 100 + i * 0.01,
        y = Math.floor(lat * 100) / 100 + i * 0.01
      lines.features.push(
        {
          type: 'Feature',
          properties: {},
          geometry: {
            type: 'LineString',
            coordinates: [
              [x, lat - 0.1],
              [x, lat + 0.1],
            ],
          },
        },
        {
          type: 'Feature',
          properties: {},
          geometry: {
            type: 'LineString',
            coordinates: [
              [lon - 0.1, y],
              [lon + 0.1, y],
            ],
          },
        },
      )
    }
    set('grid', lines)
    map.setPaintProperty('pois', 'circle-radius', [
      'case',
      ['==', ['to-string', ['id']], props.selected || ''],
      10,
      7,
    ])
    map.getCanvas().style.cursor = props.drawing ? 'crosshair' : 'grab'
  }, [ready, props.center, props.radius, props.polygon, props.pois, props.selected, props.drawing])

  useEffect(() => {
    const map = mapRef.current
    if (!ready || !map) return
    if (!tiles) {
      if (map.getLayer('basemap')) map.setLayoutProperty('basemap', 'visibility', 'none')
    } else if (map.getLayer('basemap')) {
      map.setLayoutProperty('basemap', 'visibility', 'visible')
    }
  }, [tiles, ready])
  useEffect(() => {
    if (props.focus && validCoordinate(props.focus.point))
      mapRef.current?.flyTo({ center: props.focus.point, zoom: 15, duration: 700 })
  }, [props.focus])
  useEffect(() => {
    setFallbackCenter(props.center)
  }, [props.center])
  useEffect(() => {
    if (props.focus && validCoordinate(props.focus.point)) setFallbackCenter(props.focus.point)
  }, [props.focus])

  return (
    <div className="map-frame">
      <div ref={container} className="map-canvas" aria-label="空间地图" />
      {fallback && <CoordinateFallback {...props} viewCenter={fallbackCenter} />}
      <div className="map-top-label">
        <span className="live-dot" />
        {tiles && !fallback ? 'OPENSTREETMAP' : '坐标网格'}
        <span className="map-label-separator" />
        WGS84
      </div>
      {(tileError || fallback || !validCoordinate(input.center)) && (
        <div className="map-warning">
          <TriangleAlert size={14} />
          {!validCoordinate(input.center)
            ? '地图保留上次有效中心；纬度需在 ±85° 内'
            : fallback
              ? 'WebGL 不可用，已切换坐标示意图'
              : '底图加载异常，可关闭底图继续空间操作'}
        </div>
      )}
      <div className="map-actions">
        <button aria-label="放大地图" onClick={() => mapRef.current?.zoomIn()} disabled={fallback}>
          <Plus size={17} />
        </button>
        <button aria-label="缩小地图" onClick={() => mapRef.current?.zoomOut()} disabled={fallback}>
          <Minus size={17} />
        </button>
        <span />
        <button
          aria-label="定位中心"
          onClick={() => {
            setFallbackCenter(props.center)
            mapRef.current?.flyTo({ center: props.center, zoom: 14, duration: 700 })
          }}
        >
          <Crosshair size={17} />
        </button>
        <button
          aria-label={tiles ? '关闭底图' : '显示底图'}
          title={tiles ? '关闭底图' : '显示底图'}
          onClick={() => setTiles((v) => !v)}
        >
          <Layers size={17} />
        </button>
      </div>
      <div className="map-hint">
        <MapIcon size={14} />
        {props.drawing
          ? `点击地图添加围栏顶点 · 已选 ${props.polygon.length} 点`
          : '点击地图设置中心 · 滚轮缩放 · 拖动平移'}
      </div>
      <div className="map-coordinates">
        {Math.abs(props.center[0]).toFixed(5)}° {props.center[0] < 0 ? 'W' : 'E'} /{' '}
        {Math.abs(props.center[1]).toFixed(5)}° {props.center[1] < 0 ? 'S' : 'N'}{' '}
        <span>{fallback ? '坐标示意' : `z ${zoom.toFixed(1)}`}</span>
      </div>
    </div>
  )
}

// 降级视图以局部经纬度坐标投影显示；不把缺失的真实道路数据伪造成底图。
function CoordinateFallback(props: Props & { viewCenter: Coordinate }) {
  const { viewCenter } = props
  const span = 0.045
  const project = (p: number[]) => [
    500 + ((((p[0] - viewCenter[0] + 540) % 360) - 180) / span) * 1000,
    350 - ((p[1] - viewCenter[1]) / span) * 1000,
  ]
  const path = (points: Coordinate[]) => points.map((p) => project(p).join(',')).join(' ')
  const c = project(props.center)
  return (
    <svg
      className="coordinate-fallback"
      viewBox="0 0 1000 700"
      preserveAspectRatio="none"
      role="img"
      aria-label="WGS84 坐标示意图"
      onClick={(e) => {
        const rect = e.currentTarget.getBoundingClientRect()
        props.onPick([
          Number((viewCenter[0] + ((e.clientX - rect.left) / rect.width - 0.5) * span).toFixed(6)),
          Number(
            (viewCenter[1] - ((e.clientY - rect.top) / rect.height - 0.5) * 0.7 * span).toFixed(6),
          ),
        ])
      }}
    >
      <defs>
        <pattern id="grid" width="80" height="80" patternUnits="userSpaceOnUse">
          <path d="M 80 0 L 0 0 0 80" fill="none" stroke="#35443c" strokeWidth="1" />
        </pattern>
      </defs>
      <rect width="1000" height="700" fill="#19231f" />
      <rect width="1000" height="700" fill="url(#grid)" />
      <polygon
        points={path(circleRing(props.center, props.radius))}
        fill="#b8f26b18"
        stroke="#b8f26b"
        strokeDasharray="5 4"
      />
      {props.polygon.length > 1 && (
        <polygon points={path(props.polygon)} fill="#8bccff20" stroke="#8bccff" strokeWidth="2" />
      )}
      {props.polygon.map((p, i) => {
        const xy = project(p)
        return <circle key={i} cx={xy[0]} cy={xy[1]} r="4" fill="#8bccff" />
      })}
      <circle cx={c[0]} cy={c[1]} r="4" fill="white" />
      {props.pois?.features.map((f) => {
        const p = project(f.geometry.coordinates)
        return (
          <g
            key={f.id}
            onClick={(e) => {
              e.stopPropagation()
              props.onSelect(String(f.id))
            }}
            style={{ cursor: 'pointer' }}
          >
            <circle
              cx={p[0]}
              cy={p[1]}
              r={String(f.id) === props.selected ? 11 : 7}
              fill="#b8f26b"
              stroke="#203329"
              strokeWidth="3"
            />
            <text x={p[0] + 14} y={p[1] + 4} fill="#d3e3d6" fontSize="13">
              {f.properties.name}
            </text>
          </g>
        )
      })}
    </svg>
  )
}
