<script setup lang="ts">
import { ref } from 'vue'
import { api } from '../api/client'
import LoginTransition from '../components/LoginTransition.vue'

const emit = defineEmits<{ authenticated: [actor: Awaited<ReturnType<typeof api.keyLogin>>['actor']] }>()
const credential = ref('')
const error = ref('')
const busy = ref(false)
const authenticatedActor = ref<Awaited<ReturnType<typeof api.keyLogin>>['actor']>()
const transition = ref<InstanceType<typeof LoginTransition>>()

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
  <LoginTransition
    ref="transition"
    v-model:credential="credential"
    :busy="busy"
    :error="error"
    @submit="submit"
    @transition-complete="completeTransition"
  />
</template>
