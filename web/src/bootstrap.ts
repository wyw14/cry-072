import type { App as VueApplication } from 'vue'
import { createApp } from 'vue'
import { createPinia } from 'pinia'
import AppShell from './AppShell'
import { router } from './router'

export function buildSafetyConsole(): VueApplication {
  const consoleApp = createApp(AppShell)
  consoleApp.use(createPinia())
  consoleApp.use(router)
  return consoleApp
}

export function mountSafetyConsole(target: string | Element = '#app'): VueApplication {
  const consoleApp = buildSafetyConsole()
  consoleApp.mount(target)
  return consoleApp
}
