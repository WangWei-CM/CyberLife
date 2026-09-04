<script setup lang="ts">
import { computed } from 'vue'
const props=defineProps<{ values:number[]; color?:string }>()
const points=computed(()=>{const data=props.values.length?props.values:[0];const low=Math.min(...data),high=Math.max(...data);const span=high-low||1;return data.map((value,index)=>`${(index/(Math.max(1,data.length-1)))*100},${86-((value-low)/span)*72}`).join(' ')})
</script>
<template><svg class="metric-line" viewBox="0 0 100 100" preserveAspectRatio="none" role="img" aria-label="趋势折线"><path d="M0 90H100" class="metric-base"/><polyline :points="points" :style="{stroke:color||'var(--accent)'}"/></svg></template>