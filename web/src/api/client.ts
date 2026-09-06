export type ActorType = 'admin' | 'writer' | 'reader'
export type Actor = { sessionId: string; type: ActorType; id: string; lifeId: string; nickname: string; lifeCreatedAt: string }
export type Notice = { id: string; type: string; refID?: string; refDate?: string; text: string; createdAt: string; read: boolean }
type ApiError = { error: { code: string; message: string } }

/** 后端 Actor 结构体没有 json 标签，返回的是 PascalCase 字段；这里统一归一化。 */
export function normalizeActor(raw: unknown): Actor {
  const value = (raw ?? {}) as Record<string, unknown>
  const pick = (...keys: string[]) => { for (const key of keys) { const item = value[key]; if (typeof item === 'string') return item } return '' }
  const type = pick('type', 'Type') as ActorType
  return { sessionId: pick('session_id', 'sessionId', 'SessionID'), type, id: pick('id', 'ID'), lifeId: pick('life_id', 'lifeId', 'LifeID'), nickname: pick('nickname', 'Nickname'), lifeCreatedAt: pick('life_created_at', 'lifeCreatedAt', 'LifeCreatedAt') }
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const response = await fetch(path, { credentials: 'include', headers: { 'Content-Type': 'application/json', ...init.headers }, ...init })
  if (!response.ok) {
    const body = await response.json().catch(() => null) as ApiError | null
    throw new Error(body?.error?.message ?? `请求失败（${response.status}）`)
  }
  if (response.status === 204) return undefined as T
  return response.json() as Promise<T>
}
async function upload<T>(path: string, file: File): Promise<T> {
  const form = new FormData()
  form.append('file', file)
  const response = await fetch(path, { method: 'POST', credentials: 'include', body: form })
  if (!response.ok) {
    const body = await response.json().catch(() => null) as ApiError | null
    throw new Error(body?.error?.message ?? '上传失败')
  }
  return response.json() as Promise<T>
}
const encode = encodeURIComponent

