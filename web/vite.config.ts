import { fileURLToPath } from 'node:url'
import { defineConfig, loadEnv } from 'vite'
import vue from '@vitejs/plugin-vue'

const root = fileURLToPath(new URL('.', import.meta.url))

export default defineConfig(({ mode }) => {
  const env = { ...loadEnv(mode, root, ''), ...process.env }
  const apiOrigin = env.CYBERLIFE_API_ORIGIN || 'http://127.0.0.1:8080'
  return {
    root,
    plugins: [vue()],
    server: { port: 5173, proxy: { '/api': apiOrigin, '/health': apiOrigin } },
  }
})
