<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { api, type ReaderKey, type Writer } from '../api/client'
import EmptyState from '../components/EmptyState.vue'
import AppIcon from '../components/AppIcon.vue'

const writers = ref<Writer[]>([])
const selected = ref<Writer | null>(null)
const readerKeys = ref<ReaderKey[]>([])
const nickname = ref('')
const readerNickname = ref('')
const note = ref('')
const issued = ref<{ title: string; key: string } | null>(null)
const copied = ref(false)
const error = ref('')
const busy = ref(false)

function fail(cause: unknown, fallback: string) { error.value = cause instanceof Error ? cause.message : fallback }
async function refresh() { try { writers.value = (await api.writers()).items } catch (cause) { fail(cause, '读取失败') } }
async function select(writer: Writer) { selected.value = writer; try { readerKeys.value = (await api.readerKeys(writer.life_id)).items } catch (cause) { fail(cause, '读取失败') } }
async function createWriter() {
  const name = nickname.value.trim()
  if (!name || busy.value) return
  busy.value = true
  try { const result = await api.createWriter(name); issued.value = { title: `书写者「${result.writer.nickname}」的主密钥`, key: result.master_key }; copied.value = false; nickname.value = ''; await refresh(); await select(result.writer) } catch (cause) { fail(cause, '创建失败') } finally { busy.value = false }
}
async function createReader() {
  if (!selected.value || !readerNickname.value.trim() || busy.value) return
  busy.value = true
  try { const result = await api.createReaderKey(selected.value.life_id, { nickname: readerNickname.value.trim(), note: note.value.trim() }); issued.value = { title: `阅读者「${result.reader_key.nickname}」的密钥`, key: result.key }; copied.value = false; readerNickname.value = ''; note.value = ''; await select(selected.value) } catch (cause) { fail(cause, '创建失败') } finally { busy.value = false }
}
async function revoke(key: ReaderKey) {
  if (!confirm(`确认作废「${key.nickname}」的阅读密钥？已有会话会立即失效。`)) return
  try { await api.revokeReaderKey(key.id); if (selected.value) await select(selected.value) } catch (cause) { fail(cause, '作废失败') }
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
      <aside class="card">
        <h2 class="card-title">创建书写者</h2>
        <form class="form-row" @submit.prevent="createWriter"><input v-model="nickname" placeholder="书写者昵称" maxlength="30" required /><button class="primary" type="submit" :disabled="busy || !nickname.trim()">创建</button></form>
        <h2 class="card-title">书写者</h2>
        <ul v-if="writers.length" class="writer-list">
          <li v-for="writer in writers" :key="writer.id"><button v-glow class="writer-item" :class="{ active: selected?.id === writer.id }" @click="select(writer)"><b>{{ writer.nickname }}</b><small>{{ writer.status }} · {{ writer.created_at.slice(0, 10) }}</small></button></li>
        </ul>
        <EmptyState v-else icon="key" text="还没有书写者" compact />
      </aside>
      <section class="card">
        <Transition name="fade-slide" mode="out-in">
          <div v-if="selected" :key="selected.id" class="preset-editor">
            <h2 class="card-title">{{ selected.nickname }} 的阅读密钥<small class="mono">{{ selected.life_id }}</small></h2>
            <form class="reader-form" @submit.prevent="createReader"><input v-model="readerNickname" placeholder="阅读者昵称" maxlength="30" required /><input v-model="note" placeholder="备注（可选）" maxlength="60" /><button class="primary" type="submit" :disabled="busy || !readerNickname.trim()">生成密钥</button></form>
            <ul v-if="readerKeys.length" class="key-list">
              <li v-for="key in readerKeys" :key="key.id" :class="{ revoked: key.revoked_at }">
                <span class="key-main"><b>{{ key.nickname }}</b><small class="faint">{{ key.note || '无备注' }}</small></span>
                <span class="key-meta mono faint">锚点 {{ key.anchor_local_date }}</span>
                <span v-if="key.revoked_at" class="key-status off">已作废</span>
                <button v-else class="text-button danger" @click="revoke(key)">作废</button>
              </li>
            </ul>
            <EmptyState v-else icon="key" text="尚未创建阅读密钥" compact />
          </div>
          <EmptyState v-else icon="key" text="从左侧选择一位书写者" />
        </Transition>
      </section>
    </section>
  </main>
</template>
