<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { createLoginTransition, type LoginTransitionStage } from '../lib/login-transition'
import { reducedMotion, wait } from '../lib/motion'
import AppIcon from './AppIcon.vue'

const props = defineProps<{ credential: string; busy: boolean; error: string }>()
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
  if (stage.value === 'node-fall' || stage.value === 'page-enter') return 'LIFE OPERATING SYSTEM'
  if (props.busy || stage.value === 'authenticating') return 'VERIFYING ACCESS'
  return 'ENTER ACCESS KEY'
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
  <main class="login-transition" :class="{ 'scene-failed': sceneFailed, 'transition-started': started }">
    <canvas ref="canvas" class="login-canvas" aria-hidden="true" />
    <div class="login-scanlines" aria-hidden="true" />
    <div class="login-vignette" aria-hidden="true" />
    <div class="login-corner login-corner-tl" aria-hidden="true" />
    <div class="login-corner login-corner-br" aria-hidden="true" />

    <section class="login-interface" aria-label="CyberLife 登录">
      <p class="login-kicker">LIFE OPERATING SYSTEM</p>
      <h1 class="login-logo" data-text="CYBERLIFE">CYBERLIFE</h1>
      <p class="login-stage-label" :class="{ loading: busy }">{{ stageLabel }}</p>
      <form class="login-form login-form-orbital" @submit.prevent="onSubmit">
        <label class="login-key">
          <span class="visually-hidden">访问密钥</span>
          <input
            :value="credential"
            class="login-key-input"
            type="password"
            autocomplete="current-password"
            spellcheck="false"
            placeholder="ENTER ACCESS KEY"
            :disabled="busy || started"
            autofocus
            @input="onInput"
          />
          <i class="login-key-ring" aria-hidden="true" />
        </label>
        <button v-glow class="login-submit" type="submit" aria-label="验证访问密钥" :disabled="busy || started || !credential.trim()">
          <AppIcon v-if="!busy" name="chevron-right" :size="20" />
          <span v-else class="login-spinner" aria-hidden="true" />
        </button>
      </form>
      <p class="login-error" role="alert" aria-live="polite">{{ error }}</p>
      <p v-if="started" class="login-node-caption">CYBERLIFE NODE / PRIMARY LIFE SYSTEM</p>
      <p v-else class="login-footnote">SECURE ORBITAL ACCESS // SESSION READY</p>
    </section>
  </main>
</template>
