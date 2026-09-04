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
  visibleToday: () => request<NowData>('/api/v1/today'),
  history: (from: string, to: string) => request<HistoryRange>(`/api/v1/history?from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}`),
  moodTags: () => request<{ items: MoodTag[] }>('/api/v1/now/mood-tags'),
  addMoodTag: (payload: Pick<MoodTag, 'name' | 'emoji' | 'value'>) => request<MoodTag>('/api/v1/now/mood-tags', { method: 'POST', body: JSON.stringify(payload) }),
  addMood: (tagIDs: string[], note: string, secret: boolean) => request<MoodRecord>('/api/v1/now/moods', { method: 'POST', body: JSON.stringify({ tag_ids: tagIDs, note, secret }) }),
  addBody: (score: number, note: string, secret: boolean) => request<BodyRecord>('/api/v1/now/body', { method: 'POST', body: JSON.stringify({ score, note, secret }) }),
  uploadAttachment: async (file: File) => { const form = new FormData(); form.append('file', file); const response = await fetch('/api/v1/now/diary/attachments', { method: 'POST', credentials: 'include', body: form }); if (!response.ok) { const body = await response.json().catch(() => null) as ApiError | null; throw new Error(body?.error?.message ?? '上传失败') }; return response.json() as Promise<Attachment> },
  saveDraft: (content: string) => request<Draft>('/api/v1/now/diary/draft', { method: 'PUT', body: JSON.stringify({ content }) }),
  saveDiary: (content: string) => request<Diary>('/api/v1/now/diary', { method: 'PUT', body: JSON.stringify({ content }) }),
  addTask: (title: string, description: string, priority: string) => request<Task>('/api/v1/now/tasks', { method: 'POST', body: JSON.stringify({ title, description, priority }) }),
  setTaskDone: (id: string, done: boolean) => request<Task>(`/api/v1/now/tasks/${id}/done`, { method: 'POST', body: JSON.stringify({ done }) }),
  readerKeysForWriter: () => request<{ items: ReaderKey[] }>('/api/v1/now/reader-keys'),
  presets: () => request<{ items: Preset[] }>('/api/v1/now/presets'),
  replacePresetRules: (id: string, rules: PresetRule[]) => request<void>(`/api/v1/now/presets/${id}/rules`, { method: 'PUT', body: JSON.stringify({ rules }) }),
  createPreset: (name: string, rules: PresetRule[]) => request<Preset>('/api/v1/now/presets', { method: 'POST', body: JSON.stringify({ name, rules }) }),
  setDiaryAccess: (preset_id: string, secret: boolean, commentable: boolean) => request<Diary>('/api/v1/now/diary/access', { method: 'PUT', body: JSON.stringify({ preset_id, secret, commentable }) }),
  setTaskAccess: (id: string, preset_id: string, secret: boolean, commentable: boolean) => request<Task>(`/api/v1/now/tasks/${id}/access`, { method: 'PUT', body: JSON.stringify({ preset_id, secret, commentable }) }),
  comments: (target_type: string, target_id: string) => request<{ items: Comment[] }>(`/api/v1/comments?target_type=${encodeURIComponent(target_type)}&target_id=${encodeURIComponent(target_id)}`),
  milestones: (target_type: string, target_id: string) => request<{ items: Milestone[] }>(`/api/v1/milestones?target_type=${encodeURIComponent(target_type)}&target_id=${encodeURIComponent(target_id)}`),
  addComment: (target_type: string, target_id: string, content: string) => request<Comment>('/api/v1/comments', { method: 'POST', body: JSON.stringify({ target_type, target_id, content }) }),
  addMilestone: (payload: { target_type: string; target_id: string; description: string; detail: string; preset_id: string; secret: boolean }) => request<Milestone>('/api/v1/now/milestones', { method: 'POST', body: JSON.stringify(payload) }),
}
export type Writer = { id: string; nickname: string; life_id: string; status: string; created_at: string }
export type ReaderKey = { id: string; nickname: string; anchor_local_date: string; expires_at?: string; revoked_at?: string; note: string; created_at: string }
export type MoodTag = { id: string; name: string; emoji: string; value: number; sortOrder?: number }
export type MoodRecord = { id: string; recordedAt: string; recordedDate: string; value: number; note: string; tags: MoodTag[] }
export type BodyRecord = { id: string; recordedAt: string; recordedDate: string; score: number; note: string }
export type Diary = { id: string; entryDate: string; content: string; presetId?: string; secret: boolean; commentable: boolean }
export type Draft = { entryDate: string; content: string; updatedAt: string }
export type Attachment = { id: string; originalName: string; mimeType: string; byteSize: number }
export type PresetRule = { readerKeyId: string; allowed: boolean }
export type Preset = { id: string; name: string; rules: PresetRule[] }
export type Comment = { id: string; targetType: string; targetId: string; authorKeyId: string; content: string; createdAt: string }
export type Milestone = { id: string; targetType: string; targetId: string; description: string; detail: string; presetId: string; secret: boolean }
export type Task = { id: string; taskDate: string; title: string; description: string; priority: 'low' | 'normal' | 'high'; done: boolean }
export type NowData = { diary: Diary; moods: MoodRecord[]; bodies: BodyRecord[]; tasks: Task[] }
export type HistoryDay = { date: string; diary: Diary; tasks: Task[]; moodCount: number; bodyCount: number; milestoneCount: number }
export type TrendPoint = { date: string; mood: number | null; body: number | null }
export type HistoryRange = { from: string; to: string; days: HistoryDay[]; points: TrendPoint[] }
