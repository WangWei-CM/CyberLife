<script setup lang="ts">
import { nextTick, ref } from 'vue'
import { api } from '../api/client'
import LoginTransition from '../components/LoginTransition.vue'
import { beijingNow } from '../lib/dates'
import { wait } from '../lib/motion'

const emit = defineEmits<{ authenticated: [actor: Awaited<ReturnType<typeof api.keyLogin>>['actor']] }>()
const credential = ref('')
const error = ref('')
const greeting = ref('')
const quote = ref('')
const quotes = [
  '老当益壮，宁移白首之心？穷且益坚，不坠青云之志。',
  '长风破浪会有时，直挂云帆济沧海。',
  '不积跬步，无以至千里；不积小流，无以成江海。',
  '千磨万击还坚劲，任尔东西南北风。',
  '路漫漫其修远兮，吾将上下而求索。',
  '山重水复疑无路，柳暗花明又一村。',
  '天行健，君子以自强不息。',
  '纸上得来终觉浅，绝知此事要躬行。',
  '沉舟侧畔千帆过，病树前头万木春。',
  '会当凌绝顶，一览众山小。',
]
function greetingForMoment(name: string) {
  const hour = beijingNow().getHours()
  const period = hour < 5 ? '凌晨好' : hour < 8 ? '清晨好' : hour < 10 ? '早上好' : hour < 12 ? '上午好' : hour < 14 ? '中午好' : hour < 17 ? '下午好' : hour < 19 ? '傍晚好' : hour < 23 ? '晚上好' : '凌晨好'
  return `${period}，${name || '书写者'}`
}
const busy = ref(false)
const authenticatedActor = ref<Awaited<ReturnType<typeof api.keyLogin>>['actor']>()
const transition = ref<InstanceType<typeof LoginTransition>>()
const underlayReady = ref(false)

async function waitForUnderlay() {
  const deadline = performance.now() + 1_600
  while (!underlayReady.value && performance.now() < deadline) await wait(50)
  if (underlayReady.value) await wait(100)
}

async function submit() {
  const key = credential.value.trim()
  if (!key || busy.value || authenticatedActor.value) return
  error.value = ''
  transition.value?.setStage('authenticating')
  busy.value = true
  try {
    const result = await api.keyLogin(key)
    authenticatedActor.value = result.actor
    credential.value = ''
    greeting.value = greetingForMoment(result.actor.nickname)
    quote.value = quotes[Math.floor(Math.random() * quotes.length)]
    await nextTick()
    await waitForUnderlay()
    await transition.value?.beginTransition()
  } catch (cause) {
    transition.value?.setStage('orbital-login')
    error.value = cause instanceof Error ? cause.message : '密钥无效'
  } finally {
    busy.value = false
  }
}

function completeTransition() {
  if (!authenticatedActor.value) return
  emit('authenticated', authenticatedActor.value)
}
</script>

<template>
  <div class="login-composite">
    <iframe
      v-if="authenticatedActor"
      class="login-underlay"
      src="/"
      title=""
      tabindex="-1"
      aria-hidden="true"
      @load="underlayReady = true"
    />
    <LoginTransition
      ref="transition"
      v-model:credential="credential"
      class="login-overlay"
      :busy="busy"
      :error="error"
      :greeting="greeting"
      :quote="quote"
      @submit="submit"
      @transition-complete="completeTransition"
    />
  </div>
</template>
