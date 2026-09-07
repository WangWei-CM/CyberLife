<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { MdEditor, NormalToolbar, type ToolbarNames } from 'md-editor-v3'
import 'md-editor-v3/lib/style.css'
import { api } from '../api/client'
import AppIcon from './AppIcon.vue'
import ImageLightbox from './ImageLightbox.vue'
import EmptyState from './EmptyState.vue'

/**
 * 全系统复用的 MD 编辑器（三期 §5）：
 * 1. 图片点击放大：自研 overlay，弃用 medium-zoom
 * 2. 删除备份审计：单次删除 ≥50 字自动快照到 localStorage，可查看/恢复；不重写 CodeMirror undo
 * 3. 保留图片/链接/公式/图表等全部工具栏项
 */
const props = withDefaults(defineProps<{ modelValue: string; theme: 'light' | 'dark'; vaultKey: string; placeholder?: string; editorId?: string; secret?: boolean; slantedLines?: boolean }>(), { placeholder: '', editorId: 'diary-editor', secret: false, slantedLines: false })
const emit = defineEmits<{ (event: 'update:modelValue', value: string): void; (event: 'change', value: string): void }>()

type Snapshot = { at: string; text: string }
const SNAPSHOT_THRESHOLD = 50
const root = ref<HTMLElement>()
const vaultOpen = ref(false)
const snapshots = ref<Snapshot[]>([])
const lightbox = ref<{ src: string; alt: string } | null>(null)
const themeClass = ref('')
let lastAudited = props.modelValue
let auditTimer: number | undefined
let slantedLineObserver: MutationObserver | undefined
let slantedLineResizeObserver: ResizeObserver | undefined
let slantedLineFrame: number | undefined

const storageKey = computed(() => `cyberlife-diary-vault:${props.vaultKey}`)
const toolbars: ToolbarNames[] = ['bold', 'underline', 'italic', 'strikeThrough', '-', 'title', 'sub', 'sup', 'quote', 'unorderedList', 'orderedList', 'task', '-', 'codeRow', 'code', 'link', 'image', 'table', 'mermaid', 'katex', '-', 'revoke', 'next', 0, '=', 'preview', 'previewOnly', 'catalog']

function loadSnapshots() { try { snapshots.value = JSON.parse(localStorage.getItem(storageKey.value) || '[]') } catch { snapshots.value = [] } }
function saveSnapshots() { localStorage.setItem(storageKey.value, JSON.stringify(snapshots.value.slice(0, 50))) }
function paragraphs(text: string) { return text.split(/\n{2,}/).map(item => item.trim()).filter(Boolean) }
function overlap(a: string, b: string) {
  let prefix = 0
  while (prefix < a.length && prefix < b.length && a[prefix] === b[prefix]) prefix++
  let suffix = 0
  while (suffix < a.length - prefix && suffix < b.length - prefix && a[a.length - 1 - suffix] === b[b.length - 1 - suffix]) suffix++
  return prefix + suffix
}
/** 对比前后内容，找出被删除的段落；净删除字数 ≥ 阈值时存快照。 */
function audit(next: string) {
  const before = paragraphs(lastAudited)
  const after = paragraphs(next)
  lastAudited = next
  const remaining = [...after]
  const removed: string[] = []
  for (const paragraph of before) {
    const index = remaining.indexOf(paragraph)
    if (index >= 0) remaining.splice(index, 1)
    else removed.push(paragraph)
  }
  if (!removed.length) return
  const net = removed.reduce((sum, paragraph) => sum + Math.max(0, paragraph.length - Math.max(0, ...remaining.map(item => overlap(paragraph, item)))), 0)
  if (net < SNAPSHOT_THRESHOLD) return
  snapshots.value.unshift({ at: new Date().toISOString(), text: removed.join('\n\n') })
  saveSnapshots()
}
function onChange(value: string) {
  if (value === props.modelValue) return
  emit('update:modelValue', value)
  emit('change', value)
  if (auditTimer) clearTimeout(auditTimer)
  auditTimer = window.setTimeout(() => audit(value), 400)
  // CodeMirror 会先替换行节点、再发出更新事件；此处同步一次可避免新行先以
  // 默认竖直位置绘制一帧。MutationObserver 仍会在同一轮微任务中校准新节点。
  syncSlantedLines()
}
function syncSlantedLines() {
  slantedLineFrame = undefined
  const editor = root.value?.querySelector<HTMLElement>('.md-editor')
  const content = editor?.querySelector<HTMLElement>('.cm-content')
  if (!editor || !content) return
  const lines = content.querySelectorAll<HTMLElement>('.cm-line')
  if (!props.slantedLines) {
    lines.forEach(line => line.style.removeProperty('padding-left'))
    return
  }
  const slope = Math.tan(Math.PI / 12)
  const run = Math.min(150, Math.max(86, editor.clientHeight * slope))
  const editorTop = editor.getBoundingClientRect().top
  lines.forEach(line => {
    const lineTop = line.getBoundingClientRect().top - editorTop
    line.style.paddingLeft = `${Math.max(0, Math.round(run - lineTop * slope + 12))}px`
  })
}
function scheduleSlantedLines() {
  if (!props.slantedLines || slantedLineFrame !== undefined) return
  slantedLineFrame = window.requestAnimationFrame(syncSlantedLines)
}
function setupSlantedLines() {
  if (!props.slantedLines) return
  const editor = root.value?.querySelector<HTMLElement>('.md-editor')
  const content = editor?.querySelector<HTMLElement>('.cm-content')
  const observerRoot = root.value
  if (!editor || !content || !observerRoot) { window.requestAnimationFrame(setupSlantedLines); return }
  // 行节点在输入、换行与自动换行时会被 CodeMirror 整体替换。不能把这个校准
  // 延迟到 requestAnimationFrame，否则新行会在默认位置和斜向位置之间闪跳。
  // 监听必须挂在稳定的组件根节点，不能挂在 cm-content：短行输入时 md-editor-v3
  // 会直接替换 cm-content，原先的监听器随之失效，后续行便会回到竖直默认布局。
  slantedLineObserver = new MutationObserver(syncSlantedLines)
  slantedLineObserver.observe(observerRoot, { childList: true, characterData: true, subtree: true })
  slantedLineResizeObserver = new ResizeObserver(scheduleSlantedLines)
  slantedLineResizeObserver.observe(observerRoot)
  syncSlantedLines()
  // 首次挂载时 CodeMirror 可能在本轮之后补齐可见行；下一帧仅作为初始化兜底。
  scheduleSlantedLines()
}
function restore(snapshot: Snapshot) {
  const merged = props.modelValue.trimEnd() ? `${props.modelValue.trimEnd()}\n\n${snapshot.text}\n` : `${snapshot.text}\n`
  lastAudited = merged
  emit('update:modelValue', merged)
  emit('change', merged)
  snapshots.value = snapshots.value.filter(item => item !== snapshot)
  saveSnapshots()
}
function discard(snapshot: Snapshot) { snapshots.value = snapshots.value.filter(item => item !== snapshot); saveSnapshots() }
async function uploadImages(files: File[], callback: (urls: string[]) => void) {
  const urls: string[] = []
  for (const file of files) {
    try { const attachment = await api.uploadAttachment(file); urls.push(api.attachmentUrl(attachment.id)) } catch { /* 上传失败的图片跳过 */ }
  }
  callback(urls)
}
function onPreviewClick(event: MouseEvent) {
  const target = event.target as HTMLElement
  if (target.tagName !== 'IMG' || !target.closest('.md-editor-preview')) return
  event.preventDefault()
  themeClass.value = document.querySelector('.app-shell')?.className.replace(/\b(shell-enter|drop-target|theme-shift)\b/g, '').trim() ?? ''
  lightbox.value = { src: (target as HTMLImageElement).src, alt: (target as HTMLImageElement).alt }
}
function onDocumentClick(event: MouseEvent) { if (vaultOpen.value && root.value && !root.value.querySelector('.vault')?.contains(event.target as Node) && !(event.target as HTMLElement).closest('.vault-trigger')) vaultOpen.value = false }
function snapshotLabel(value: string) { const date = new Date(value); return `${date.getMonth() + 1}月${date.getDate()}日 ${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}` }

