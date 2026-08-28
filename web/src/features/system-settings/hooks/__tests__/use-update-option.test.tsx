import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor } from '@testing-library/react'
import type { ReactNode } from 'react'
import { beforeEach, describe, expect, test, vi } from 'vitest'

import { updateSystemOption } from '../../api'
import { useUpdateOption } from '../use-update-option'

vi.mock('../../api', () => ({
  updateSystemOption: vi.fn(),
}))

vi.mock('sonner', () => ({
  toast: {
    error: vi.fn(),
    success: vi.fn(),
  },
}))

const updateSystemOptionMock = vi.mocked(updateSystemOption)

function createWrapper(queryClient: QueryClient) {
  return function QueryWrapper(props: { children: ReactNode }) {
    return (
      <QueryClientProvider client={queryClient}>
        {props.children}
      </QueryClientProvider>
    )
  }
}

describe('useUpdateOption', () => {
  beforeEach(() => {
    updateSystemOptionMock.mockReset()
  })

  test('rejects unsuccessful server responses so a multi-option save stops', async () => {
    updateSystemOptionMock.mockResolvedValue({
      success: false,
      message: 'invalid policy',
    })
    const queryClient = new QueryClient({
      defaultOptions: { mutations: { retry: false } },
    })
    const { result } = renderHook(() => useUpdateOption(), {
      wrapper: createWrapper(queryClient),
    })

    await expect(
      result.current.mutateAsync({
        key: 'global.gpt_request_policy.fast_policy',
        value: 'invalid',
      })
    ).rejects.toThrow('invalid policy')
  })

  test('publishes the saved server value into the system options cache', async () => {
    const savedOption = {
      key: 'global.gpt_request_policy.fast_policy',
      value: 'allow',
    }
    updateSystemOptionMock.mockResolvedValue({
      success: true,
      message: '',
      data: savedOption,
    })
    const queryClient = new QueryClient({
      defaultOptions: { mutations: { retry: false } },
    })
    queryClient.setQueryData(['system-options'], {
      success: true,
      message: '',
      data: [{ key: savedOption.key, value: 'disabled' }],
    })
    const { result } = renderHook(() => useUpdateOption(), {
      wrapper: createWrapper(queryClient),
    })

    await result.current.mutateAsync({
      key: savedOption.key,
      value: 'allow',
    })

    await waitFor(() => {
      expect(queryClient.getQueryData(['system-options'])).toMatchObject({
        data: [savedOption],
      })
    })
  })
})
