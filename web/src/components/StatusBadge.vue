<script setup lang="ts">
import { computed } from 'vue'
const props = defineProps<{ value: string }>()
const labels: Record<string, string> = { critical:'紧急', high:'高风险', medium:'中风险', low:'低风险', pending_confirmation:'待确认', handling:'处理中', pending_review:'待复核', resolved:'已解除', closed:'已关闭', focus:'重点', normal:'普通' }
const tone = computed(() => ['critical','high','focus'].includes(props.value) ? 'red' : ['medium','pending_review'].includes(props.value) ? 'amber' : ['resolved','closed','low'].includes(props.value) ? 'green' : props.value === 'handling' ? 'blue' : 'gray')
</script>
<template><span class="badge" :class="tone">{{ labels[value] ?? value }}</span></template>
