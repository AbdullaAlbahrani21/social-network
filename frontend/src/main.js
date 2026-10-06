import { createApp } from 'vue'
import { createPinia } from 'pinia'
// Tokens first, then the global base that reads them, and both before App so every component's own styles load after and win.
import './styles/tokens.css'
import './styles/global.css'
import App from './App.vue'
import router from './router'

createApp(App).use(createPinia()).use(router).mount('#app')
