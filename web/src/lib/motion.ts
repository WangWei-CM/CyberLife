import { onBeforeUnmount, ref, watch, type Directive, type Ref } from 'vue'

/** 动效令牌（与 tokens.css 保持一致，供 JS 时序使用） */
export const DUR = { fast: 150, mid: 300, slow: 600, page: 800, stagger: 40, staggerMax: 400 } as const

export function reducedMotion(): boolean {
  return typeof matchMedia === 'function' && matchMedia('(prefers-reduced-motion: reduce)').matches
}

export const wait = (ms: number) => new Promise<void>(resolve => setTimeout(resolve, reducedMotion() ? 0 : ms))

/**
 * v-stagger：容器内的直接子元素按 DOM 顺序淡入（opacity 0→1，translateY 12→0），
 * 间隔 40ms，总时长 ≤ 600ms。传入的值变化时重新播放（用于页面/数据切换）。
 */
function playStagger(container: HTMLElement, selector?: string) {
  const items = Array.from(selector ? container.querySelectorAll<HTMLElement>(selector) : (container.children as HTMLCollectionOf<HTMLElement>))
  if (!items.length) return
  if (reducedMotion()) { items.forEach(item => item.classList.remove('card-enter', 'visible')); return }
  items.forEach((item, index) => {
    item.classList.remove('visible')
    item.classList.add('card-enter')
    item.style.transitionDelay = `${Math.min(index * DUR.stagger, DUR.staggerMax)}ms`
  })
  requestAnimationFrame(() => requestAnimationFrame(() => {
    items.forEach(item => item.classList.add('visible'))
    setTimeout(() => items.forEach(item => { item.classList.remove('card-enter', 'visible'); item.style.transitionDelay = '' }), DUR.staggerMax + DUR.mid + 80)
  }))
}
export const vStagger: Directive<HTMLElement, string | { selector?: string; key?: unknown } | undefined> = {
  mounted(el, binding) { playStagger(el, typeof binding.value === 'object' ? binding.value?.selector : undefined) },
  updated(el, binding) {
    const previous = typeof binding.oldValue === 'object' ? binding.oldValue?.key : binding.oldValue
    const next = typeof binding.value === 'object' ? binding.value?.key : binding.value
    if (previous !== next) playStagger(el, typeof binding.value === 'object' ? binding.value?.selector : undefined)
  },
}

/**
 * v-glow：跟随光斑。指针移动时把坐标写入 --mx/--my，CSS 里的径向渐变随之点亮。
 * 直接绑在元素上，登录页等不在 app-shell 内的元素同样生效。
 */
const glowHandlers = new WeakMap<HTMLElement, (event: PointerEvent) => void>()
export const vGlow: Directive<HTMLElement> = {
  mounted(el) {
    el.classList.add('glow-spot')
    const handler = (event: PointerEvent) => {
      const bounds = el.getBoundingClientRect()
      el.style.setProperty('--mx', `${event.clientX - bounds.left}px`)
      el.style.setProperty('--my', `${event.clientY - bounds.top}px`)
    }
    glowHandlers.set(el, handler)
    el.addEventListener('pointermove', handler, { passive: true })
  },
  unmounted(el) {
    const handler = glowHandlers.get(el)
    if (handler) el.removeEventListener('pointermove', handler)
    glowHandlers.delete(el)
  },
}

/** 数值过渡：目标值变化时用 rAF 插值逼近，返回用于渲染的 ref。 */
export function useCountUp(source: () => number, duration: number = DUR.slow): Ref<number> {
  const display = ref(source())
  let frame = 0
  let from = display.value
  let to = display.value
  let started = 0
  const step = (now: number) => {
    const progress = Math.min(1, (now - started) / duration)
    const eased = 1 - Math.pow(1 - progress, 3)
    display.value = from + (to - from) * eased
    if (progress < 1) frame = requestAnimationFrame(step)
    else frame = 0
  }
  watch(source, next => {
    if (!Number.isFinite(next)) return
    if (reducedMotion() || duration <= 0) { display.value = next; return }
    from = display.value; to = next; started = performance.now()
    if (!frame) frame = requestAnimationFrame(step)
  })
  onBeforeUnmount(() => { if (frame) cancelAnimationFrame(frame) })
  return display
}

/**
 * rAF 驱动的逼近器：display += (target - display) * factor，直到差值 < epsilon。
 * 过去页时间轴与未来页日历的连续缩放共用。
 */
export function createApproach(options: { factor?: number; epsilon?: number; onFrame: (value: number) => void; onSettle?: () => void }) {
  const factor = options.factor ?? .12
  const epsilon = options.epsilon ?? .01
  let display = 0
  let target = 0
  let frame = 0
  const tick = () => {
    const distance = target - display
    if (Math.abs(distance) > epsilon && !reducedMotion()) {
      display += distance * factor
      options.onFrame(display)
      frame = requestAnimationFrame(tick)
    } else {
      display = target
      options.onFrame(display)
      frame = 0
      options.onSettle?.()
    }
  }
  return {
    get display() { return display },
    get target() { return target },
    set(value: number, immediate = false) {
      target = value
      if (immediate) { display = value; if (frame) { cancelAnimationFrame(frame); frame = 0 } options.onFrame(display); options.onSettle?.(); return }
      if (!frame) frame = requestAnimationFrame(tick)
    },
    stop() { if (frame) cancelAnimationFrame(frame); frame = 0 },
  }
}

/** 主题/绝密切换：给根节点临时挂 .theme-shift，让所有颜色属性以 --dur-slow 迁移。 */
let shiftTimer: number | undefined
export function transitionTheme(root: HTMLElement | null, apply: () => void) {
  if (!root || reducedMotion()) { apply(); return }
  root.classList.add('theme-shift')
  if (shiftTimer) clearTimeout(shiftTimer)
  requestAnimationFrame(apply)
  shiftTimer = window.setTimeout(() => root.classList.remove('theme-shift'), DUR.slow + 120)
}

/** 分段控件滑块：根据 .active 按钮位置写入 --thumb-x/--thumb-w。 */
export function positionSegmentThumb(container: HTMLElement | null) {
  if (!container) return
  const active = container.querySelector<HTMLElement>('button.active')
  if (!active) { container.style.setProperty('--thumb-w', '0px'); return }
  container.style.setProperty('--thumb-x', `${active.offsetLeft}px`)
  container.style.setProperty('--thumb-w', `${active.offsetWidth}px`)
}
