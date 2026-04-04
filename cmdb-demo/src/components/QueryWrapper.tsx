import { type ReactNode } from 'react'
import { type UseQueryResult } from '@tanstack/react-query'
import Icon from './Icon'

interface QueryWrapperProps<T> {
  query: UseQueryResult<T>
  children: (data: T) => ReactNode
  emptyMessage?: string
}

export default function QueryWrapper<T>({ query, children, emptyMessage }: QueryWrapperProps<T>) {
  if (query.isLoading) {
    return (
      <div className="flex items-center justify-center py-20">
        <div className="animate-spin rounded-full h-8 w-8 border-2 border-sky-400 border-t-transparent" />
      </div>
    )
  }

  if (query.error) {
    return (
      <div className="flex flex-col items-center justify-center py-20 gap-3">
        <Icon name="error" className="text-red-400 text-4xl" />
        <p className="text-red-300 text-sm">Failed to load data</p>
        <button
          onClick={() => query.refetch()}
          className="px-4 py-1.5 rounded bg-white/10 text-sm hover:bg-white/20 transition"
        >
          Retry
        </button>
      </div>
    )
  }

  if (!query.data) {
    return (
      <div className="flex flex-col items-center justify-center py-20 gap-2">
        <Icon name="inbox" className="text-white/30 text-4xl" />
        <p className="text-white/40 text-sm">{emptyMessage || 'No data'}</p>
      </div>
    )
  }

  return <>{children(query.data)}</>
}
