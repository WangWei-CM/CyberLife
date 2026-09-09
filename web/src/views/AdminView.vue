<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { api, type Writer } from '../api/client'
import EmptyState from '../components/EmptyState.vue'
import AppIcon from '../components/AppIcon.vue'

const writers = ref<Writer[]>([])
const nickname = ref('')
const issued = ref<{ title: string; key: string } | null>(null)
const copied = ref(false)
const error = ref('')
const busy = ref(false)

function fail(cause: unknown, fallback: string) { error.value = cause instanceof Error ? cause.message : fallback }
async function refresh() { try { writers.value = (await api.writers()).items } catch (cause) { fail(cause, '读取失败') } }
async function createWriter() {
  const name = nickname.value.trim()
  if (!name || busy.value) return
  busy.value = true
  try { const result = await api.createWriter(name); issued.value = { title: `书写者「${result.writer.nickname}」的主密钥`, key: result.master_key }; copied.value = false; nickname.value = ''; await refresh() } catch (cause) { fail(cause, '创建失败') } finally { busy.value = false }
}
async function copyKey() { if (!issued.value) return; try { await navigator.clipboard.writeText(issued.value.key); copied.value = true; setTimeout(() => { copied.value = false }, 2000) } catch { /* 剪贴板不可用时用户可手动选择复制 */ } }
onMounted(refresh)
</script>

<template>
  <main class="page page-narrow admin-page">
    <header class="admin-head"><h1>人生空间管理</h1><small class="faint">{{ writers.length }} 位书写者</small></header>
    <Transition name="fade"><p v-if="error" class="error page-error" role="alert">{{ error }}<button class="text-button" @click="error = ''"><AppIcon name="close" :size="14" /></button></p></Transition>
    <Transition name="fade-slide">
      <section v-if="issued" class="key-reveal" role="alert">
        <b>{{ issued.title }}</b>
        <p>只显示这一次，请立即保存并交给持有人。</p>
        <code>{{ issued.key }}</code>
        <div class="form-row"><button class="text-button" @click="copyKey"><AppIcon :name="copied ? 'check' : 'copy'" :size="14" />{{ copied ? '已复制' : '复制' }}</button><button class="text-button" @click="issued = null"><AppIcon name="close" :size="14" />关闭</button></div>
      </section>
    </Transition>
    <section v-stagger class="admin-grid">
      <section class="card">
        <h2 class="card-title">创建书写者</h2>
        <form class="form-row" @submit.prevent="createWriter"><input v-model="nickname" placeholder="书写者昵称" maxlength="30" required /><button class="primary" type="submit" :disabled="busy || !nickname.trim()">创建</button></form>
        <h2 class="card-title">书写者</h2>
        <ul v-if="writers.length" class="writer-list">
          <li v-for="writer in writers" :key="writer.id"><div class="writer-item"><b>{{ writer.nickname }}</b><small>{{ writer.status }} · {{ writer.created_at.slice(0, 10) }}</small></div></li>
        </ul>
        <EmptyState v-else icon="key" text="还没有书写者" compact />
      </section>
    </section>
  </main>
</template>
