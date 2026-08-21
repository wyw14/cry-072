<script setup lang="ts">
import { AlarmClock, Building2, ChevronRight, ShieldAlert } from 'lucide-vue-next'
import { formatDate } from '../api/client'
import type { Hazard } from '../domain'
import StatusBadge from './StatusBadge.vue'
defineProps<{ items: Hazard[]; loading: boolean }>()
</script>
<template>
  <div class="queue-board" :aria-busy="loading">
    <article v-for="item in items" :key="item.id" class="queue-row">
      <div class="risk-signal" :data-level="item.risk_level">
        <ShieldAlert :size="17" aria-hidden="true" />
        <strong>{{ item.risk_score }}</strong>
        <span>{{ item.risk_level }}</span>
      </div>
      <div class="queue-summary">
        <div class="queue-title-line">
          <strong>{{ item.title }}</strong>
          <StatusBadge :value="item.state" />
        </div>
        <div class="queue-context">
          <span><Building2 :size="14" aria-hidden="true" />{{ item.site_id }} / {{ item.facility_id }}</span>
          <span>{{ item.id }}</span>
        </div>
        <p v-if="item.matched_rule_id" class="rule-hit">
          自动升级依据：{{ item.matched_rule_id }} v{{ item.matched_rule_version }}
        </p>
      </div>
      <dl class="queue-duty">
        <div><dt>负责人</dt><dd>{{ item.owner_id || '待分派' }}</dd></div>
        <div><dt><AlarmClock :size="14" aria-hidden="true" />截止</dt><dd>{{ formatDate(item.due_at) }}</dd></div>
      </dl>
      <RouterLink class="button icon secondary" :to="`/hazards/${item.id}`" title="查看处置详情">
        <ChevronRight :size="17" />
      </RouterLink>
    </article>
    <div v-if="loading" class="queue-loading" role="status">正在刷新处置队列</div>
    <div v-else-if="items.length === 0" class="empty"><span>当前筛选条件下没有待处置隐患</span></div>
  </div>
</template>