export const api = {
  keyLogin: async (key: string) => { const result = await request<{ actor: unknown }>('/api/v1/auth/key-login', { method: 'POST', body: JSON.stringify({ key }) }); return { actor: normalizeActor(result.actor) } },
  adminLogin: async (password: string) => { const result = await request<{ actor: unknown }>('/api/v1/admin/auth/login', { method: 'POST', body: JSON.stringify({ password }) }); return { actor: normalizeActor(result.actor) } },
  logout: () => request<void>('/api/v1/auth/logout', { method: 'POST' }),
  me: async () => { const result = await request<{ actor: unknown; capabilities: string[] }>('/api/v1/auth/me'); return { actor: normalizeActor(result.actor), capabilities: result.capabilities ?? [] } },
  writers: () => request<{ items: Writer[] }>('/api/v1/admin/writers'),
  createWriter: (nickname: string) => request<{ writer: Writer; master_key: string }>('/api/v1/admin/writers', { method: 'POST', body: JSON.stringify({ nickname }) }),
  readerKeys: (lifeID: string) => request<{ items: ReaderKey[] }>(`/api/v1/admin/writers/${encode(lifeID)}/reader-keys`),
  createReaderKey: (lifeID: string, payload: { nickname: string; note: string; expires_at?: string }) => request<{ reader_key: ReaderKey; key: string }>(`/api/v1/admin/writers/${encode(lifeID)}/reader-keys`, { method: 'POST', body: JSON.stringify(payload) }),
  revokeReaderKey: (id: string) => request<void>(`/api/v1/admin/reader-keys/${encode(id)}/revoke`, { method: 'POST' }),
  today: () => request<NowData>('/api/v1/now'),
  visibleToday: () => request<NowData>('/api/v1/today'),
  history: (from: string, to: string) => request<HistoryRange>(`/api/v1/history?from=${encode(from)}&to=${encode(to)}`),
  plans: () => request<{ items: Plan[] }>('/api/v1/plans'),
  createPlan: (payload: Pick<Plan, 'name' | 'startDate' | 'endDate' | 'intro'>) => request<Plan>('/api/v1/now/plans', { method: 'POST', body: JSON.stringify({ name: payload.name, start_date: payload.startDate, end_date: payload.endDate, intro: payload.intro }) }),
  updatePlan: (id: string, payload: Pick<Plan, 'name' | 'startDate' | 'endDate' | 'intro'>) => request<Plan>(`/api/v1/now/plans/${encode(id)}`, { method: 'PUT', body: JSON.stringify({ name: payload.name, start_date: payload.startDate, end_date: payload.endDate, intro: payload.intro }) }),
  reorderPlans: (ids: string[]) => request<void>('/api/v1/now/plans/reorder', { method: 'POST', body: JSON.stringify({ ids }) }),
  setPlanProgress: (id: string, date: string, percent: number) => request<Plan>(`/api/v1/now/plans/${encode(id)}/progress`, { method: 'POST', body: JSON.stringify({ date, percent }) }),
  uploadPlanImage: (id: string, kind: 'cover' | 'icon', file: File) => upload<Plan>(`/api/v1/now/plans/${encode(id)}/${kind}`, file),
  uploadPlanFile: (id: string, file: File) => upload<PlanFile>(`/api/v1/now/plans/${encode(id)}/files`, file),
  deletePlanFile: (id: string, fileID: string) => request<void>(`/api/v1/now/plans/${encode(id)}/files/${encode(fileID)}`, { method: 'DELETE' }),
  calendar: (from: string, to: string) => request<{ items: FutureTask[] }>(`/api/v1/calendar?from=${encode(from)}&to=${encode(to)}`),
  notifications: () => request<{ items: Notice[] }>('/api/v1/notifications'),
  markNotificationRead: (id: string) => request<void>(`/api/v1/notifications/${encode(id)}/read`, { method: 'POST' }),
  moodTags: () => request<{ items: MoodTag[] }>('/api/v1/now/mood-tags'),
  addMoodTag: (payload: Pick<MoodTag, 'name' | 'emoji' | 'value'>) => request<MoodTag>('/api/v1/now/mood-tags', { method: 'POST', body: JSON.stringify(payload) }),
  addMood: (tagIDs: string[], note: string, secret: boolean) => request<MoodRecord>('/api/v1/now/moods', { method: 'POST', body: JSON.stringify({ tag_ids: tagIDs, note, secret }) }),
  addBody: (score: number, note: string, secret: boolean) => request<BodyRecord>('/api/v1/now/body', { method: 'POST', body: JSON.stringify({ score, note, secret }) }),
  uploadAttachment: (file: File) => upload<Attachment>('/api/v1/now/diary/attachments', file),
  attachmentUrl: (id: string) => `/api/v1/attachments/${encode(id)}`,
  musicPlaylists: () => request<{ items: Playlist[] }>('/api/v1/now/music/playlists'),
  replaceMusicPlaylist: (page: PlaylistPage, payload: { name: string; mode: PlaylistMode; volume: number; default_enabled?: boolean }) => request<Playlist>(`/api/v1/now/music/playlists/${page}`, { method: 'PUT', body: JSON.stringify(payload) }),
  deleteMusicPlaylist: (page: PlaylistPage) => request<void>(`/api/v1/now/music/playlists/${page}`, { method: 'DELETE' }),
  uploadMusicTrack: (page: PlaylistPage, file: File) => upload<MusicTrack>(`/api/v1/now/music/playlists/${page}/tracks`, file),
  deleteMusicTrack: (id: string) => request<void>(`/api/v1/now/music/tracks/${encode(id)}`, { method: 'DELETE' }),
  saveDraft: (content: string, secret = false) => request<Draft>('/api/v1/now/diary/draft', { method: 'PUT', body: JSON.stringify({ content, secret }) }),
  saveDiary: (content: string, secret = false) => request<Diary>('/api/v1/now/diary', { method: 'PUT', body: JSON.stringify({ content, secret }) }),
  /** date 为空则记到今天；否则记到指定日期（YYYY-MM-DD）。 */
  addTask: (title: string, description: string, priority: string, date = '') => request<Task>('/api/v1/now/tasks', { method: 'POST', body: JSON.stringify({ title, description, priority, date }) }),
  task: (id: string, date: string) => request<Task>(`/api/v1/now/tasks/${encode(id)}?date=${encode(date)}`),
  updateFutureTask: (id: string, date: string, payload: { title: string; description: string; priority: Task['priority'] }) => request<Task>(`/api/v1/now/tasks/${encode(id)}/future-detail`, { method: 'PUT', body: JSON.stringify({ date, title: payload.title, description: payload.description, priority: payload.priority }) }),
  deleteFutureTask: (id: string, date: string) => request<void>(`/api/v1/now/tasks/${encode(id)}/future-detail`, { method: 'DELETE', body: JSON.stringify({ date }) }),
  /** 任务按月分库，传 date 以便服务端定位到对应月份。 */
  setTaskDone: (id: string, done: boolean, date = '') => request<Task>(`/api/v1/now/tasks/${encode(id)}/done`, { method: 'POST', body: JSON.stringify({ done, date }) }),
  readerKeysForWriter: () => request<{ items: ReaderKey[] }>('/api/v1/now/reader-keys'),
  presets: () => request<{ items: Preset[] }>('/api/v1/now/presets'),
  replacePresetRules: (id: string, rules: PresetRule[]) => request<void>(`/api/v1/now/presets/${encode(id)}/rules`, { method: 'PUT', body: JSON.stringify({ rules }) }),
  createPreset: (name: string, rules: PresetRule[]) => request<Preset>('/api/v1/now/presets', { method: 'POST', body: JSON.stringify({ name, rules }) }),
  setDiaryAccess: (preset_id: string, secret: boolean, commentable: boolean) => request<Diary>('/api/v1/now/diary/access', { method: 'PUT', body: JSON.stringify({ preset_id, secret, commentable }) }),
  setTaskAccess: (id: string, preset_id: string, secret: boolean, commentable: boolean) => request<Task>(`/api/v1/now/tasks/${encode(id)}/access`, { method: 'PUT', body: JSON.stringify({ preset_id, secret, commentable }) }),
  comments: (target_type: string, target_id: string) => request<{ items: Comment[] }>(`/api/v1/comments?target_type=${encode(target_type)}&target_id=${encode(target_id)}`),
  milestones: (target_type: string, target_id: string) => request<{ items: Milestone[] }>(`/api/v1/milestones?target_type=${encode(target_type)}&target_id=${encode(target_id)}`),
  addComment: (target_type: string, target_id: string, content: string) => request<Comment>('/api/v1/comments', { method: 'POST', body: JSON.stringify({ target_type, target_id, content }) }),
  addMilestone: (payload: { target_type: string; target_id: string; description: string; detail: string; preset_id: string; secret: boolean }) => request<Milestone>('/api/v1/now/milestones', { method: 'POST', body: JSON.stringify(payload) }),
}

