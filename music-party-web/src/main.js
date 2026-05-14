import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { watch } from 'vue'
import './style.css'
import App from './App.vue'
import { useUiStore } from './stores/ui'
import { i18n } from './i18n/index'

const pinia = createPinia()
const app = createApp(App)

app.use(pinia)
app.use(i18n)
const uiStore = useUiStore(pinia)
watch(() => uiStore.locale, (locale) => {
  i18n.global.locale.value = locale
}, { immediate: true })
document.documentElement.classList.toggle('dark', uiStore.isDarkMode)
document.documentElement.classList.toggle('light', !uiStore.isDarkMode)
app.mount('#app')
