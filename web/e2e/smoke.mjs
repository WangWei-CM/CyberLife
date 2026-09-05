// Frontend smoke test: logs in with a real key, walks every screen, exercises the main interactions
// and fails on console errors or missing landmarks. Screenshots land in web/e2e/shots/.
//
//   CYBERLIFE_E2E_KEY=cl_xxx CYBERLIFE_E2E_BASE=http://127.0.0.1:5173 node e2e/smoke.mjs
//   (optional) CYBERLIFE_E2E_READER_KEY=cl_yyy  验证阅读者视角
import { launch } from './cdp.mjs'

const base = process.env.CYBERLIFE_E2E_BASE || 'http://127.0.0.1:5173'
const key = process.env.CYBERLIFE_E2E_KEY
const readerKey = process.env.CYBERLIFE_E2E_READER_KEY
if (!key) { console.error('缺少 CYBERLIFE_E2E_KEY（书写者主密钥）'); process.exit(2) }

const failures = []
const check = (condition, message) => { if (!condition) failures.push(message) }
const b = await launch()
try {
  await b.goto(`${base}/`, 1000)
  await b.logoutApi().catch(() => {})
  await b.goto(`${base}/`, 1000)
  check(await b.exists('.login-logo'), '登录页缺少 CyberLife logo')
  check(await b.exists('.login-canvas'), '登录页缺少轨道场景画布')
  check(await b.exists('.login-key-input'), '登录页缺少密钥输入框')
  await b.shot('login')

  // 登录序列
  await b.type('.login-key-input', key)
  await b.click('.login-submit')
  await b.sleep(1400)
  check((await b.text('.login-stage-label')).includes('CYBERLIFE NODE') || (await b.text('.login-stage-label')).includes('LIFE OPERATING SYSTEM'), '登录后没有进入节点阶段')
  await b.sleep(3000)
  check(await b.exists('.app-shell'), '节点转场完成后没有进入面板')

  // 现在页
  await b.sleep(1200)
  check(await b.exists('.now-clock .clock-date'), '现在页缺少时钟')
  check(await b.exists('.md-editor'), '现在页缺少日记编辑器')
  check(await b.exists('.fc'), '现在页缺少日历')
  await b.shot('now', { fullPage: true })
  await b.click('.notice-button')
  await b.sleep(400)
  check(await b.exists('.notice-popover'), '通知弹层未打开')
  await b.key('Escape')
  await b.click('.music-more')
  await b.sleep(400)
  check(await b.exists('.music-popover'), '音乐弹层未打开')
  await b.key('Escape')

  // 过去页：时间轴缩放前后可见范围应变化
  await b.click('.screen-nav button:nth-child(1)')
  await b.sleep(1500)
  check(await b.exists('.life-axis'), '过去页缺少时间轴')
  const before = await b.text('.axis-range')
  await b.wheel('.life-axis', 120, 8, 30)
  await b.sleep(1400)
  const after = await b.text('.axis-range')
  check(before !== after, `时间轴滚轮缩放无效（${before} → ${after}）`)
  await b.shot('past', { fullPage: true })

  // 未来页：连续缩放日历
  await b.click('.screen-nav button:nth-child(3)')
  await b.sleep(1500)
  check(await b.exists('.zoom-calendar'), '未来页缺少缩放日历')
  const scaleBefore = await b.text('.zoom-scale')
  await b.wheel('.zoom-calendar', 120, 10, 25)
  await b.sleep(1400)
  const scaleAfter = await b.text('.zoom-scale')
  check(scaleBefore !== scaleAfter, `未来页日历缩放无效（${scaleBefore} → ${scaleAfter}）`)
  await b.shot('future', { fullPage: true })

  // 设置页 tab
  await b.click('.icon-button[aria-label="设置"]')
  await b.sleep(800)
  check(await b.exists('.settings-page .segmented'), '设置页缺少 tab')
  await b.shot('settings', { fullPage: true })

  // 移动端
  await b.viewport(390, 844)
  await b.click('.screen-nav button:nth-child(2)')
  await b.sleep(1200)
  await b.shot('mobile-now', { fullPage: true })
  await b.viewport(1440, 900)

  if (readerKey) {
    await b.logoutApi()
    await b.loginWith(readerKey)
    await b.goto(`${base}/`, 2000)
    check(await b.exists('.reader-page'), '阅读者未进入阅读视图')
    await b.shot('reader-now', { fullPage: true })
    await b.click('.screen-nav button:nth-child(3)')
    await b.sleep(1500)
    await b.shot('reader-future', { fullPage: true })
  }
  await b.logoutApi()
} catch (error) {
  failures.push(`执行异常：${error.message}`)
  await b.shot('failure').catch(() => {})
} finally {
  b.close()
}
if (b.consoleErrors.length) failures.push(`浏览器控制台错误：${b.consoleErrors.slice(0, 5).join(' | ')}`)
if (failures.length) { console.error('冒烟测试未通过：\n- ' + failures.join('\n- ')); process.exit(1) }
console.log('冒烟测试通过，截图见 web/e2e/shots/')
