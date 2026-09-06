<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { createLoginTransition, type LoginTransitionStage } from '../lib/login-transition'
import { reducedMotion, wait } from '../lib/motion'

const props = defineProps<{ credential: string; busy: boolean; error: string; greeting: string; quote: string }>()
const emit = defineEmits<{
  'update:credential': [value: string]
  submit: []
  'transition-complete': []
}>()

const canvas = ref<HTMLCanvasElement>()
const sceneFailed = ref(false)
const stage = ref<LoginTransitionStage>('orbital-login')
const started = ref(false)
const scrambling = ref(false)
const greetingGlyphs = ref<ScrambleGlyph[]>([])
const quoteGlyphs = ref<ScrambleGlyph[]>([])
let scene: ReturnType<typeof createLoginTransition> | undefined
let scrambleFrame = 0

const SCRAMBLE_DURATION_MS = 470
const SCRAMBLE_SETTLE_START_MS = 180
const SCRAMBLE_SETTLE_END_MS = 440
const SCRAMBLE_STEP_MS = 34
const SCRAMBLE_POOL = Array.from('ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789&$#@%?+*<>/')

type GlyphState = 'waiting' | 'cycling' | 'settled'

type ScrambleGlyph = {
  id: string
  final: string
  value: string
  kind: 'full' | 'narrow' | 'space'
  state: GlyphState
  startAt: number
  settleAt: number
}

function glyphKind(character: string): ScrambleGlyph['kind'] {
  if (/\s/u.test(character)) return 'space'
  return /^[\u0000-\u024f]$/u.test(character) ? 'narrow' : 'full'
}

function createGlyphs(text: string, prefix: string): ScrambleGlyph[] {
  return Array.from(text).map((character, index) => ({
    id: `${prefix}-${index}`,
    final: character,
    value: /\s/u.test(character) ? character : '\u00a0',
    kind: glyphKind(character),
    state: /\s/u.test(character) ? 'settled' : 'waiting',
    startAt: 0,
    settleAt: 0,
  }))
}

function settleGlyphs(glyphs: ScrambleGlyph[]) {
  return glyphs.map(glyph => ({ ...glyph, value: glyph.final, state: 'settled' as const }))
}

function renderGlyphs(glyphs: ScrambleGlyph[], elapsed: number) {
  return glyphs.map(glyph => {
    if (glyph.kind === 'space' || elapsed >= glyph.settleAt) {
      return { ...glyph, value: glyph.final, state: 'settled' as const }
    }
    if (elapsed < glyph.startAt) {
      return { ...glyph, value: '\u00a0', state: 'waiting' as const }
    }
    return {
      ...glyph,
      value: SCRAMBLE_POOL[Math.floor(Math.random() * SCRAMBLE_POOL.length)],
      state: 'cycling' as const,
    }
  })
}

function startTextScramble() {
  if (scrambleFrame) cancelAnimationFrame(scrambleFrame)

  const greetingPlan = createGlyphs(props.greeting, 'greeting')
  const quotePlan = createGlyphs(props.quote, 'quote')
  const activeGlyphs = [...greetingPlan, ...quotePlan].filter(glyph => glyph.kind !== 'space')

  for (let index = activeGlyphs.length - 1; index > 0; index -= 1) {
    const swapIndex = Math.floor(Math.random() * (index + 1))
    ;[activeGlyphs[index], activeGlyphs[swapIndex]] = [activeGlyphs[swapIndex], activeGlyphs[index]]
  }

  activeGlyphs.forEach((glyph, index) => {
    const progress = activeGlyphs.length <= 1 ? .5 : index / (activeGlyphs.length - 1)
    glyph.startAt = Math.random() * 82
    glyph.settleAt = SCRAMBLE_SETTLE_START_MS
      + progress * (SCRAMBLE_SETTLE_END_MS - SCRAMBLE_SETTLE_START_MS)
  })

  if (reducedMotion()) {
    greetingGlyphs.value = settleGlyphs(greetingPlan)
    quoteGlyphs.value = settleGlyphs(quotePlan)
    scrambling.value = false
    scrambleFrame = 0
    return
  }

  scrambling.value = true
  greetingGlyphs.value = renderGlyphs(greetingPlan, 0)
  quoteGlyphs.value = renderGlyphs(quotePlan, 0)
  const startedAt = performance.now()
  let lastStepAt = -SCRAMBLE_STEP_MS

  const tick = (now: number) => {
    const elapsed = now - startedAt
    if (elapsed >= SCRAMBLE_DURATION_MS) {
      greetingGlyphs.value = settleGlyphs(greetingPlan)
      quoteGlyphs.value = settleGlyphs(quotePlan)
      scrambling.value = false
      scrambleFrame = 0
      return
    }
    if (elapsed - lastStepAt >= SCRAMBLE_STEP_MS) {
      greetingGlyphs.value = renderGlyphs(greetingPlan, elapsed)
      quoteGlyphs.value = renderGlyphs(quotePlan, elapsed)
      lastStepAt = elapsed
    }
    scrambleFrame = requestAnimationFrame(tick)
  }

  scrambleFrame = requestAnimationFrame(tick)
}

