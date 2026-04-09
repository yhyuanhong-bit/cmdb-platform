import { QueryClient, QueryClientProvider, MutationCache } from '@tanstack/react-query'
import { ReactNode } from 'react'
import { toast } from 'sonner'

const queryClient = new QueryClient({
  mutationCache: new MutationCache({
    onError: (error, _variables, _context, mutation) => {
      // Skip if the mutation already defines its own onError handler
      if (mutation.options.onError) return
      const message = error instanceof Error ? error.message : 'Operation failed'
      toast.error(message)
    },
  }),
  defaultOptions: {
    queries: {
      staleTime: 30 * 1000,
      retry: 2,
      refetchOnWindowFocus: false,
    },
  },
})

export default function QueryProvider({ children }: { children: ReactNode }) {
  return (
    <QueryClientProvider client={queryClient}>
      {children}
    </QueryClientProvider>
  )
}
