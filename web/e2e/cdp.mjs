// Minimal Chrome DevTools Protocol driver for headless screenshots and interaction.
// No dependencies: it launches a local Chrome/Edge with --remote-debugging-port and talks CDP over WebSocket.
import { spawn } from 'node:child_process'
import { existsSync, mkdirSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const here = fileURLToPath(new URL('.', import.meta.url))
const CANDIDATES = [
  process.env.CHROME_PATH,
  'C:/Program Files/Google/Chrome/Application/chrome.exe',
  'C:/Program Files (x86)/Google/Chrome/Application/chrome.exe',
  'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe',
  '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',
  '/usr/bin/google-chrome',
  '/usr/bin/chromium',
].filter(Boolean)

export function findBrowser() {
  const found = CANDIDATES.find(candidate => existsSync(candidate))
  if (!found) throw new Error('未找到 Chrome/Edge，可用环境变量 CHROME_PATH 指定可执行文件')
  return found
}

export async function launch({ port = 9333, width = 1440, height = 900 } = {}) {
  const profile = join(tmpdir(), 'cyberlife-e2e-profile')
  mkdirSync(profile, { recursive: true })
  const chrome = spawn(findBrowser(), ['--headless=new', `--remote-debugging-port=${port}`, `--window-size=${width},${height}`, `--user-data-dir=${profile}`, '--no-first-run', '--no-default-browser-check', '--hide-scrollbars', '--force-device-scale-factor=1', '--disable-gpu', '--autoplay-policy=no-user-gesture-required', 'about:blank'], { stdio: 'ignore' })
  let targets
  for (let i = 0; i < 60; i++) {
    try { targets = await (await fetch(`http://127.0.0.1:${port}/json`)).json(); if (targets.some(t => t.type === 'page')) break } catch { /* not ready */ }
    await new Promise(r => setTimeout(r, 250))
  }
  if (!targets) { chrome.kill(); throw new Error('浏览器未能启动') }
  const page = targets.find(t => t.type === 'page')
  const ws = new WebSocket(page.webSocketDebuggerUrl)
  await new Promise((resolve, reject) => { ws.onopen = resolve; ws.onerror = reject })
  let id = 0
  const pending = new Map()
  const listeners = new Map()
  ws.onmessage = event => {
    const message = JSON.parse(event.data)
    if (message.id && pending.has(message.id)) { const { resolve, reject } = pending.get(message.id); pending.delete(message.id); message.error ? reject(new Error(message.error.message)) : resolve(message.result) }
    else if (message.method) for (const fn of listeners.get(message.method) ?? []) fn(message.params)
  }
  const send = (method, params = {}) => new Promise((resolve, reject) => { const messageId = ++id; pending.set(messageId, { resolve, reject }); ws.send(JSON.stringify({ id: messageId, method, params })) })
  const on = (method, fn) => { if (!listeners.has(method)) listeners.set(method, []); listeners.get(method).push(fn) }
  const once = method => new Promise(resolve => { const fn = params => { listeners.set(method, (listeners.get(method) ?? []).filter(f => f !== fn)); resolve(params) }; on(method, fn) })
  await send('Page.enable'); await send('Runtime.enable'); await send('Network.enable')
  await send('Emulation.setDeviceMetricsOverride', { width, height, deviceScaleFactor: 1, mobile: false })
  const consoleErrors = []
  on('Runtime.exceptionThrown', p => consoleErrors.push(p.exceptionDetails?.exception?.description || p.exceptionDetails?.text))
  on('Runtime.consoleAPICalled', p => { if (p.type === 'error') consoleErrors.push(`[${p.type}] ${p.args.map(a => a.value ?? a.description).join(' ')}`) })

  const sleep = ms => new Promise(r => setTimeout(r, ms))
  const evaluate = async expression => { const r = await send('Runtime.evaluate', { expression, awaitPromise: true, returnByValue: true }); if (r.exceptionDetails) throw new Error(r.exceptionDetails.exception?.description || r.exceptionDetails.text); return r.result.value }
  const rect = async selector => evaluate(`(() => { const el = document.querySelector(${JSON.stringify(selector)}); if (!el) return null; el.scrollIntoView({ block: 'nearest' }); const r = el.getBoundingClientRect(); return { x: r.left + r.width / 2, y: r.top + r.height / 2, w: r.width, h: r.height } })()`)
  return {
    send, on, once, sleep, evaluate, consoleErrors,
    async goto(url, wait = 800) { const loaded = once('Page.loadEventFired'); await send('Page.navigate', { url }); await loaded; await sleep(wait) },
    async reload(wait = 800) { const loaded = once('Page.loadEventFired'); await send('Page.reload'); await loaded; await sleep(wait) },
    async shot(name, { fullPage = false, clip } = {}) {
      const params = { format: 'png', captureBeyondViewport: fullPage }
      if (fullPage) { const m = await send('Page.getLayoutMetrics'); params.clip = { x: 0, y: 0, width: m.cssContentSize.width, height: Math.min(m.cssContentSize.height, 4000), scale: 1 } }
      if (clip) params.clip = { ...clip, scale: 1 }
      const r = await send('Page.captureScreenshot', params)
      const file = join(here, 'shots', `${name}.png`)
      mkdirSync(dirname(file), { recursive: true })
      writeFileSync(file, Buffer.from(r.data, 'base64'))
      return file
    },
    async exists(selector) { return evaluate(`!!document.querySelector(${JSON.stringify(selector)})`) },
    async text(selector) { return evaluate(`document.querySelector(${JSON.stringify(selector)})?.textContent ?? ''`) },
    async hover(selector) { const r = await rect(selector); if (!r) throw new Error(`no element ${selector}`); await send('Input.dispatchMouseEvent', { type: 'mouseMoved', x: r.x, y: r.y }); return r },
    async click(selector) { const r = await rect(selector); if (!r) throw new Error(`no element ${selector}`); await send('Input.dispatchMouseEvent', { type: 'mouseMoved', x: r.x, y: r.y }); await send('Input.dispatchMouseEvent', { type: 'mousePressed', x: r.x, y: r.y, button: 'left', clickCount: 1 }); await send('Input.dispatchMouseEvent', { type: 'mouseReleased', x: r.x, y: r.y, button: 'left', clickCount: 1 }); return r },
    async wheel(selector, deltaY, times = 1, gap = 40) { const r = await rect(selector); if (!r) throw new Error(`no element ${selector}`); for (let i = 0; i < times; i++) { await send('Input.dispatchMouseEvent', { type: 'mouseWheel', x: r.x, y: r.y, deltaX: 0, deltaY }); await sleep(gap) } },
    async type(selector, text) { await evaluate(`document.querySelector(${JSON.stringify(selector)}).focus()`); await send('Input.insertText', { text }) },
    async key(key, code) { const vk = key === 'Enter' ? 13 : key === 'Escape' ? 27 : 0; await send('Input.dispatchKeyEvent', { type: 'keyDown', key, code: code || key, windowsVirtualKeyCode: vk }); await send('Input.dispatchKeyEvent', { type: 'keyUp', key, code: code || key, windowsVirtualKeyCode: vk }) },
    async media({ reducedMotion = false, dark = true } = {}) { await send('Emulation.setEmulatedMedia', { features: [{ name: 'prefers-reduced-motion', value: reducedMotion ? 'reduce' : 'no-preference' }, { name: 'prefers-color-scheme', value: dark ? 'dark' : 'light' }] }) },
    async viewport(width, height) { await send('Emulation.setDeviceMetricsOverride', { width, height, deviceScaleFactor: 1, mobile: false }) },
    async loginWith(key) { return evaluate(`fetch('/api/v1/auth/key-login', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ key: ${JSON.stringify(key)} }) }).then(r => r.status)`) },
    async logoutApi() { return evaluate(`fetch('/api/v1/auth/logout', { method: 'POST' }).then(r => r.status)`) },
    close() { try { ws.close() } catch { /* ignore */ } chrome.kill() },
  }
}
