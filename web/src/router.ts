import { createRouter, createWebHistory } from 'vue-router'
import QueueView from './views/QueueView.vue'
import ReportView from './views/ReportView.vue'
import DetailView from './views/DetailView.vue'
import RulesView from './views/RulesView.vue'
import ReinspectionView from './views/ReinspectionView.vue'
import AnalyticsView from './views/AnalyticsView'
import HistoryView from './views/HistoryView.vue'

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', redirect: '/queues/focus' },
    { path: '/report', component: ReportView },
    { path: '/queues/:queue', component: QueueView },
    { path: '/hazards/:id', component: DetailView },
    { path: '/rules', component: RulesView },
    { path: '/reinspection', component: ReinspectionView },
    { path: '/analytics', component: AnalyticsView },
    { path: '/history', component: HistoryView },
  ],
})
