import { computed, reactive } from 'vue'
import { api, type Actor } from '../api/client'

export const authState = reactive<{ actor: Actor | null; loading: boolean; capabilities: string[]; justLoggedIn: boolean }>({ actor: null, loading: true, capabilities: [], justLoggedIn: false })

export const isWriter = computed(() => authState.actor?.type === 'writer')
export const isReader = computed(() => authState.actor?.type === 'reader')
export const isAdmin = computed(() => authState.actor?.type === 'admin')
export const roleLabel = computed(() => authState.actor?.type === 'admin' ? '管理员' : authState.actor?.type === 'writer' ? '书写者' : '阅读者')
/** 顶栏显示：有昵称时显示昵称，否则显示身份。 */
export const displayName = computed(() => authState.actor?.nickname || roleLabel.value)
/** 注册日（人生创建时间，按北京日期），接口未返回时为空。 */
export const lifeStartISO = computed(() => {
  const raw = authState.actor?.lifeCreatedAt
  if (!raw) return ''
  const date = new Date(raw)
  if (Number.isNaN(date.getTime())) return ''
  const parts = new Intl.DateTimeFormat('en-CA', { timeZone: 'Asia/Shanghai', year: 'numeric', month: '2-digit', day: '2-digit' }).format(date)
  return parts
})

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
