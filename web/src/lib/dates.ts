/**
 * 日期工具。后端一律按北京时间（Asia/Shanghai）判定"今天"，前端的"今天"与时钟也以北京时间为准，
 * 这样无论浏览器所在时区如何，日记 / 待办 / 时间轴对应的日期都与服务端一致。
 */
const DAY_MS = 86_400_000
const BEIJING = 'Asia/Shanghai'
const beijingParts = new Intl.DateTimeFormat('en-US', { timeZone: BEIJING, hourCycle: 'h23', year: 'numeric', month: 'numeric', day: 'numeric', hour: 'numeric', minute: 'numeric', second: 'numeric' })
/** 返回一个"本地字段等于北京时间字段"的 Date，供展示与日期运算使用。 */
export function beijingNow(base = new Date()): Date {
  const parts: Record<string, number> = {}
  for (const part of beijingParts.formatToParts(base)) if (part.type !== 'literal') parts[part.type] = Number(part.value)
  return new Date(parts.year, parts.month - 1, parts.day, parts.hour % 24, parts.minute, parts.second)
}

export function iso(value: Date): string {
  const y = value.getFullYear()
  const m = String(value.getMonth() + 1).padStart(2, '0')
  const d = String(value.getDate()).padStart(2, '0')
  return `${y}-${m}-${d}`
}
export function parseISO(value: string): Date {
  const [y, m, d] = value.split('-').map(Number)
  return new Date(y, (m || 1) - 1, d || 1)
}
export function todayISO(): string { return iso(beijingNow()) }
export function startOfDay(value: Date): Date { return new Date(value.getFullYear(), value.getMonth(), value.getDate()) }
export function addDays(value: Date, days: number): Date { const result = new Date(value); result.setDate(result.getDate() + days); return result }
export function addDaysISO(value: string, days: number): string { return iso(addDays(parseISO(value), days)) }
/** 以 1970-01-01 为 0 的本地日序号（整数），用于把日期映射到像素。 */
export function dayIndex(value: Date | string): number {
  const date = typeof value === 'string' ? parseISO(value) : startOfDay(value)
  return Math.round((Date.UTC(date.getFullYear(), date.getMonth(), date.getDate())) / DAY_MS)
}
export function dateFromIndex(index: number): Date {
  const utc = new Date(Math.floor(index) * DAY_MS)
  return new Date(utc.getUTCFullYear(), utc.getUTCMonth(), utc.getUTCDate())
}
export function diffDays(from: string, to: string): number { return dayIndex(to) - dayIndex(from) }
export function daysInMonth(year: number, month: number): number { return new Date(year, month + 1, 0).getDate() }
export function clampDate(value: string, min: string, max: string): string { return value < min ? min : value > max ? max : value }

const weekdayFormat = new Intl.DateTimeFormat('zh-CN', { weekday: 'long' })
const lunarFormat = new Intl.DateTimeFormat('zh-CN-u-ca-chinese', { month: 'long', day: 'numeric' })
const monthDayFormat = new Intl.DateTimeFormat('zh-CN', { month: 'long', day: 'numeric' })
const fullFormat = new Intl.DateTimeFormat('zh-CN', { year: 'numeric', month: 'long', day: 'numeric' })
const timeFormat = new Intl.DateTimeFormat('zh-CN', { hour: '2-digit', minute: '2-digit', hour12: false })

export function weekdayLabel(value: Date): string { return weekdayFormat.format(value) }
export function lunarLabel(value: Date): string { try { return lunarFormat.format(value) } catch { return '' } }
export function monthDayLabel(value: Date | string): string { return monthDayFormat.format(typeof value === 'string' ? parseISO(value) : value) }
export function fullDateLabel(value: Date | string): string { return fullFormat.format(typeof value === 'string' ? parseISO(value) : value) }
/** 服务端时间戳（UTC）→ 北京时间 hh:mm；已经是"北京字段 Date"的值直接格式化。 */
export function timeLabel(value: Date | string): string { return timeFormat.format(typeof value === 'string' ? beijingNow(new Date(value)) : value) }
export function relativeDayLabel(value: string): string {
  const delta = diffDays(todayISO(), value)
  if (delta === 0) return '今天'
  if (delta === -1) return '昨天'
  if (delta === 1) return '明天'
  return delta < 0 ? `${-delta} 天前` : `${delta} 天后`
}
export function greetingForHour(hour = beijingNow().getHours()): string {
  if (hour < 5) return '深夜好'
  if (hour < 9) return '早上好'
  if (hour < 12) return '上午好'
  if (hour < 14) return '中午好'
  if (hour < 18) return '下午好'
  if (hour < 23) return '晚上好'
  return '深夜好'
}
export function isDaytime(value = new Date()): boolean {
  // 日出日落档：以 6:30 – 18:30 作为白天区间。
  const minutes = value.getHours() * 60 + value.getMinutes()
  return minutes >= 6 * 60 + 30 && minutes < 18 * 60 + 30
}