const stageLabel = computed(() => {
  if (stage.value === 'node-reveal') return 'CYBERLIFE NODE'
  if (props.busy || stage.value === 'authenticating') return 'VERIFYING ACCESS'
  return ''
})

function onInput(event: Event) {
  emit('update:credential', (event.target as HTMLInputElement).value)
}

async function beginTransition() {
  if (started.value) return
  started.value = true
  if (sceneFailed.value) {
    emit('transition-complete')
    return
  }
  scene?.setStage('authenticating')
  await wait(180)
  scene?.setStage('cloud-gather')
  if (reducedMotion()) {
    await wait(50)
    scene?.setStage('node-reveal')
  }
}

function onSubmit() {
  if (props.busy || started.value || !props.credential.trim()) return
  emit('submit')
}

onMounted(async () => {
  await nextTick()
  if (!canvas.value) { sceneFailed.value = true; return }
  try {
    scene = createLoginTransition(canvas.value, {
      onStage: value => {
        stage.value = value
        if (value === 'node-reveal') startTextScramble()
      },
      onComplete: fallback => {
        if (fallback) sceneFailed.value = true
        emit('transition-complete')
      },
    })
  } catch {
    sceneFailed.value = true
  }
})

onBeforeUnmount(() => {
  if (scrambleFrame) cancelAnimationFrame(scrambleFrame)
  scene?.destroy()
  scene = undefined
})

defineExpose({
  beginTransition,
  setStage: (next: LoginTransitionStage) => scene?.setStage(next),
})
</script>

<template>
  <main class="login-transition" :class="{ 'scene-failed': sceneFailed, 'transition-started': started, 'cloud-gather': stage === 'cloud-gather', 'node-reveal': stage === 'node-reveal', 'node-fall': stage === 'node-fall', 'page-enter': stage === 'page-enter' }">
    <canvas ref="canvas" class="login-canvas" aria-hidden="true" />
    <div class="login-scanlines" aria-hidden="true" />
    <div class="login-vignette" aria-hidden="true" />

    <section class="login-interface" aria-label="CyberLife 登录">
      <img class="login-brand-image" src="/branding/cyberlife-logo.png" alt="CyberLife" />
      <p v-if="stageLabel" class="login-stage-label" :class="{ loading: busy }">{{ stageLabel }}</p>
      <form class="login-form login-form-orbital" @submit.prevent="onSubmit">
        <label class="login-key">
          <span class="visually-hidden">访问密钥</span>
          <input
            :value="credential"
            class="login-key-input"
            type="password"
            autocomplete="current-password"
            spellcheck="false"
            :disabled="busy || started"
            autofocus
            @input="onInput"
            @keydown.enter.prevent="onSubmit"
          />
          <i class="login-key-ring" aria-hidden="true" />
        </label>
      </form>
      <p class="login-error" role="alert" aria-live="polite">{{ error }}</p>
    </section>
    <div v-if="started" class="login-welcome" :class="{ scrambling }">
      <p class="login-node-caption">CYBERLIFE NODE / PRIMARY LIFE SYSTEM</p>
      <p class="login-greeting" :aria-label="greeting">
        <span
          v-for="glyph in greetingGlyphs"
          :key="glyph.id"
          class="login-scramble-glyph"
          :class="[`is-${glyph.kind}`, `is-${glyph.state}`]"
          aria-hidden="true"
        >{{ glyph.value }}</span>
      </p>
      <p class="login-quote" :aria-label="quote">
        <span
          v-for="glyph in quoteGlyphs"
          :key="glyph.id"
          class="login-scramble-glyph"
          :class="[`is-${glyph.kind}`, `is-${glyph.state}`]"
          aria-hidden="true"
        >{{ glyph.value }}</span>
      </p>
    </div>
  </main>
</template>
