<script setup lang="ts">
import { ref } from 'vue'
import { Download, Search } from 'lucide-vue-next'
import { formatDate, request, RequestError } from '../api/client'
import type { AuditEvent, Page } from '../domain'
const hazardID=ref('');const events=ref<AuditEvent[]>([]);const error=ref('')
async function search(){error.value='';try{const page=await request<Page<AuditEvent>>(`/hazards/${hazardID.value}/audit?page=1&page_size=100`);events.value=page.items}catch(e){error.value=e instanceof RequestError?e.detail.message:'读取历史失败'}}
function download(){window.location.href=`/api/v1/hazards/${encodeURIComponent(hazardID.value)}/audit.csv`}
const labels:Record<string,string>={'hazard.reported':'隐患上报','hazard.assigned':'负责人分派','hazard.state_changed':'状态变更','hazard.reinspected':'复检完成','hazard.downgraded':'人工降级','notification.attempted':'提醒尝试'}
</script>
<template><section class="page"><header class="page-header"><div><h1>审计历史</h1><p>按时间顺序查看状态、规则版本和操作理由，敏感字段在公开视图中脱敏。</p></div><div class="toolbar"><input v-model="hazardID" placeholder="输入隐患标识"/><button class="button" @click="search"><Search :size="16"/>查询</button><button class="button secondary" :disabled="!hazardID" @click="download"><Download :size="16"/>导出</button></div></header><div v-if="error" class="error-banner">{{error}}</div><div class="panel"><div class="panel-heading"><h2>时间线</h2><span class="badge gray">{{events.length}} 条</span></div><div class="panel-body"><div v-if="events.length" class="timeline"><article v-for="event in events" :key="event.id" class="timeline-item"><time class="timeline-time">{{formatDate(event.occurred_at)}}</time><span class="timeline-track"><i class="timeline-dot"></i></span><div class="timeline-copy"><strong>{{labels[event.event_type]??event.event_type}}</strong><p>{{event.actor_id}} · {{event.reason||'无附加说明'}}<template v-if="event.rule_id"> · {{event.rule_id}} v{{event.rule_version}}</template></p></div></article></div><div v-else class="empty">输入隐患标识后查询完整审计时间线</div></div></div></section></template>
