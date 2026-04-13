import { describe, it, expect } from 'vitest'
import { ApiRequestError } from './client'

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
