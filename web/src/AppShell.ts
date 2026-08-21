import { defineComponent, h, type Component } from 'vue'
import { RouterLink, RouterView, useRoute } from 'vue-router'
import { Activity, AlertTriangle, BarChart3, ClipboardCheck, FileClock, ListChecks, Settings2, ShieldCheck } from 'lucide-vue-next'

interface NavigationItem {
  to: string
  label: string
  icon: Component
}

const navigation: NavigationItem[] = [
  { to: '/queues/focus', label: '重点跟进', icon: AlertTriangle },
  { to: '/queues/normal', label: '普通队列', icon: ListChecks },
  { to: '/report', label: '巡检上报', icon: ClipboardCheck },
  { to: '/reinspection', label: '复检确认', icon: ShieldCheck },
  { to: '/rules', label: '升级规则', icon: Settings2 },
  { to: '/analytics', label: '口径比较', icon: BarChart3 },
  { to: '/history', label: '审计历史', icon: FileClock },
]

function brand() {
  return h('div', { class: 'brand' }, [
    h('span', { class: 'brand-mark' }, [h(Activity, { size: 20 })]),
    h('span', [h('strong', '场安台'), h('small', '隐患分级处置')]),
  ])
}

function operatorStatus() {
  return h('div', { class: 'operator' }, [
    h('span', { class: 'operator-dot', 'aria-hidden': 'true' }),
    h('span', [h('strong', '演示值守组'), h('small', '离线适配器已连接')]),
  ])
}

export default defineComponent({
  name: 'SafetyConsoleShell',
  setup() {
    const route = useRoute()
    return () => h('div', { class: 'shell' }, [
      h('aside', { class: 'sidebar' }, [
        brand(),
        h('nav', { 'aria-label': '主导航' }, navigation.map((item) =>
          h(RouterLink, {
            to: item.to,
            class: { active: route.path.startsWith(item.to) },
          }, {
            default: () => [h(item.icon, { size: 18, 'aria-hidden': 'true' }), h('span', item.label)],
          }),
        )),
        operatorStatus(),
      ]),
      h('main', { class: 'main' }, [h(RouterView)]),
    ])
  },
})
