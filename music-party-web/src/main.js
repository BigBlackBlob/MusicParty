import { createApp } from 'vue'
import { createPinia } from 'pinia'
import './style.css'
import App from './App.vue'
import { useUiStore } from './stores/ui'

const pinia = createPinia()
const app = createApp(App)

app.use(pinia)
const uiStore = useUiStore(pinia)
document.documentElement.classList.toggle('dark', uiStore.isDarkMode)
document.documentElement.classList.toggle('light', !uiStore.isDarkMode)
app.mount('#app')
