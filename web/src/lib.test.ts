import { describe, expect, it, vi } from 'vitest'
import { circleRing, makeSearch, closeRing, postJSON } from './lib'

describe('WGS84 query geometry', () => {
  it('creates a closed geodesic circle of the requested radius', () => {
    const ring = circleRing([0, 0], 1000)
    expect(ring).toHaveLength(97)
    expect(ring[0]).toEqual(ring.at(-1))
    const latitude = (ring[0][1] * Math.PI) / 180
    expect(latitude * 6371008.8).toBeCloseTo(1000, 4)
  })
  it('keeps a circle continuous across the date line', () => {
    const ring = circleRing([179.999, 0], 1000)
    expect(Math.max(...ring.slice(1).map((p, i) => Math.abs(p[0] - ring[i][0])))).toBeLessThan(0.01)
  })
  it('closes a polygon without mutating the drawing', () => {
    const points: [number, number][] = [
      [0, 0],
      [1, 0],
      [1, 1],
    ]
    expect(closeRing(points)).toEqual([...points, points[0]])
    expect(points).toHaveLength(3)
    expect(() =>
      closeRing([
        [0, 0],
        [1, 1],
      ]),
    ).toThrow()
  })
  it('validates radius and coordinates, and uses polygon as an intersection', () => {
    const points: [number, number][] = [
      [0, 0],
      [1, 0],
      [1, 1],
    ]
    const result = makeSearch([121, 31], 1000, 'cafe', points)
    expect(result.polygon?.coordinates[0]).toHaveLength(4)
    expect(result.limit).toBe(100)
    expect(() => makeSearch([181, 31], 1000, '', [])).toThrow()
    expect(() => makeSearch([121, 31], NaN, '', [])).toThrow()
    expect(() => makeSearch([121, 31], 50001, '', [])).toThrow()
  })
})

describe('API client', () => {
  it('propagates abort signals and safe domain errors', async () => {
    const fetcher = vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ error: { code: 'invalid_argument', message: '围栏无效' } }), {
        status: 400,
      }),
    )
    const controller = new AbortController()
    await expect(postJSON('/v1/poi/search', {}, controller.signal)).rejects.toThrow('围栏无效')
    expect(fetcher.mock.calls[0][1]?.signal).toBe(controller.signal)
    fetcher.mockRestore()
  })
  it('handles non-JSON gateway errors', async () => {
    const fetcher = vi
      .spyOn(globalThis, 'fetch')
      .mockResolvedValue(new Response('<html>bad gateway</html>', { status: 502 }))
    await expect(postJSON('/v1/agent/run', {})).rejects.toThrow('502')
    fetcher.mockRestore()
  })
})
