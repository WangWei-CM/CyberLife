<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { api, type ScheduleClass, type ScheduleClassInput, type ScheduleCSVMapping, type ScheduleImportPreview } from '../api/client'
import AppIcon from './AppIcon.vue'

const classes = ref<ScheduleClass[]>([]); const error = ref(''); const editing = ref<ScheduleClass | null>(null); const busy = ref(false); const file = ref<File | null>(null); const preview = ref<ScheduleImportPreview | null>(null); const mode = ref<'replace'|'merge'>('replace')
const draft = ref<ScheduleClassInput>({ title: '', weekday: 1, sessionDate: '', startTime: '08:00', endTime: '09:35', effectiveStartDate: '', effectiveEndDate: '', location: '', note: '' })
const emptyMapping = (): ScheduleCSVMapping => ({ title:'',weekday:'',sessionDate:'',startTime:'',endTime:'',location:'',note:'',effectiveStartDate:'',effectiveEndDate:'' })
const mapping = ref<ScheduleCSVMapping>(emptyMapping())
async function load(){try{classes.value=(await api.scheduleClasses()).items}catch(e){error.value=e instanceof Error?e.message:'读取课表失败'}}
function reset(){editing.value=null;draft.value={title:'',weekday:1,sessionDate:'',startTime:'08:00',endTime:'09:35',effectiveStartDate:'',effectiveEndDate:'',location:'',note:''}}
function edit(item: ScheduleClass){editing.value=item;draft.value={title:item.title,weekday:item.weekday,sessionDate:item.sessionDate||'',startTime:item.startTime,endTime:item.endTime,effectiveStartDate:item.effectiveStartDate||'',effectiveEndDate:item.effectiveEndDate||'',location:item.location,note:item.note}}
async function save(){if(!draft.value.title.trim()||busy.value)return;busy.value=true;const payload={...draft.value,weekday:draft.value.sessionDate?undefined:draft.value.weekday,sessionDate:draft.value.sessionDate||''};try{if(editing.value)await api.updateScheduleClass(editing.value.id,payload);else await api.createScheduleClass(payload);reset();await load()}catch(e){error.value=e instanceof Error?e.message:'保存失败'}finally{busy.value=false}}
async function remove(item: ScheduleClass){if(!window.confirm(`删除课程“${item.title}”吗？`))return;try{await api.deleteScheduleClass(item.id);await load()}catch(e){error.value=e instanceof Error?e.message:'删除失败'}}
function choose(event: Event){file.value=(event.target as HTMLInputElement).files?.[0]??null;preview.value=null}
async function inspect(){if(!file.value)return;try{preview.value=await api.previewScheduleImport(file.value);mapping.value={...emptyMapping(),...preview.value.detectedMapping}}catch(e){error.value=e instanceof Error?e.message:'导入预览失败'}}
async function apply(){if(!file.value||!preview.value)return;busy.value=true;try{await api.applyScheduleImport(file.value,mode.value,mapping.value);file.value=null;preview.value=null;await load()}catch(e){error.value=e instanceof Error?e.message:'导入失败'}finally{busy.value=false}}
function template(){const blob=new Blob(['title,weekday,startTime,endTime,location,note\n高等数学,1,08:00,09:35,教学楼 A201,'],{type:'text/csv;charset=utf-8'});const a=document.createElement('a');a.href=URL.createObjectURL(blob);a.download='课表模板.csv';a.click();URL.revokeObjectURL(a.href)}
onMounted(load)
</script>

<template>
  <section class="settings-panel schedule-settings" v-stagger>
    <Transition name="fade"><p v-if="error" class="error page-error">{{ error }}<button class="text-button" @click="error=''">关闭</button></p></Transition>
    <article class="card"><header class="card-title"><span>我的课表</span><small>每周循环与单次例外</small></header>
      <form class="schedule-editor" @submit.prevent="save">
        <div class="form-row"><input v-model="draft.title" placeholder="课程名称" maxlength="80" required /><select v-model="draft.weekday" :disabled="!!draft.sessionDate"><option :value="1">周一</option><option :value="2">周二</option><option :value="3">周三</option><option :value="4">周四</option><option :value="5">周五</option><option :value="6">周六</option><option :value="7">周日</option></select><input v-model="draft.sessionDate" type="date" aria-label="单次日期" /></div>
        <div class="form-row"><label class="field"><span>开始</span><input v-model="draft.startTime" type="time" required /></label><label class="field"><span>结束</span><input v-model="draft.endTime" type="time" required /></label><input v-model="draft.location" placeholder="地点（可选）" /></div>
        <div class="form-row"><input v-model="draft.effectiveStartDate" type="date" aria-label="生效开始日期" /><input v-model="draft.effectiveEndDate" type="date" aria-label="生效结束日期" /><input v-model="draft.note" placeholder="备注（可选）" /></div>
        <div class="form-row"><button class="primary" type="submit" :disabled="busy">{{ editing ? '保存修改' : '添加课程' }}</button><button v-if="editing" class="text-button" type="button" @click="reset">取消</button></div>
      </form>
      <ul v-if="classes.length" class="schedule-list"><li v-for="item in classes" :key="item.id"><span><b>{{ item.title }}</b><small>{{ item.sessionDate || `周${['','一','二','三','四','五','六','日'][item.weekday || 1]}` }} · {{ item.startTime }}–{{ item.endTime }}<template v-if="item.location"> · {{ item.location }}</template></small></span><span class="schedule-list-actions"><button class="text-button" @click="edit(item)">编辑</button><button class="text-button danger" @click="remove(item)">删除</button></span></li></ul><p v-else class="faint empty-settings">还没有课程</p>
    </article>
    <article class="card"><header class="card-title"><span>导入课表</span><small>ICS / CSV</small></header><div class="schedule-import-actions"><label class="btn"><AppIcon name="upload" :size="16" />选择文件<input type="file" accept=".ics,.ical,.csv,text/calendar,text/csv" @change="choose" /></label><button class="text-button" type="button" @click="template">下载 CSV 模板</button><button class="primary" type="button" :disabled="!file||busy" @click="inspect">预览</button></div><small v-if="file" class="faint">{{ file.name }}</small>
      <div v-if="preview" class="import-preview"><p>识别 {{ preview.items.length }} 节课程，{{ preview.warnings.length }} 条提醒</p><div v-if="preview.format==='csv'" class="mapping-grid"><label v-for="key in ['title','weekday','sessionDate','startTime','endTime','location','note']" :key="key"><span>{{ key }}</span><select v-model="mapping[key as keyof ScheduleCSVMapping]"><option value="">未映射</option><option v-for="column in preview.columns" :key="column" :value="column">{{ column }}</option></select></label></div><ul v-if="preview.warnings.length" class="import-warnings"><li v-for="warning in preview.warnings" :key="warning">{{ warning }}</li></ul><div class="form-row"><label class="radio-field"><input v-model="mode" value="replace" type="radio" />整体替换</label><label class="radio-field"><input v-model="mode" value="merge" type="radio" />合并去重</label><button class="primary" type="button" :disabled="busy" @click="apply">确认导入</button></div></div>
    </article>
  </section>
</template>