export type Writer = { id: string; nickname: string; life_id: string; status: string; created_at: string }
export type ReaderKey = { id: string; nickname: string; anchor_local_date: string; expires_at?: string | null; revoked_at?: string | null; note: string; created_at: string }
export type MoodTag = { id: string; name: string; emoji: string; value: number; sortOrder?: number }
export type MoodRecord = { id: string; recordedAt: string; recordedDate: string; value: number; note: string; tags: MoodTag[]; secret?: boolean }
export type BodyRecord = { id: string; recordedAt: string; recordedDate: string; score: number; note: string; secret?: boolean }
export type Diary = { id: string; entryDate: string; content: string; presetId?: string; secret: boolean; commentable: boolean }
export type Draft = { entryDate: string; content: string; updatedAt: string }
export type Attachment = { id: string; originalName: string; mimeType: string; byteSize: number }
export type PlaylistPage = 'now' | 'past' | 'future'
export type PlaylistMode = 'list' | 'random' | 'single'
export type MusicTrack = { id: string; title: string; mimeType: string; byteSize: number; sortOrder: number; url: string }
export type Playlist = { id: string; page: PlaylistPage; name: string; mode: PlaylistMode; volume: number; defaultEnabled: boolean; tracks: MusicTrack[]; updatedAt: string }
export type PresetRule = { readerKeyId: string; allowed: boolean }
export type Preset = { id: string; name: string; rules: PresetRule[] }
export type Comment = { id: string; targetType: string; targetId: string; authorKeyId: string; content: string; createdAt: string }
export type Milestone = { id: string; targetType: string; targetId: string; description: string; detail: string; presetId: string; secret: boolean }
export type Task = { id: string; taskDate: string; title: string; description: string; priority: 'low' | 'normal' | 'high'; done: boolean; presetId?: string; secret?: boolean; commentable?: boolean }
export type NowData = { diary: Diary; secretDiary?: Diary; moods: MoodRecord[]; bodies: BodyRecord[]; tasks: Task[] }
export type PlanFile = { id: string; planId: string; originalName: string; mimeType: string; byteSize: number; url: string }
export type Plan = { id: string; name: string; startDate: string; endDate: string; intro: string; progress: number; timeProgress: number; sortOrder?: number; secret?: boolean; presetId?: string; coverUrl?: string; iconUrl?: string; files?: PlanFile[] }
export type FutureTask = { id: string; date: string; title: string; priority: string; done: boolean; presetId?: string; secret?: boolean }
export type HistoryDay = { date: string; diary: Diary; secretDiary?: Diary | null; tasks: Task[]; moodCount: number; bodyCount: number; milestoneCount: number }
export type TrendPoint = { date: string; mood: number | null; body: number | null }
export type HistoryRange = { from: string; to: string; days: HistoryDay[]; points: TrendPoint[] }
