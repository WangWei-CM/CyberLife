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
let scene: ReturnType<typeof createLoginTransition> | undefined

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
  await wait(220)
  scene?.setStage('orbital-login')
  await wait(520)
  scene?.setStage('node-reveal')
  if (reducedMotion()) await wait(50)
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
      onStage: value => { stage.value = value },
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
  scene?.destroy()
  scene = undefined
})

defineExpose({
  beginTransition,
  setStage: (next: LoginTransitionStage) => scene?.setStage(next),
})
</script>

<template>
  <main class="login-transition" :class="{ 'scene-failed': sceneFailed, 'transition-started': started, 'node-reveal': stage === 'node-reveal', 'node-fall': stage === 'node-fall' }">
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
    <div v-if="started" class="login-welcome">
      <p class="login-node-caption">CYBERLIFE NODE / PRIMARY LIFE SYSTEM</p>
      <p class="login-greeting">{{ greeting }}</p>
      <p class="login-quote">{{ quote }}</p>
    </div>
  </main>
</template>
