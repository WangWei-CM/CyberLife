<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { api } from '../api/client'

const props = withDefaults(defineProps<{ modelValue: string; placeholder?: string }>(), { placeholder: '任务详细描述（支持 Markdown）' })
const emit = defineEmits<{ (event: 'update:modelValue', value: string): void }>()
type Row = { from: number; to: number; value: string; inset: number }
const root = ref<HTMLElement>()
const width = ref(640)
const active = ref({ from: 0, to: 0 })
let observer: ResizeObserver | undefined

const rows = computed<Row[]>(() => {
  const char = 9.1, step = 20 * Math.tan(Math.PI / 12), run = Math.min(146, Math.max(84, width.value * .14))
  const output: Row[] = []
  let offset = 0, index = 0
  for (const logical of props.modelValue.split('\n')) {
    const max = Math.max(12, Math.floor((width.value - Math.max(0, run - index * step) - 20) / char))
    const parts = logical.length ? logical.match(new RegExp(`.{1,${max}}`, 'g')) || [''] : ['']
    let local = 0
    for (const value of parts) {
      output.push({ from: offset + local, to: offset + local + value.length, value, inset: Math.max(0, run - index++ * step) })
      local += value.length
    }
    offset += logical.length + 1
  }
  return output
})

function replace(from: number, to: number, value: string, caret = from + value.length) {
  emit('update:modelValue', `${props.modelValue.slice(0, from)}${value}${props.modelValue.slice(to)}`)
  active.value = { from: caret, to: caret }
  nextTick(() => {
    const input = root.value?.querySelector<HTMLInputElement>(`input[data-from="${from}"]`)
    if (!input) return
    const position = Math.min(input.value.length, Math.max(0, caret - from))
    input.focus(); input.setSelectionRange(position, position)
  })
}
function input(row: Row, event: Event) {
  const field = event.target as HTMLInputElement
  const start = row.from + field.selectionStart!
  replace(row.from, row.to, field.value, start)
}
function focus(row: Row, event: Event) {
  const field = event.target as HTMLInputElement
  active.value = { from: row.from + field.selectionStart!, to: row.from + field.selectionEnd! }
}
function keydown(row: Row, event: KeyboardEvent) {
  const field = event.target as HTMLInputElement
  const start = field.selectionStart ?? 0
  if (event.key === 'Enter') { event.preventDefault(); replace(row.from + start, row.from + (field.selectionEnd ?? start), '\n'); return }
  if (event.key === 'Backspace' && start === 0 && row.from > 0) { event.preventDefault(); replace(row.from - 1, row.from, '', row.from - 1) }
}
function wrap(before: string, after = before) {
  const { from, to } = active.value
  replace(from, to, `${before}${props.modelValue.slice(from, to)}${after}`, to + before.length + after.length)
}
async function insertImage(event: Event) {
  const file = (event.target as HTMLInputElement).files?.[0]
  if (!file) return
  try { const attachment = await api.uploadAttachment(file); wrap(`![${file.name}](${api.attachmentUrl(attachment.id)})`) } finally { (event.target as HTMLInputElement).value = '' }
}
onMounted(() => { observer = new ResizeObserver(([entry]) => { width.value = entry.contentRect.width }); if (root.value) observer.observe(root.value) })
onBeforeUnmount(() => observer?.disconnect())
</script>

<template>
  <section ref="root" class="slanted-markdown-editor" aria-label="Markdown 编辑器">
    <nav class="slanted-markdown-toolbar" aria-label="Markdown 工具栏">
      <button type="button" title="加粗" @click="wrap('**', '**')">B</button><button type="button" title="斜体" @click="wrap('*', '*')">I</button><button type="button" title="删除线" @click="wrap('~~', '~~')">S</button><i />
      <button type="button" title="标题" @click="wrap('## ')">H</button><button type="button" title="引用" @click="wrap('> ')">❯</button><button type="button" title="列表" @click="wrap('- ')">☷</button><button type="button" title="代码" @click="wrap('`', '`')">&lt;/&gt;</button><button type="button" title="链接" @click="wrap('[', '](url)')">↗</button>
      <label title="上传图片">▧<input type="file" accept="image/*" @change="insertImage" /></label>
    </nav>
    <div class="slanted-markdown-lines">
      <input v-for="row in rows" :key="`${row.from}:${row.to}`" class="slanted-markdown-row" :style="{ marginLeft: `${row.inset}px`, width: `calc(100% - ${row.inset}px)` }" :data-from="row.from" :value="row.value" :placeholder="row.from === 0 ? placeholder : ''" spellcheck="true" @input="input(row, $event)" @focus="focus(row, $event)" @click="focus(row, $event)" @select="focus(row, $event)" @keydown="keydown(row, $event)" />
    </div>
  </section>
</template>
