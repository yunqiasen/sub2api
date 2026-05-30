import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import AccountTableFilters from './AccountTableFilters.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key
  })
}))

const SelectStub = {
  props: ['modelValue', 'options'],
  emits: ['update:modelValue', 'change'],
  template: '<div data-test="select"></div>'
}

describe('AccountTableFilters', () => {
  it('adds detailed error buckets to the status dropdown', () => {
    const wrapper = mount(AccountTableFilters, {
      props: {
        searchQuery: '',
        filters: {
          platform: '',
          type: '',
          status: '',
          privacy_mode: '',
          group: ''
        },
        groups: []
      },
      global: {
        stubs: {
          Select: SelectStub,
          SearchInput: { template: '<div />' }
        }
      }
    })

    const selects = wrapper.findAllComponents(SelectStub)
    const statusOptions = selects[2].props('options') as Array<{ value: string; label: string }>

    expect(statusOptions).toEqual(expect.arrayContaining([
      { value: 'error', label: 'admin.accounts.status.errorAuthInvalid' },
      { value: 'error_network', label: 'admin.accounts.status.errorNetwork' },
      { value: 'error_cf', label: 'admin.accounts.status.errorCF' },
      { value: 'error_other', label: 'admin.accounts.status.errorOther' }
    ]))
  })
})
