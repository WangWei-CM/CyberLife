export type Actor = { session_id: string; type: 'admin' | 'writer' | 'reader'; id: string; life_id: string }
type ApiError = { error: { code: string; message: string } }

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const response = await fetch(path, { credentials: 'include', headers: { 'Content-Type': 'application/json', ...init.headers }, ...init })
  if (!response.ok) {
    const body = await response.json().catch(() => null) as ApiError | null
    throw new Error(body?.error?.message ?? `请求失败（${response.status}）`)
  }
  if (response.status === 204) return undefined as T
  return response.json() as Promise<T>
}
export const api = {
  keyLogin: (key: string) => request<{ actor: Actor }>('/api/v1/auth/key-login', { method: 'POST', body: JSON.stringify({ key }) }),
  adminLogin: (password: string) => request<{ actor: Actor }>('/api/v1/admin/auth/login', { method: 'POST', body: JSON.stringify({ password }) }),
  logout: () => request<void>('/api/v1/auth/logout', { method: 'POST' }),
  me: () => request<{ actor: Actor; capabilities: string[] }>('/api/v1/auth/me'),
  writers: () => request<{ items: Writer[] }>('/api/v1/admin/writers'),
  createWriter: (nickname: string) => request<{ writer: Writer; master_key: string }>('/api/v1/admin/writers', { method: 'POST', body: JSON.stringify({ nickname }) }),
  readerKeys: (lifeID: string) => request<{ items: ReaderKey[] }>(`/api/v1/admin/writers/${lifeID}/reader-keys`),
  createReaderKey: (lifeID: string, payload: { nickname: string; note: string; expires_at?: string }) => request<{ reader_key: ReaderKey; key: string }>(`/api/v1/admin/writers/${lifeID}/reader-keys`, { method: 'POST', body: JSON.stringify(payload) }),
  revokeReaderKey: (id: string) => request<void>(`/api/v1/admin/reader-keys/${id}/revoke`, { method: 'POST' }),
  today: () => request<NowData>('/api/v1/now'),
  moodTags: () => request<{ items: MoodTag[] }>('/api/v1/now/mood-tags'),
  addMoodTag: (payload: Pick<MoodTag, 'name' | 'emoji' | 'value'>) => request<MoodTag>('/api/v1/now/mood-tags', { method: 'POST', body: JSON.stringify(payload) }),
  addMood: (tagIDs: string[], note: string) => request<MoodRecord>('/api/v1/now/moods', { method: 'POST', body: JSON.stringify({ tag_ids: tagIDs, note }) }),
  addBody: (score: number, note: string) => request<BodyRecord>('/api/v1/now/body', { method: 'POST', body: JSON.stringify({ score, note }) }),
  saveDiary: (content: string) => request<Diary>('/api/v1/now/diary', { method: 'PUT', body: JSON.stringify({ content }) }),
  addTask: (title: string, description: string, priority: string) => request<Task>('/api/v1/now/tasks', { method: 'POST', body: JSON.stringify({ title, description, priority }) }),
  setTaskDone: (id: string, done: boolean) => request<Task>(`/api/v1/now/tasks/${id}/done`, { method: 'POST', body: JSON.stringify({ done }) }),
}
export type Writer = { id: string; nickname: string; life_id: string; status: string; created_at: string }
export type ReaderKey = { id: string; nickname: string; anchor_local_date: string; expires_at?: string; revoked_at?: string; note: string; created_at: string }
export type MoodTag = { id: string; name: string; emoji: string; value: number; sortOrder?: number }
export type MoodRecord = { id: string; recordedAt: string; recordedDate: string; value: number; note: string; tags: MoodTag[] }
export type BodyRecord = { id: string; recordedAt: string; recordedDate: string; score: number; note: string }
export type Diary = { id: string; entryDate: string; content: string }
export type Task = { id: string; taskDate: string; title: string; description: string; priority: 'low' | 'normal' | 'high'; done: boolean }
export type NowData = { diary: Diary; moods: MoodRecord[]; bodies: BodyRecord[]; tasks: Task[] }