watch(storageKey, () => { loadSnapshots(); lastAudited = props.modelValue })
watch(() => props.modelValue, value => { if (auditTimer === undefined && value !== lastAudited && Math.abs(value.length - lastAudited.length) > 200) lastAudited = value })
onMounted(() => { loadSnapshots(); root.value?.addEventListener('click', onPreviewClick); document.addEventListener('click', onDocumentClick); setupSlantedLines() })
onBeforeUnmount(() => { root.value?.removeEventListener('click', onPreviewClick); document.removeEventListener('click', onDocumentClick); if (auditTimer) clearTimeout(auditTimer); if (slantedLineFrame !== undefined) cancelAnimationFrame(slantedLineFrame); slantedLineObserver?.disconnect(); slantedLineResizeObserver?.disconnect() })
</script>

<template>
  <div ref="root" class="diary-editor" :class="{ secret }">
    <MdEditor :model-value="modelValue" :editor-id="editorId" :theme="theme" language="zh-CN" :toolbars="toolbars" :placeholder="placeholder" :no-img-zoom-in="true" :preview="false" :auto-fold-threshold="60" @update:model-value="onChange" @on-upload-img="uploadImages">
      <template #defToolbars>
        <NormalToolbar title="删除备份" class="vault-trigger" @on-click="vaultOpen = !vaultOpen">
          <template #trigger><span class="vault-icon" :class="{ has: snapshots.length }"><AppIcon name="history" :size="16" /><i v-if="snapshots.length" /></span></template>
        </NormalToolbar>
      </template>
    </MdEditor>
    <Transition name="popover">
      <section v-if="vaultOpen" class="popover vault" aria-label="删除备份">
        <header class="popover-head"><b>删除备份</b><small class="faint">单次删除 ≥ {{ SNAPSHOT_THRESHOLD }} 字自动保存</small></header>
        <EmptyState v-if="!snapshots.length" icon="history" text="还没有被删除的段落" compact />
        <ul v-else class="vault-list">
          <li v-for="snapshot in snapshots" :key="snapshot.at">
            <small class="mono">{{ snapshotLabel(snapshot.at) }} · {{ snapshot.text.length }} 字</small>
            <p>{{ snapshot.text }}</p>
            <div class="vault-actions"><button class="text-button" @click="restore(snapshot)"><AppIcon name="restore" :size="14" />恢复到文末</button><button class="text-button danger" @click="discard(snapshot)"><AppIcon name="trash" :size="14" />丢弃</button></div>
          </li>
        </ul>
      </section>
    </Transition>
    <Transition name="lightbox">
      <ImageLightbox v-if="lightbox" :src="lightbox.src" :alt="lightbox.alt" :theme-class="themeClass" @close="lightbox = null" />
    </Transition>
  </div>
</template>
