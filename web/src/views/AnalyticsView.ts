import { computed, defineComponent, h, onMounted, reactive, ref } from 'vue'
import { RefreshCw, Scale, TimerOff } from 'lucide-vue-next'
import { request } from '../api/client'
import type { AnalyticsSnapshot, QueueMetrics } from '../domain'

function percent(value: number) {
  return `${Math.round(value * 100)}%`
}

function overdueRate(metric: QueueMetrics) {
  return metric.open_count === 0 ? 0 : metric.overdue_count / metric.open_count
}

function metricDefinition(label: string, value: string | number) {
  return h('div', [h('dt', label), h('dd', String(value))])
}

function queueColumn(label: string, metric: QueueMetrics, focus = false) {
  return h('section', { class: ['comparison-column', focus ? 'focus' : 'normal'] }, [
    h('header', [h('span', label), h('small', `${metric.resolved_total} 项已解除`)]),
    h('strong', percent(metric.on_time_rate)),
    h('dl', [
      metricDefinition('平均耗时', `${Math.round(metric.average_handle_minutes)} 分钟`),
      metricDefinition('当前未关闭', metric.open_count),
      metricDefinition('当前超时', metric.overdue_count),
    ]),
  ])
}

export default defineComponent({
  name: 'QueueAnalyticsView',
  setup() {
    const today = new Date()
    const thirtyDaysAgo = new Date(today.getTime() - 30 * 24 * 60 * 60 * 1000)
    const range = reactive({
      from: thirtyDaysAgo.toISOString().slice(0, 10),
      to: today.toISOString().slice(0, 10),
    })
    const snapshot = ref<AnalyticsSnapshot | null>(null)

    async function load() {
      const from = new Date(`${range.from}T00:00:00Z`).toISOString()
      const to = new Date(`${range.to}T23:59:59Z`).toISOString()
      snapshot.value = await request(`/analytics/queue-comparison?from=${from}&to=${to}`)
    }

    const comparison = computed(() => {
      if (!snapshot.value) return null
      const { normal, focus } = snapshot.value
      return {
        onTimeGap: focus.on_time_rate - normal.on_time_rate,
        handleTimeGap: focus.average_handle_minutes - normal.average_handle_minutes,
        overdueGap: overdueRate(focus) - overdueRate(normal),
      }
    })

    function dateInput(kind: 'from' | 'to', label: string) {
      return h('input', {
        type: 'date',
        value: range[kind],
        'aria-label': label,
        onInput: (event: Event) => { range[kind] = (event.target as HTMLInputElement).value },
      })
    }

    function header() {
      return h('header', { class: 'page-header' }, [
        h('div', [
          h('h1', '处置口径比较'),
          h('p', '普通与重点队列使用各自分母计算时效，重点风险不会被普通任务量稀释。'),
        ]),
        h('div', { class: 'toolbar' }, [
          dateInput('from', '统计开始日期'),
          h('span', '至'),
          dateInput('to', '统计结束日期'),
          h('button', { class: 'button icon secondary', title: '刷新统计', onClick: load }, [h(RefreshCw, { size: 17 })]),
        ]),
      ])
    }

    function deltaColumn() {
      const delta = comparison.value!
      return h('section', { class: 'comparison-delta' }, [
        h(Scale, { size: 22, 'aria-hidden': 'true' }),
        h('span', '重点队列相对差值'),
        h('strong', { class: { adverse: delta.onTimeGap < 0 } },
          `${delta.onTimeGap >= 0 ? '+' : ''}${percent(delta.onTimeGap)} 按时率`),
        h('small', `${Math.round(delta.handleTimeGap)} 分钟耗时差`),
        h('small', `${delta.overdueGap >= 0 ? '+' : ''}${percent(delta.overdueGap)} 超时占比差`),
      ])
    }

    function anomalyLedger(data: AnalyticsSnapshot) {
      const content = data.anomalies.length === 0
        ? h('div', { class: 'empty' }, '当前区间没有触发异常阈值')
        : h('ol', data.anomalies.map((item) => h('li', { key: `${item.code}-${item.site_id}` }, [
            h('div', [h('strong', item.description), h('small', `${item.code} · ${item.site_id || '全部场地'}`)]),
            h('div', { class: 'deviation' }, [
              h('span', `观测 ${item.observed}`),
              h('strong', `超出 ${Math.max(0, item.observed - item.threshold)}`),
              h('span', `阈值 ${item.threshold}`),
            ]),
          ])))
      return h('section', { class: 'anomaly-ledger' }, [
        h('header', [
          h('div', [h('h2', '异常阈值观测'), h('p', `${data.from.slice(0, 10)} 至 ${data.to.slice(0, 10)}`)]),
          h(TimerOff, { size: 19, 'aria-hidden': 'true' }),
        ]),
        content,
      ])
    }

    onMounted(load)
    return () => {
      const data = snapshot.value
      const body = data && comparison.value
        ? [
            h('div', { class: 'queue-comparison', 'aria-label': '普通与重点队列口径对照' }, [
              queueColumn('普通处置', data.normal),
              deltaColumn(),
              queueColumn('重点处置', data.focus, true),
            ]),
            anomalyLedger(data),
          ]
        : []
      return h('section', { class: 'page analytics-page' }, [header(), ...body])
    }
  },
})
