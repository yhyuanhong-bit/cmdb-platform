import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { ApiRequestError, apiClient } from './client'

describe('ApiRequestError', () => {
  it('creates error with correct properties', () => {
    const err = new ApiRequestError('NOT_FOUND', 'Resource not found', 404)
    expect(err.code).toBe('NOT_FOUND')
    expect(err.message).toBe('Resource not found')
    expect(err.status).toBe(404)
    expect(err).toBeInstanceOf(Error)
  })

  it('has name set to ApiRequestError', () => {
    const err = new ApiRequestError('SERVER_ERROR', 'Internal error', 500)
    expect(err.name).toBe('ApiRequestError')
  })

  it('inherits from Error prototype', () => {
    const err = new ApiRequestError('UNAUTHORIZED', 'Not authorized', 401)
    expect(err instanceof Error).toBe(true)
    expect(err.stack).toBeDefined()
  })

  it('handles empty message', () => {
    const err = new ApiRequestError('UNKNOWN', '', 0)
    expect(err.message).toBe('')
    expect(err.code).toBe('UNKNOWN')
    expect(err.status).toBe(0)
  })
})

describe('apiClient 429 retry', () => {
  const originalFetch = globalThis.fetch

  beforeEach(() => {
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
    globalThis.fetch = originalFetch
  })

  it('retries after 429 then succeeds', async () => {
    const tooMany = new Response(JSON.stringify({ error: { code: 'RATE_LIMITED', message: 'slow down' } }), {
      status: 429,
      headers: { 'Retry-After': '0', 'Content-Type': 'application/json' },
    })
    const ok = new Response(JSON.stringify({ data: { hello: 'world' } }), {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
    })
    const fetchMock = vi.fn().mockResolvedValueOnce(tooMany).mockResolvedValueOnce(ok)
    globalThis.fetch = fetchMock as unknown as typeof fetch

    const promise = apiClient.get<{ data: { hello: string } }>('/retry-ok')
    await vi.runAllTimersAsync()
    const result = await promise

    expect(fetchMock).toHaveBeenCalledTimes(2)
    expect(result.data.hello).toBe('world')
  })

  it('gives up and throws after exhausting retries', async () => {
    const tooMany = () =>
      new Response(JSON.stringify({ error: { code: 'RATE_LIMITED', message: 'slow down' } }), {
        status: 429,
        headers: { 'Retry-After': '0', 'Content-Type': 'application/json' },
      })
    const fetchMock = vi.fn(async () => tooMany())
    globalThis.fetch = fetchMock as unknown as typeof fetch

    const promise = apiClient.get('/retry-exhaust').catch((e) => e)
    await vi.runAllTimersAsync()
    const err = await promise

    expect(fetchMock).toHaveBeenCalledTimes(4) // initial + 3 retries
    expect(err).toBeInstanceOf(ApiRequestError)
    expect((err as ApiRequestError).status).toBe(429)
  })
})
