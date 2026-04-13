import { useAuthStore } from '../stores/authStore'

export function usePermission(module: string, action: string): boolean {
  const user = useAuthStore((s) => s.user)
  if (!user?.permissions) return false

  // Super-admin wildcard: {"*": ["*"]}
  const wildcard = user.permissions['*']
  if (wildcard && (wildcard.includes('*') || wildcard.includes(action))) return true

  const actions = user.permissions[module]
  if (!actions) return false

  // Direct match, wildcard action, write implies read
  if (actions.includes(action) || actions.includes('*')) return true
  if (action === 'read' && actions.includes('write')) return true

  return false
}
