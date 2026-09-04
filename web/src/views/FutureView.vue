<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { api, type FutureTask, type Plan } from '../api/client'
const today=new Date();const iso=(v:Date)=>v.toISOString().slice(0,10);const add=(base:Date,days:number)=>{const v=new Date(base);v.setDate(v.getDate()+days);return v};const scales=[7,14,30,182,365,1825,3650,10950,36500]
const plans=ref<Plan[]>([]);const tasks=ref<FutureTask[]>([]);const scaleIndex=ref(0);const anchor=ref(new Date());const error=ref('');const selectedPlan=ref<Plan|null>(null);const editMode=ref(false);const progress=ref(0);const name=ref('');const startDate=ref(iso(today));const endDate=ref(iso(add(today,30)));const intro=ref('')
const scale = computed(() => scales[scaleIndex.value])
const cells = computed(() => {
  const count = scale.value <= 30 ? scale.value : scale.value <= 365 ? 12 : scale.value <= 3650 ? 10 : scale.value <= 10950 ? 30 : 100
  const step = scale.value <= 30 ? 'day' : scale.value <= 365 ? 'month' : 'year'
  return Array.from({ length: count }, (_, index) => {
    const date = new Date(anchor.value)
    if (step === 'day') date.setDate(date.getDate() + index)
    else if (step === 'month') date.setMonth(date.getMonth() + index)
    else date.setFullYear(date.getFullYear() + index)
    const label = step === 'day'
      ? new Intl.DateTimeFormat('zh-CN', { month: 'short', day: 'numeric' }).format(date)
      : step === 'month'
        ? new Intl.DateTimeFormat('zh-CN', { year: 'numeric', month: 'short' }).format(date)
        : `${date.getFullYear()} 年`
    const cellTasks = tasks.value.filter(task => step === 'day'
      ? task.date === iso(date)
      : step === 'month'
        ? task.date.startsWith(iso(date).slice(0, 7))
        : task.date.startsWith(String(date.getFullYear())))
    return { key: `${step}-${index}`, date, label, tasks: cellTasks }
  })
})
async function load() {
  try {
    plans.value = (await api.plans()).items
    const end = iso(add(anchor.value, Math.min(scale.value, 366)))
    tasks.value = (await api.calendar(iso(anchor.value), end)).items
  } catch (e) {
    error.value = e instanceof Error ? e.message : '读取失败'
  }
}
async function create(){try{await api.createPlan({name:name.value,startDate:startDate.value,endDate:endDate.value,intro:intro.value});name.value='';intro.value='';await load()}catch(e){error.value=e instanceof Error?e.message:'创建失败'}}
async function saveProgress(){if(!selectedPlan.value)return;try{await api.setPlanProgress(selectedPlan.value.id,iso(today),progress.value);editMode.value=false;await load()}catch(e){error.value=e instanceof Error?e.message:'保存失败'}}
function choose(plan:Plan){selectedPlan.value=plan;progress.value=plan.progress}function zoom(delta:number){scaleIndex.value=Math.max(0,Math.min(scales.length-1,scaleIndex.value+delta));load()}function selectCell(date:Date){startDate.value=iso(date);endDate.value=iso(date)}function onWheel(e:WheelEvent){e.preventDefault();zoom(e.deltaY>0?1:-1)}
onMounted(load)
</script>
<template><main class="page future-page" @wheel="onWheel"><p v-if="error" class="error">{{error}}</p><section class="future-split"><aside class="future-list panel"><h2>规划</h2><button v-for="plan in plans" :key="plan.id" class="future-plan" :class="{active:selectedPlan?.id===plan.id}" @click="choose(plan)"><b>{{plan.name}}</b><small>{{plan.endDate}}</small><i class="future-bar time" :style="{width:`${plan.timeProgress}%`}"/><i class="future-bar work" :style="{width:`${plan.progress}%`}"/></button><form @submit.prevent="create"><input v-model.trim="name" placeholder="规划名称" required/><input v-model="startDate" type="date" required/><input v-model="endDate" type="date" required/><textarea v-model="intro" placeholder="简介"/><button class="primary">新建</button></form></aside><section class="future-detail panel"><template v-if="selectedPlan"><header><div><h2>{{selectedPlan.name}}</h2><small>{{selectedPlan.startDate}} — {{selectedPlan.endDate}}</small></div><button class="text-button" @click="editMode=!editMode">{{editMode?'完成':'编辑'}}</button></header><p>{{selectedPlan.intro||'暂无简介'}}</p><div class="dual-progress"><span><i :style="{width:`${selectedPlan.timeProgress}%`}"/></span><span><i :style="{width:`${selectedPlan.progress}%`}"/></span></div><form v-if="editMode" @submit.prevent="saveProgress"><label>计划进度 {{progress}}%<input v-model.number="progress" type="range" min="0" max="100"/></label><button class="primary">保存</button></form></template><div v-else class="empty">选择一项规划</div></section></section><section class="future-calendar panel"><header><h2>待办日历</h2><span>{{scale<=30?`${scale} 天`:scale<=365?'按月':scale<=3650?'按年':'长期视图'}}</span></header><div class="zoom-calendar"><button v-for="cell in cells" :key="cell.key" @click="selectCell(cell.date)"><time>{{cell.label}}</time><i v-for="task in cell.tasks.slice(0,3)" :key="task.id" :class="task.done?'done':''">{{task.title}}</i></button></div></section></main></template>