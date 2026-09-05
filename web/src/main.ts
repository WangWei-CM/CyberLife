import { createApp } from 'vue'
import App from './App.vue'
import { vGlow, vStagger } from './lib/motion'
import './styles/tokens.css'
import './styles/base.css'
import './styles/motion.css'
import './styles/glow.css'
import './styles/shell.css'
import './styles/components.css'
import './styles/login.css'

createApp(App).directive('glow', vGlow).directive('stagger', vStagger).mount('#app')
