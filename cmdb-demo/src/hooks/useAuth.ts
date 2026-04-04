import { useAuthStore } from '../stores/authStore'

export function useAuth() {
  const { user, isAuthenticated, login, logout } = useAuthStore()
  return { user, isAuthenticated, login, logout }
}
