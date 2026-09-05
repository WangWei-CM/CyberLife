import { reactive } from 'vue'

/** 跨页面的待处理跳转：通知中心点击后由目标页面消费并清空。 */
export const ui = reactive({
  pendingPastDate: '',
  pendingPlanId: '',
  pendingSettingsTab: '',
})
