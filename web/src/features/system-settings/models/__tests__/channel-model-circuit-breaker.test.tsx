/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'

import { getChannelModelEvents } from '../../api'
import { ChannelModelCircuitBreakerSection } from '../channel-model-circuit-breaker-section'

vi.mock('../../api', () => ({
  getChannelModelEvents: vi.fn().mockResolvedValue({
    success: true,
    data: { page: 1, page_size: 20, total: 0, items: [] },
  }),
}))

function renderSection(circuitBreakerEnabled: boolean, recoveryEnabled = true) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  render(
    <QueryClientProvider client={queryClient}>
      <ChannelModelCircuitBreakerSection
        defaultValues={{
          'monitor_setting.channel_model_circuit_breaker_enabled':
            circuitBreakerEnabled,
          'monitor_setting.channel_model_excluded_channel_ids': '',
          'monitor_setting.channel_error_window_minutes': 5,
          'monitor_setting.channel_error_threshold': 20,
          'monitor_setting.channel_error_consecutive_threshold': 10,
          'monitor_setting.channel_error_status_codes': '429,503',
          'monitor_setting.channel_model_recovery_enabled': recoveryEnabled,
          'monitor_setting.channel_model_recovery_success_threshold': 2,
          'monitor_setting.channel_model_recovery_delay_minutes': 5,
          'monitor_setting.channel_model_recovery_interval_minutes': 1,
          'monitor_setting.channel_model_recovery_concurrency': 4,
          'monitor_setting.channel_model_event_retention_days': 30,
          'monitor_setting.channel_model_wecom_bot_enabled': false,
          'monitor_setting.channel_model_wecom_bot_url':
            'https://alertplus.intsig.net/api/alarm/receive/',
          'monitor_setting.channel_model_wecom_bot_webhook_key_configured': false,
          'monitor_setting.channel_model_wecom_bot_contact': '',
        }}
      />
    </QueryClientProvider>
  )
  return queryClient
}

describe('ChannelModelCircuitBreakerSection', () => {
  it('shows configuration first and only loads logs after switching tabs', async () => {
    const user = userEvent.setup()
    const queryClient = renderSection(true)

    expect(
      screen.getByRole('tab', { name: 'Parameter Configuration' })
    ).toHaveAttribute('data-active')
    expect(screen.queryByText('Circuit breaker and recovery logs')).not.toBeInTheDocument()
    expect(getChannelModelEvents).not.toHaveBeenCalled()

    await user.click(screen.getByRole('tab', { name: 'Log Query' }))

    expect(screen.getByText('Circuit breaker and recovery logs')).toBeInTheDocument()
    expect(getChannelModelEvents).toHaveBeenCalledTimes(1)

    queryClient.clear()
  })

  it('shows recovery defaults and disables dependent fields when circuit breaker is off', () => {
    const queryClient = renderSection(false)

    expect(
      screen.getByRole('switch', {
        name: 'Enable channel-model circuit breaker',
      })
    ).not.toBeChecked()
    expect(
      screen.getByRole('spinbutton', { name: 'Failure window (minutes)' })
    ).toBeDisabled()
    expect(
      screen.getByRole('textbox', { name: 'Excluded channel IDs' })
    ).toBeDisabled()
    expect(
      screen.getByRole('spinbutton', {
        name: 'Consecutive successes to recover',
      })
    ).toHaveValue(2)
    expect(
      screen.getByRole('spinbutton', {
        name: 'Wait before first probe (minutes)',
      })
    ).toHaveValue(5)
    expect(
      screen.getByRole('spinbutton', {
        name: 'Recovery probe interval (minutes)',
      })
    ).toHaveValue(1)
    expect(
      screen.getByRole('spinbutton', { name: 'Recovery probe concurrency' })
    ).toHaveValue(4)

    queryClient.clear()
  })

  it('enables circuit breaker and recovery inputs through their switches', async () => {
    const user = userEvent.setup()
    const queryClient = renderSection(false)
    const circuitBreakerSwitch = screen.getByRole('switch', {
      name: 'Enable channel-model circuit breaker',
    })

    await user.click(circuitBreakerSwitch)

    expect(
      screen.getByRole('spinbutton', { name: 'Failure window (minutes)' })
    ).toBeEnabled()
    expect(
      screen.getByRole('textbox', { name: 'Excluded channel IDs' })
    ).toBeEnabled()
    expect(
      screen.getByRole('spinbutton', {
        name: 'Consecutive successes to recover',
      })
    ).toBeEnabled()

    queryClient.clear()
  })

  it('accepts full-width separators and duplicate excluded channel IDs', async () => {
    const user = userEvent.setup()
    const queryClient = renderSection(true)
    const excludedInput = screen.getByRole('textbox', {
      name: 'Excluded channel IDs',
    })

    await user.type(excludedInput, '10，2,10')

    expect(excludedInput).toHaveValue('10，2,10')
    expect(screen.queryByText(/Invalid channel IDs/)).not.toBeInTheDocument()

    queryClient.clear()
  })

  it('requires a webhook key before WeCom notifications can be enabled', async () => {
    const user = userEvent.setup()
    const queryClient = renderSection(true)
    const notificationSwitch = screen.getByRole('switch', {
      name: 'Enable WeCom bot notifications',
    })

    expect(notificationSwitch).not.toBeChecked()
    await user.click(notificationSwitch)
    const form = document.querySelector('form')
    if (!form) throw new Error('settings form not found')
    fireEvent.submit(form)

    expect(
      await screen.findByText(
        'Webhook Key is required before enabling notifications'
      )
    ).toBeInTheDocument()

    queryClient.clear()
  })
})
