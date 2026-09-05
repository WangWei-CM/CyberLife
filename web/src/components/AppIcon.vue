<script setup lang="ts">
import { computed } from 'vue'

/** 线性 SVG 图标集（24 viewBox，描边随 currentColor）。三屏通过颜色变量重新着色。 */
const icons: Record<string, string> = {
  play: '<path d="M8 5.5v13l11-6.5z" fill="currentColor" stroke="none"/>',
  stop: '<rect x="6.5" y="6.5" width="11" height="11" rx="1.5" fill="currentColor" stroke="none"/>',
  sun: '<circle cx="12" cy="12" r="4"/><path d="M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4"/>',
  moon: '<path d="M20.5 14.2A8.5 8.5 0 0 1 9.8 3.5a8.5 8.5 0 1 0 10.7 10.7z"/>',
  gear: '<circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.7 1.7 0 0 0 .3 1.8l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.7 1.7 0 0 0-1.8-.3 1.7 1.7 0 0 0-1 1.5V21a2 2 0 1 1-4 0v-.1a1.7 1.7 0 0 0-1.1-1.5 1.7 1.7 0 0 0-1.8.3l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1a1.7 1.7 0 0 0 .3-1.8 1.7 1.7 0 0 0-1.5-1H3a2 2 0 1 1 0-4h.1a1.7 1.7 0 0 0 1.5-1.1 1.7 1.7 0 0 0-.3-1.8l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1a1.7 1.7 0 0 0 1.8.3H9a1.7 1.7 0 0 0 1-1.5V3a2 2 0 1 1 4 0v.1a1.7 1.7 0 0 0 1 1.5 1.7 1.7 0 0 0 1.8-.3l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1a1.7 1.7 0 0 0-.3 1.8V9a1.7 1.7 0 0 0 1.5 1H21a2 2 0 1 1 0 4h-.1a1.7 1.7 0 0 0-1.5 1z"/>',
  bell: '<path d="M6 8.5a6 6 0 0 1 12 0c0 6.5 2.5 8.5 2.5 8.5h-17S6 15 6 8.5"/><path d="M10.3 21a2 2 0 0 0 3.4 0"/>',
  lock: '<rect x="4.5" y="10.5" width="15" height="10" rx="1.5"/><path d="M8 10.5V7a4 4 0 0 1 8 0v3.5"/><circle cx="12" cy="15.5" r="1" fill="currentColor" stroke="none"/>',
  unlock: '<rect x="4.5" y="10.5" width="15" height="10" rx="1.5"/><path d="M8 10.5V7a4 4 0 0 1 7.6-1.8"/><circle cx="12" cy="15.5" r="1" fill="currentColor" stroke="none"/>',
  logout: '<path d="M9.5 21H5.5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4M16 17l5-5-5-5M21 12H9.5"/>',
  'chevron-left': '<path d="M14.5 6l-6 6 6 6"/>',
  'chevron-right': '<path d="M9.5 6l6 6-6 6"/>',
  'chevron-down': '<path d="M6 9.5l6 6 6-6"/>',
  check: '<path d="M20 6.5L9 17.5l-5-5"/>',
  trash: '<path d="M3.5 6.5h17M8.5 6.5V4.5h7v2M18.5 6.5l-1 14h-11l-1-14M10 11v6M14 11v6"/>',
  close: '<path d="M18 6L6 18M6 6l12 12"/>',
  plus: '<path d="M12 5v14M5 12h14"/>',
  minus: '<path d="M5 12h14"/>',
  image: '<rect x="3.5" y="4.5" width="17" height="15" rx="1.5"/><circle cx="9" cy="10" r="1.6"/><path d="M20.5 15.5l-4.5-4.5-8 8"/>',
  restore: '<path d="M3.5 12a8.5 8.5 0 1 0 2.6-6.1L3.5 8.5M3.5 3.5v5h5"/>',
  history: '<path d="M3.5 12a8.5 8.5 0 1 0 2.6-6.1L3.5 8.5M3.5 3.5v5h5M12 7.5v5l3 2"/>',
  grip: '<circle cx="9" cy="6" r="1.3" fill="currentColor" stroke="none"/><circle cx="15" cy="6" r="1.3" fill="currentColor" stroke="none"/><circle cx="9" cy="12" r="1.3" fill="currentColor" stroke="none"/><circle cx="15" cy="12" r="1.3" fill="currentColor" stroke="none"/><circle cx="9" cy="18" r="1.3" fill="currentColor" stroke="none"/><circle cx="15" cy="18" r="1.3" fill="currentColor" stroke="none"/>',
  music: '<path d="M9 18.5V5.5l11.5-2v13"/><circle cx="6.5" cy="18.5" r="2.5"/><circle cx="18" cy="16.5" r="2.5"/>',
  shuffle: '<path d="M16.5 3.5l3.5 3.5-3.5 3.5M4 7h3.5c3 0 4.5 2 6 5s3 5 6 5H20M16.5 13.5L20 17l-3.5 3.5M4 17h3.5c1.5 0 2.7-.6 3.7-1.6"/>',
  repeat: '<path d="M17.5 2.5L21 6l-3.5 3.5M3 11.5V9a3 3 0 0 1 3-3h15M6.5 21.5L3 18l3.5-3.5M21 12.5V15a3 3 0 0 1-3 3H3"/>',
  'repeat-one': '<path d="M17.5 2.5L21 6l-3.5 3.5M3 11.5V9a3 3 0 0 1 3-3h15M6.5 21.5L3 18l3.5-3.5M21 12.5V15a3 3 0 0 1-3 3H3M11 10l1.8-1v6"/>',
  volume: '<path d="M11 5.5L6.5 9H3v6h3.5L11 18.5zM15.5 8.5a5 5 0 0 1 0 7M18.5 5.5a9 9 0 0 1 0 13"/>',
  calendar: '<rect x="3.5" y="5" width="17" height="15.5" rx="1.5"/><path d="M3.5 10h17M8 3v4M16 3v4"/>',
  flag: '<path d="M5 21.5V4.5a1 1 0 0 1 1-1h11.5l-1.5 4 1.5 4H5"/>',
  star: '<path d="M12 3.5l2.6 5.4 5.9.8-4.3 4.1 1.1 5.9-5.3-2.8-5.3 2.8 1.1-5.9-4.3-4.1 5.9-.8z"/>',
  medal: '<circle cx="12" cy="10" r="5.5"/><path d="M12 7.2l.9 1.9 2.1.3-1.5 1.4.4 2.1-1.9-1-1.9 1 .4-2.1-1.5-1.4 2.1-.3zM9 14.5l-2 6 5-2.5 5 2.5-2-6" stroke-width="1.4"/>',
  inbox: '<path d="M22 12.5h-6l-2 3h-4l-2-3H2M5.5 5.5h13l3.5 7v7H2v-7z"/>',
  key: '<path d="M21 2.5l-2 2M11.4 12.6a5.5 5.5 0 1 1-7.8-7.8 5.5 5.5 0 0 1 7.8 7.8zM15.5 8.5l2.9 2.9M19 5l3-3"/>',
  edit: '<path d="M12 20.5h9M16.5 3.5a2.1 2.1 0 0 1 3 3L7 19l-4 1 1-4z"/>',
  copy: '<rect x="9" y="9" width="12" height="12" rx="1.5"/><path d="M5 15H4a1.5 1.5 0 0 1-1.5-1.5V4A1.5 1.5 0 0 1 4 2.5h9.5A1.5 1.5 0 0 1 15 4v1"/>',
  eye: '<path d="M2 12s3.5-6.5 10-6.5S22 12 22 12s-3.5 6.5-10 6.5S2 12 2 12z"/><circle cx="12" cy="12" r="3"/>',
  'eye-off': '<path d="M3 3l18 18M10.6 5.9A10 10 0 0 1 12 5.5c6.5 0 10 6.5 10 6.5a15 15 0 0 1-3.3 4.1M6.6 6.6C3.8 8.4 2 12 2 12s3.5 6.5 10 6.5c1.6 0 3-.4 4.2-1M9.9 9.9a3 3 0 0 0 4.2 4.2"/>',
  more: '<circle cx="5.5" cy="12" r="1.4" fill="currentColor" stroke="none"/><circle cx="12" cy="12" r="1.4" fill="currentColor" stroke="none"/><circle cx="18.5" cy="12" r="1.4" fill="currentColor" stroke="none"/>',
  smile: '<circle cx="12" cy="12" r="9"/><path d="M8.5 14.5s1.2 2 3.5 2 3.5-2 3.5-2M9 9.5h.01M15 9.5h.01"/>',
  pulse: '<path d="M3 12h4l2-6 3 12 3-9 1.5 3H21"/>',
  book: '<path d="M4 4.5A1.5 1.5 0 0 1 5.5 3H20v16.5H5.5A1.5 1.5 0 0 0 4 21z"/><path d="M4 19.5A1.5 1.5 0 0 1 5.5 18H20"/>',
  target: '<circle cx="12" cy="12" r="9"/><circle cx="12" cy="12" r="5"/><circle cx="12" cy="12" r="1.2" fill="currentColor" stroke="none"/>',
  'zoom-in': '<circle cx="11" cy="11" r="7"/><path d="M21 21l-4.5-4.5M11 8v6M8 11h6"/>',
  'zoom-out': '<circle cx="11" cy="11" r="7"/><path d="M21 21l-4.5-4.5M8 11h6"/>',
  comment: '<path d="M21 12a8 8 0 0 1-8 8H8l-5 3v-11a8 8 0 0 1 8-8h2a8 8 0 0 1 8 8z"/>',
  send: '<path d="M22 2L11 13M22 2l-7 20-4-9-9-4z"/>',
  up: '<path d="M12 19V5M5 12l7-7 7 7"/>',
  down: '<path d="M12 5v14M19 12l-7 7-7-7"/>',
  drag: '<path d="M8 5h.01M16 5h.01M8 12h.01M16 12h.01M8 19h.01M16 19h.01" stroke-width="3"/>',
  spark: '<path d="M12 3l1.8 5.2L19 10l-5.2 1.8L12 17l-1.8-5.2L5 10l5.2-1.8z"/>',
  layout: '<rect x="3.5" y="3.5" width="17" height="17" rx="1.5"/><path d="M3.5 9h17M9 9v11.5"/>',
  download: '<path d="M12 3.5v12M7 11l5 5 5-5M4 20.5h16"/>',
  upload: '<path d="M12 15.5v-12M7 8l5-5 5 5M4 20.5h16"/>',
  clock: '<circle cx="12" cy="12" r="9"/><path d="M12 7v5l3.5 2"/>',
  shield: '<path d="M12 2.5l8 3.5v6c0 5-3.5 8.5-8 9.5-4.5-1-8-4.5-8-9.5V6z"/>',
}
const props = withDefaults(defineProps<{ name: string; size?: number; strokeWidth?: number }>(), { size: 18, strokeWidth: 1.75 })
const markup = computed(() => icons[props.name] ?? '')
</script>

<template>
  <svg class="app-icon" :class="`icon-${name}`" :width="size" :height="size" viewBox="0 0 24 24" fill="none" stroke="currentColor" :stroke-width="strokeWidth" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true" v-html="markup" />
</template>
