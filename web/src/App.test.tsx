import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'vitest'
import App from './App'

// jsdom 无 WebGL；地图仍在真实浏览器单独检查。此处检查真实组件到 HTTP 的状态流。
vi.mock('./MapPanel', () => ({ default: () => <div aria-label="空间地图" /> }))
afterEach(() => vi.restoreAllMocks())
const result = {
  type: 'FeatureCollection',
  source: 'demo',
  features: [
    {
      type: 'Feature',
      id: 'demo-1',
      geometry: { type: 'Point', coordinates: [121.4737, 31.2304] },
      properties: { name: '演示咖啡 A', category: 'cafe', distance_meters: 0 },
    },
  ],
}
function mockHealth() {
  return vi.spyOn(globalThis, 'fetch').mockImplementation(async (input) => {
    if (String(input) === '/healthz')
      return new Response(
        JSON.stringify({ status: 'ok', poi_source: 'demo', agent_enabled: false }),
      )
    return new Response(JSON.stringify(result))
  })
}
it('works without a model, searches real API and labels synthetic data', async () => {
  const fetcher = mockHealth()
  render(<App />)
  expect(await screen.findByText('合成演示数据')).toBeInTheDocument()
  expect(screen.getByRole('button', { name: '发送消息' })).toBeDisabled()
  fireEvent.click(screen.getByRole('button', { name: '检索区域' }))
  expect(await screen.findByText('演示咖啡 A')).toBeInTheDocument()
  expect(
    fetcher.mock.calls.some(
      ([url, init]) =>
        String(url) === '/v1/poi/search' && JSON.parse(String(init?.body)).radius_meters === 1000,
    ),
  ).toBe(true)
  fireEvent.click(screen.getByRole('tab', { name: /执行记录/ }))
  expect(screen.getByText('search_poi')).toBeInTheDocument()
})
it('shows request errors without pretending there are fresh results', async () => {
  const fetcher = mockHealth()
  render(<App />)
  await screen.findByText('合成演示数据')
  fetcher.mockResolvedValue(
    new Response(JSON.stringify({ error: { message: '围栏无效', code: 'invalid_argument' } }), {
      status: 400,
    }),
  )
  fireEvent.click(screen.getByRole('button', { name: '检索区域' }))
  expect(await screen.findByRole('alert')).toHaveTextContent('围栏无效')
  expect(screen.getByText('等待首次检索')).toBeInTheDocument()
})
it('aborts an active request and does not apply its late result', async () => {
  const fetcher = mockHealth()
  render(<App />)
  await screen.findByText('合成演示数据')
  let resolve!: (r: Response) => void
  let signal: AbortSignal | undefined
  fetcher.mockImplementation(async (_, init) => {
    signal = init?.signal as AbortSignal
    return new Promise<Response>((r) => {
      resolve = r
    })
  })
  fireEvent.click(screen.getByRole('button', { name: '检索区域' }))
  fireEvent.click((await screen.findAllByRole('button', { name: '停止请求' }))[0])
  expect(signal?.aborted).toBe(true)
  await act(async () => {
    resolve(new Response(JSON.stringify(result)))
  })
  await waitFor(() => expect(screen.queryByText('演示咖啡 A')).not.toBeInTheDocument())
})

it('shows actual Agent tool parameters and duration rather than inferring them from map inputs', async () => {
  const actual = { center: [120, 30], radius_meters: 500, category: 'shop', limit: 10 }
  vi.spyOn(globalThis, 'fetch').mockImplementation(async (url) => {
    if (String(url) === '/healthz')
      return new Response(JSON.stringify({ status: 'ok', poi_source: 'demo', agent_enabled: true }))
    return new Response(
      JSON.stringify({
        answer: '已查询',
        tool_results: [{ name: 'search_poi', arguments: actual, elapsed_ms: 7, result }],
      }),
    )
  })
  render(<App />)
  await screen.findByText('合成演示数据')
  fireEvent.change(screen.getByRole('textbox', { name: '空间任务' }), {
    target: { value: '使用另一个中心查询' },
  })
  fireEvent.click(screen.getByRole('button', { name: '发送消息' }))
  expect(await screen.findByText('已查询')).toBeInTheDocument()
  expect(screen.getByRole('spinbutton', { name: '中心经度' })).toHaveValue(120)
  fireEvent.click(screen.getByRole('tab', { name: /执行记录/ }))
  expect(screen.getByText('search_poi')).toBeInTheDocument()
  expect(
    within(screen.getByText('search_poi').closest('details')!).getByText('7 ms'),
  ).toBeInTheDocument()
})
