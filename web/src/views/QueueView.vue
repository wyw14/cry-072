<script setup lang="ts">
import { computed, onMounted, watch } from 'vue'
import { RefreshCw } from 'lucide-vue-next'
import { useRoute, useRouter } from 'vue-router'
import QueueTable from '../components/QueueTable.vue'
import { useHazardStore } from '../stores/hazards'
import type { QueueKind } from '../domain'
const route=useRoute(); const router=useRouter(); const store=useHazardStore()
const queue=computed<QueueKind>(()=>route.params.queue==='normal'?'normal':'focus')
async function load(){await store.load(queue.value)}
watch(queue,load); onMounted(load)
</script>
<template><section class="page">
  <header class="page-header"><div><h1>{{ queue==='focus'?'重点跟进队列':'普通处置队列' }}</h1><p>{{ queue==='focus'?'自动升级与人工确认后的重点隐患，按截止时间优先处置。':'未命中升级条件的日常隐患，按统一普通口径跟进。' }}</p></div><div class="toolbar"><div class="segmented"><button :class="{active:queue==='focus'}" @click="router.push('/queues/focus')">重点</button><button :class="{active:queue==='normal'}" @click="router.push('/queues/normal')">普通</button></div><button class="button icon secondary" title="刷新队列" @click="load"><RefreshCw :size="17" /></button></div></header>
  <div class="metrics"><div class="metric"><small>队列总数</small><strong>{{ store.total }}</strong></div><div class="metric"><small>已超时</small><strong>{{ store.overdue }}</strong></div><div class="metric"><small>待分派</small><strong>{{ store.items.filter(i=>!i.owner_id).length }}</strong></div><div class="metric"><small>待复核</small><strong>{{ store.items.filter(i=>i.state==='pending_review').length }}</strong></div></div>
  <div class="panel"><div class="panel-heading"><h2>处置清单</h2><span class="badge" :class="queue==='focus'?'red':'gray'">{{ queue==='focus'?'重点口径':'普通过口径' }}</span></div><QueueTable :items="store.items" :loading="store.loading" /></div>
</section></template>
