import { useAuthStore } from '../stores/authStore'

export function usePermission(module: string, action: string): boolean {
  const user = useAuthStore((s) => s.user)
  if (!user?.permissions) return false
  const actions = user.permissions[module]
  if (!actions) return false
  return actions.includes(action) || actions.includes('admin')
}
