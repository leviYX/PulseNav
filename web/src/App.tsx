import { useCallback, useEffect, useRef, useState } from 'react'
import {
  Activity,
  ArrowUpRight,
  Check,
  ChevronDown,
  Circle,
  Compass,
  Crosshair,
  Hexagon,
  LoaderCircle,
  MapPin,
  MousePointer2,
  RotateCcw,
  Search,
  Square,
  Undo2,
  X,
} from 'lucide-react'
import ChatPanel from './ChatPanel'
import MapPanel from './MapPanel'
import ResultsPanel from './ResultsPanel'
import { isPOICollection, makeSearch, postJSON } from './lib'
import type {
  AgentResult,
  ChatMessage,
  Coordinate,
  Execution,
  Health,
  POICollection,
  SearchRequest,
} from './types'

const INITIAL: Coordinate = [121.4737, 31.2304]
export default function App() {
  const [health, setHealth] = useState<Health | null>(null),
    [healthError, setHealthError] = useState(false)
  const [center, setCenter] = useState<Coordinate>(INITIAL),
    [radius, setRadius] = useState(1000),
    [category, setCategory] = useState('cafe')
  const [polygon, setPolygon] = useState<Coordinate[]>([]),
    [drawing, setDrawing] = useState(false)
  const [pois, setPois] = useState<POICollection | null>(null),
    [selected, setSelected] = useState<string | null>(null)
  const [focus, setFocus] = useState<{ point: Coordinate; revision: number } | null>(null)
  const [messages, setMessages] = useState<ChatMessage[]>([]),
    [records, setRecords] = useState<Execution[]>([])
  const [busy, setBusy] = useState<'search' | 'agent' | null>(null),
    [error, setError] = useState('')
  const [resultSignature, setResultSignature] = useState<string | null>(null)
  const id = useRef(0),
    active = useRef<AbortController | null>(null)
  const signature = JSON.stringify({ center, radius, category, polygon })
  useEffect(() => {
    const controller = new AbortController()
    fetch('/healthz', { signal: controller.signal })
      .then((r) => {
        if (!r.ok) throw new Error()
        return r.json()
      })
      .then((v: Health) => {
        if (!controller.signal.aborted) setHealth(v)
      })
      .catch(() => {
        if (!controller.signal.aborted) setHealthError(true)
      })
    return () => {
      controller.abort()
      active.current?.abort()
    }
  }, [])
  const finishDrawing = useCallback(() => {
    if (polygon.length < 3) {
      setError('围栏至少需要 3 个顶点')
      return
    }
    setDrawing(false)
    setError('')
  }, [polygon.length])
  useEffect(() => {
    function key(e: KeyboardEvent) {
      if (!drawing || ['INPUT', 'TEXTAREA', 'SELECT'].includes((e.target as HTMLElement)?.tagName))
        return
      if (e.key === 'Escape') {
        setDrawing(false)
        setPolygon([])
      }
      if (e.key === 'Enter') {
        e.preventDefault()
        finishDrawing()
      }
    }
    window.addEventListener('keydown', key)
    return () => window.removeEventListener('keydown', key)
  }, [drawing, finishDrawing])

  function addRecord(record: Execution) {
    setRecords((prev) => [record, ...prev].slice(0, 30))
  }
  function applyPOIs(value: unknown, querySignature: string) {
    if (isPOICollection(value)) {
      setPois(value)
      setSelected(null)
      setResultSignature(querySignature)
    }
  }
  function applyAgentPOIs(value: unknown, args: unknown) {
    if (!isPOICollection(value)) return
    const query = args as Partial<SearchRequest> | undefined
    // 使用服务端实际执行的工具参数同步范围，不能把当前地图条件冒充 Agent 的查询条件。
    if (
      query &&
      Array.isArray(query.center) &&
      query.center.length === 2 &&
      typeof query.radius_meters === 'number'
    ) {
      const ring = query.polygon?.coordinates?.[0]
      const vertices = ring ? (ring.slice(0, -1) as Coordinate[]) : []
      try {
        const actual = makeSearch(query.center, query.radius_meters, query.category || '', vertices)
        setCenter(actual.center)
        setRadius(actual.radius_meters)
        setCategory(actual.category || '')
        setPolygon(vertices)
        setDrawing(false)
        setFocus({ point: actual.center, revision: ++id.current })
        applyPOIs(
          value,
          JSON.stringify({
            center: actual.center,
            radius: actual.radius_meters,
            category: actual.category || '',
            polygon: vertices,
          }),
        )
        return
      } catch {
        /* 旧服务或不同查询形态保留结果，但标明范围不能与当前控件对齐。 */
      }
    }
    applyPOIs(value, '')
  }
  async function run(kind: 'search' | 'agent', input: unknown, text?: string) {
    if (active.current) return
    setError('')
    const controller = new AbortController()
    active.current = controller
    setBusy(kind)
    const started = performance.now(),
      requestId = ++id.current,
      name = kind === 'search' ? 'search_poi' : 'agent.run'
    if (text) setMessages((prev) => [...prev, { id: ++id.current, role: 'user', content: text }])
    try {
      const result = await postJSON<POICollection | AgentResult>(
        kind === 'search' ? '/v1/poi/search' : '/v1/agent/run',
        input,
        controller.signal,
      )
      if (controller.signal.aborted) return
      if (kind === 'search') applyPOIs(result, signature)
      else {
        const response = result as AgentResult
        setMessages((prev) => [
          ...prev,
          {
            id: ++id.current,
            role: 'assistant',
            content: response.answer || '任务已完成，请查看工具结果。',
          },
        ])
        for (const tool of response.tool_results || []) {
          if (tool.name === 'search_poi') applyAgentPOIs(tool.result, tool.arguments)
          addRecord({
            id: ++id.current,
            name: tool.name,
            input: tool.arguments ?? '此服务未返回工具参数',
            result: tool.result,
            elapsed: tool.elapsed_ms ?? 0,
            status: 'success',
          })
        }
      }
      addRecord({
        id: requestId,
        name,
        input,
        result,
        elapsed: performance.now() - started,
        status: 'success',
      })
    } catch (err) {
      if (controller.signal.aborted) return
      const message = err instanceof Error ? err.message : '请求失败，请稍后重试'
      setError(message)
      addRecord({
        id: requestId,
        name,
        input,
        elapsed: performance.now() - started,
        status: 'error',
        error: message,
      })
      if (kind === 'agent')
        setMessages((prev) => [
          ...prev,
          { id: ++id.current, role: 'assistant', content: message, error: true },
        ])
    } finally {
      if (controller.signal.aborted)
        addRecord({
          id: requestId,
          name,
          input,
          elapsed: performance.now() - started,
          status: 'canceled',
          error: '请求已取消',
        })
      if (active.current === controller) {
        active.current = null
        setBusy(null)
      }
    }
  }
  function cancel() {
    active.current?.abort()
    active.current = null
    setBusy(null)
    setError('')
  }
  function search() {
    if (drawing) {
      setError('请先完成或取消围栏绘制')
      return
    }
    try {
      void run('search', makeSearch(center, radius, category, polygon))
    } catch (e) {
      setError((e as Error).message)
    }
  }
  function send(text: string, include: boolean) {
    try {
      const context = include
        ? `\n\n当前地图上下文（WGS84，坐标为[经度,纬度]，距离单位米）：${JSON.stringify(makeSearch(center, radius, category, polygon))}\n如需检索请使用这些空间条件；本次请求独立于之前的任务。`
        : ''
      if (new TextEncoder().encode(text + context).length > 16000)
        throw new Error('任务与地图上下文合计超过 16000 字节，请缩短任务或取消附带地图范围')
      void run('agent', { message: text + context }, text)
    } catch (e) {
      setError((e as Error).message)
    }
  }
  function selectPOI(key: string) {
    const poi = pois?.features.find((p) => String(p.id) === key)
    setSelected(key)
    if (poi) setFocus({ point: poi.geometry.coordinates as Coordinate, revision: ++id.current })
  }
  function pick(point: Coordinate) {
    if (busy) return
    if (drawing) {
      if (polygon.length >= 256) {
        setError('围栏最多 256 个顶点')
        return
      }
      setPolygon((prev) => [...prev, point])
    } else setCenter(point)
  }
  function reset() {
    setCenter(INITIAL)
    setRadius(1000)
    setCategory('cafe')
    setPolygon([])
    setDrawing(false)
    setError('')
    setFocus({ point: INITIAL, revision: ++id.current })
  }

  return (
    <div className="app-shell">
      <header className="topbar">
        <a className="brand" href="/" aria-label="PulseGeo 工作台">
          <span className="brand-icon">
            <Activity size={23} strokeWidth={2.5} />
          </span>
          <span>
            Pulse<span className="brand-geo">Geo</span>
          </span>
          <span className="version-tag">WORKSPACE</span>
        </a>
        <nav>
          <span className="nav-active">
            <Compass size={15} />
            空间探索
          </span>
          <span className="nav-divider" />
          <span className="nav-caption">Spatial intelligence, grounded.</span>
        </nav>
        <div className="topbar-status">
          <span className={'connection ' + (healthError ? 'offline' : '')}>
            <span className="status-dot" />
            {healthError ? '服务连接失败' : health ? '服务已连接' : '连接中'}
          </span>
          <a href="https://geojson.org/" target="_blank" rel="noreferrer" className="docs-link">
            GeoJSON <ArrowUpRight size={13} />
          </a>
          <span className="profile-avatar">PG</span>
        </div>
      </header>
      <div className="workspace-title">
        <div>
          <span className="eyebrow">EXPLORATION / 01</span>
          <h1>
            空间探索工作台 <span>把意图落在地图上</span>
          </h1>
        </div>
        <div className={'data-badge ' + (health?.poi_source === 'demo' ? 'demo' : '')}>
          <span className="status-dot" />
          <span>
            {health?.poi_source === 'demo'
              ? '合成演示数据'
              : health
                ? 'PostGIS 数据源'
                : '数据源连接中'}
          </span>
          <span className="badge-divider" />
          WGS84
        </div>
      </div>
      <main className="workspace">
        <ChatPanel
          health={health}
          messages={messages}
          busy={!!busy}
          onSend={send}
          onCancel={cancel}
          onReset={() => setMessages([])}
        />
        <section className="explore-panel panel">
          <div className="map-panel-heading">
            <div>
              <MapPin size={17} />
              <strong>区域探索</strong>
              <span>REGION EXPLORER</span>
            </div>
            <button className="text-button" onClick={reset} disabled={!!busy}>
              <RotateCcw size={13} />
              重置视图
            </button>
          </div>
          <div className="search-controls">
            <div className="coordinate-fields">
              <label>
                经度
                <input
                  aria-label="中心经度"
                  type="number"
                  disabled={!!busy}
                  step="0.0001"
                  value={center[0]}
                  onChange={(e) => setCenter([Number(e.target.value), center[1]])}
                />
              </label>
              <span>/</span>
              <label>
                纬度
                <input
                  aria-label="中心纬度"
                  type="number"
                  disabled={!!busy}
                  step="0.0001"
                  value={center[1]}
                  onChange={(e) => setCenter([center[0], Number(e.target.value)])}
                />
              </label>
            </div>
            <label className="radius-field">
              半径
              <div>
                <input
                  aria-label="搜索半径"
                  type="number"
                  disabled={!!busy}
                  min="1"
                  max="50000"
                  step="100"
                  value={radius}
                  onChange={(e) => setRadius(Number(e.target.value))}
                />
                <span>m</span>
              </div>
            </label>
            <label className="category-field">
              地点分类
              <div>
                <select
                  aria-label="地点分类"
                  disabled={!!busy}
                  value={category}
                  onChange={(e) => setCategory(e.target.value)}
                >
                  <option value="cafe">咖啡店</option>
                  <option value="shop">商店</option>
                  <option value="">全部分类</option>
                  {!['cafe', 'shop', ''].includes(category) && (
                    <option value={category}>{category}</option>
                  )}
                </select>
                <ChevronDown size={12} />
              </div>
            </label>
          </div>
          <div className="map-toolbar">
            <div className="mode-controls">
              <button
                className={!drawing ? 'active' : ''}
                onClick={() => setDrawing(false)}
                disabled={drawing}
              >
                <MousePointer2 size={14} />
                选中心
              </button>
              <button
                className={drawing ? 'active' : ''}
                onClick={() => {
                  setPolygon([])
                  setDrawing(true)
                  setError('')
                }}
                disabled={!!busy}
              >
                <Hexagon size={14} />
                绘制围栏
              </button>
            </div>
            {drawing ? (
              <div className="drawing-actions">
                <button
                  aria-label="撤销顶点"
                  disabled={!polygon.length}
                  onClick={() => setPolygon((prev) => prev.slice(0, -1))}
                >
                  <Undo2 size={14} />
                </button>
                <button onClick={finishDrawing} disabled={polygon.length < 3}>
                  <Check size={13} />
                  完成
                </button>
                <button
                  aria-label="取消围栏"
                  onClick={() => {
                    setPolygon([])
                    setDrawing(false)
                  }}
                >
                  <X size={14} />
                </button>
              </div>
            ) : polygon.length > 0 ? (
              <button className="fence-badge" onClick={() => setPolygon([])}>
                围栏 ∩ 半径
                <X size={12} />
              </button>
            ) : (
              <span className="toolbar-caption">
                <Circle size={9} />
                半径范围检索
              </span>
            )}
          </div>
          <MapPanel
            center={center}
            radius={radius > 0 && radius <= 50000 ? radius : 1000}
            polygon={polygon}
            drawing={drawing}
            pois={pois}
            selected={selected}
            focus={focus}
            onPick={pick}
            onSelect={selectPOI}
          />
          {error && (
            <div className="error-banner" role="alert">
              <span>{error}</span>
              <button className="icon-button" aria-label="关闭提示" onClick={() => setError('')}>
                <X size={14} />
              </button>
            </div>
          )}
          <div className="map-bottom">
            <div>
              <Crosshair size={16} />
              <span>
                {drawing
                  ? '绘制围栏中'
                  : polygon.length
                    ? `${polygon.length} 个顶点 · 与半径取交集`
                    : '中心 + 半径'}
                <small>
                  {pois
                    ? `${pois.features.length} 个地点 · ${resultSignature !== signature ? '条件已变更' : '结果已就绪'}`
                    : '空间条件已准备'}
                </small>
              </span>
            </div>
            {busy ? (
              <button className="search-button is-loading" aria-label="停止请求" onClick={cancel}>
                <LoaderCircle size={16} className="spin" />
                停止请求
                <Square size={12} />
              </button>
            ) : (
              <button
                className="search-button"
                aria-label="检索区域"
                onClick={search}
                disabled={drawing || !health}
              >
                <Search size={16} />
                检索区域<span>↵</span>
              </button>
            )}
          </div>
        </section>
        <ResultsPanel
          pois={pois}
          records={records}
          selected={selected}
          stale={!!pois && resultSignature !== signature}
          onSelect={selectPOI}
          onClear={() => setRecords([])}
        />
      </main>
      <footer className="workspace-footer">
        <span>
          <span className="status-dot" />
          确定性空间计算 · Go + PostGIS
        </span>
        <span>
          LLM 理解意图<span className="footer-arrow">→</span>工具执行计算
          <span className="footer-arrow">→</span>结果可验证
        </span>
        <span>
          PULSE / GEO <small>© 2026</small>
        </span>
      </footer>
    </div>
  )
}
