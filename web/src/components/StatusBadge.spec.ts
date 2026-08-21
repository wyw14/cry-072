import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import StatusBadge from './StatusBadge.vue'

describe('StatusBadge', () => {
  it('renders business labels with risk tone', () => {
    const wrapper = mount(StatusBadge, { props: { value: 'critical' } })
    expect(wrapper.text()).toBe('紧急')
    expect(wrapper.classes()).toContain('red')
  })

  it('keeps unknown backend values inspectable', () => {
    const wrapper = mount(StatusBadge, { props: { value: 'paused' } })
    expect(wrapper.text()).toBe('paused')
    expect(wrapper.classes()).toContain('gray')
  })
})
