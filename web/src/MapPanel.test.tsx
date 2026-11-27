import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { expect, it, vi } from 'vitest'
import MapPanel from './MapPanel'
const { setWorkerUrl, remove, flyTo, fail } = vi.hoisted(() => ({
  setWorkerUrl: vi.fn(),
  remove: vi.fn(),
  flyTo: vi.fn(),
  fail: { enabled: false },
}))
vi.mock('maplibre-gl', () => ({
  setWorkerUrl,
  Map: class {
    constructor() {
      if (fail.enabled) throw new Error('WebGL unavailable')
    }
    on(event: string, callback: () => void) {
      if (event === 'load') queueMicrotask(callback)
    }
    addSource() {}
    addLayer() {}
    setPaintProperty() {}
    getSource() {
      return { setData() {} }
    }
    getCanvas() {
      return { style: {} }
    }
    getLayer() {
      return {}
    }
    setLayoutProperty() {}
    remove = remove
    flyTo = flyTo
  },
}))
it('sets an explicitly bundled worker URL before loading GeoJSON layers', async () => {
  vi.stubGlobal(
    'ResizeObserver',
    class {
      observe() {}
      disconnect() {}
    },
  )
  const { unmount } = render(
    <MapPanel
      center={[121, 31]}
      radius={1000}
      polygon={[]}
      drawing={false}
      pois={null}
      selected={null}
      focus={null}
      onPick={() => {}}
      onSelect={() => {}}
    />,
  )
  await waitFor(() => expect(setWorkerUrl).toHaveBeenCalled())
  expect(setWorkerUrl.mock.calls[0][0]).toContain('worker')
  expect(screen.queryByText(/WebGL 不可用/)).not.toBeInTheDocument()
  unmount()
  expect(remove).toHaveBeenCalled()
  vi.unstubAllGlobals()
})
it('does not send invalid form coordinates to map navigation', async () => {
  vi.stubGlobal(
    'ResizeObserver',
    class {
      observe() {}
      disconnect() {}
    },
  )
  flyTo.mockClear()
  const props = {
    center: [121, 31] as [number, number],
    radius: 1000,
    polygon: [],
    drawing: false,
    pois: null,
    selected: null,
    focus: null,
    onPick: () => {},
    onSelect: () => {},
  }
  const view = render(<MapPanel {...props} />)
  await waitFor(() => expect(screen.getByRole('button', { name: '定位中心' })).toBeEnabled())
  await waitFor(() => expect(setWorkerUrl).toHaveBeenCalled())
  view.rerender(<MapPanel {...props} center={[121, 91]} />)
  fireEvent.click(screen.getByRole('button', { name: '定位中心' }))
  expect(flyTo).not.toHaveBeenCalledWith(expect.objectContaining({ center: [121, 91] }))
  view.unmount()
  vi.unstubAllGlobals()
})
it('reprojects the SVG fallback when query center or selected POI changes', async () => {
  fail.enabled = true
  const props = {
    center: [121, 31] as [number, number],
    radius: 1000,
    polygon: [],
    drawing: false,
    pois: null,
    selected: null,
    focus: null,
    onPick: () => {},
    onSelect: () => {},
  }
  const view = render(<MapPanel {...props} />)
  await screen.findByRole('img', { name: 'WGS84 坐标示意图' })
  view.rerender(<MapPanel {...props} center={[120, 30]} />)
  let svg = screen.getByRole('img', { name: 'WGS84 坐标示意图' })
  expect(svg.querySelector('circle')?.getAttribute('cx')).toBe('500')
  expect(svg.querySelector('circle')?.getAttribute('cy')).toBe('350')
  view.rerender(
    <MapPanel {...props} center={[120, 30]} focus={{ point: [120.01, 30], revision: 1 }} />,
  )
  svg = screen.getByRole('img', { name: 'WGS84 坐标示意图' })
  expect(Number(svg.querySelector('circle')?.getAttribute('cx'))).toBeLessThan(500)
  fireEvent.click(screen.getByRole('button', { name: '定位中心' }))
  expect(svg.querySelector('circle')?.getAttribute('cx')).toBe('500')
  view.unmount()
  fail.enabled = false
})
