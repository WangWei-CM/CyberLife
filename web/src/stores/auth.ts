import { computed, reactive } from 'vue'
import { api, type Actor } from '../api/client'

export const authState = reactive<{ actor: Actor | null; loading: boolean; capabilities: string[]; justLoggedIn: boolean }>({ actor: null, loading: true, capabilities: [], justLoggedIn: false })

export const isWriter = computed(() => authState.actor?.type === 'writer')
export const isReader = computed(() => authState.actor?.type === 'reader')
export const isAdmin = computed(() => authState.actor?.type === 'admin')
export const roleLabel = computed(() => authState.actor?.type === 'admin' ? '管理员' : authState.actor?.type === 'writer' ? '书写者' : '阅读者')

export async function restoreSession() {
  try {
    const result = await api.me()
    authState.actor = result.actor
    authState.capabilities = result.capabilities
  } catch {
    authState.actor = null
  } finally {
    authState.loading = false
  }
}
export function signIn(actor: Actor) {
  authState.actor = actor
  authState.justLoggedIn = true
}
export async function logout() {
  try { await api.logout() } finally { authState.actor = null; authState.justLoggedIn = false }
}
