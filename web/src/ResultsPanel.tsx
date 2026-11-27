import { Braces, ChevronRight, Download, MapPin, Radar, Terminal, X } from 'lucide-react'
import { useState } from 'react'
import type { Execution, POICollection } from './types'
import { formatDistance } from './lib'

type Props = {
  pois: POICollection | null
  records: Execution[]
  selected: string | null
  stale: boolean
  onSelect: (id: string) => void
  onClear: () => void
}
export default function ResultsPanel({ pois, records, selected, stale, onSelect, onClear }: Props) {
  const [tab, setTab] = useState<'results' | 'trace'>('results')
  function download() {
    if (!pois) return
    const url = URL.createObjectURL(
      new Blob([JSON.stringify(pois, null, 2)], { type: 'application/geo+json' }),
    )
    const a = document.createElement('a')
    a.href = url
    a.download = 'pulsegeo-results.geojson'
    a.click()
    setTimeout(() => URL.revokeObjectURL(url), 1000)
  }
  return (
    <aside className="results-panel panel">
      <div className="panel-heading">
        <span className="eyebrow">INSPECTOR</span>
        <span className="subtle">可验证的空间结果</span>
      </div>
      <div className="tabs" role="tablist" aria-label="结果面板">
        <button role="tab" aria-selected={tab === 'results'} onClick={() => setTab('results')}>
          <MapPin size={15} />
          检索结果 <span>{pois?.features.length ?? '—'}</span>
        </button>
        <button role="tab" aria-selected={tab === 'trace'} onClick={() => setTab('trace')}>
          <Terminal size={15} />
          执行记录 <span>{records.length}</span>
        </button>
      </div>
      <div className="result-scroll">
        {tab === 'results' ? (
          <>
            {pois ? (
              <>
                <div className="results-summary">
                  <div>
                    <strong>{pois.features.length}</strong>
                    <span> 个地点</span>
                  </div>
                  <button className="icon-button" aria-label="导出 GeoJSON" onClick={download}>
                    <Download size={16} />
                  </button>
                </div>
                <div className="source-note">
                  <span className={'status-dot ' + (pois.source === 'demo' ? 'amber' : '')} />
                  {pois.source === 'demo' ? '本次结果为合成数据' : `来源：${pois.source}`} · WGS84
                </div>
                {stale && <div className="stale-note">查询条件已变更，以下为上次结果</div>}
                {pois.features.length === 0 && (
                  <div className="empty-small">
                    该区域没有符合条件的 POI。可以扩大半径或切换分类后重试。
                  </div>
                )}
                {pois.features.map((feature, index) => (
                  <button
                    className={'poi-card ' + (selected === String(feature.id) ? 'selected' : '')}
                    key={feature.id}
                    onClick={() => onSelect(String(feature.id))}
                  >
                    <div className="poi-icon">
                      <MapPin size={17} />
                      <span>{String(index + 1).padStart(2, '0')}</span>
                    </div>
                    <div className="poi-content">
                      <strong>{feature.properties.name}</strong>
                      <div>
                        <span className="category-tag">{feature.properties.category}</span>
                        <span>{formatDistance(feature.properties.distance_meters)}</span>
                      </div>
                      <small>
                        {feature.geometry.coordinates.map((n) => n.toFixed(5)).join(', ')}
                      </small>
                    </div>
                    <ChevronRight size={14} />
                  </button>
                ))}
                <div className="result-footnote">
                  <Braces size={14} />
                  GeoJSON · 坐标与计算结果可导出核对
                </div>
              </>
            ) : (
              <div className="empty-state">
                <div className="empty-orbit">
                  <Radar size={30} />
                </div>
                <h3>等待首次检索</h3>
                <p>
                  在地图选择中心与搜索半径，
                  <br />
                  点击「检索区域」发现周围地点。
                </p>
                <span className="empty-label">DETERMINISTIC BY DESIGN</span>
              </div>
            )}
          </>
        ) : (
          <>
            <div className="trace-heading">
              <span>最近 {records.length} 条 · 仅本次页面</span>
              <button
                className="icon-button"
                aria-label="清空执行记录"
                onClick={onClear}
                disabled={!records.length}
              >
                <X size={14} />
              </button>
            </div>
            {!records.length && (
              <div className="empty-state">
                <Terminal size={28} />
                <h3>每一步都有迹可循</h3>
                <p>
                  执行查询或发送任务后，
                  <br />
                  这里会显示参数、耗时和原始结果。
                </p>
              </div>
            )}
            {records.map((record) => (
              <details className="trace-item" key={record.id} open={record.status === 'error'}>
                <summary>
                  <span className={'trace-status ' + record.status} />
                  <strong>{record.name}</strong>
                  <span>{Math.round(record.elapsed)} ms</span>
                  <ChevronRight size={14} />
                </summary>
                <div className="trace-detail">
                  <span>INPUT</span>
                  <pre>{JSON.stringify(record.input, null, 2)}</pre>
                  {record.error && <p className="trace-error">{record.error}</p>}
                  <span>{record.status === 'canceled' ? 'CANCELED' : 'OUTPUT'}</span>
                  {record.result !== undefined && (
                    <pre>{JSON.stringify(record.result, null, 2)}</pre>
                  )}
                </div>
              </details>
            ))}
          </>
        )}
      </div>
      <div className="inspector-footer">
        <span className="status-dot" />
        空间计算由 Go 执行<span>v0.1</span>
      </div>
    </aside>
  )
}
