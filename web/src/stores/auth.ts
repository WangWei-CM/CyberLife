import { reactive } from 'vue'
import { api, type Actor } from '../api/client'

export const authState = reactive<{ actor: Actor | null; loading: boolean }>({ actor: null, loading: true })
export async function restoreSession() { try { authState.actor = (await api.me()).actor } catch { authState.actor = null } finally { authState.loading = false } }
export async function logout() { await api.logout(); authState.actor = null }
