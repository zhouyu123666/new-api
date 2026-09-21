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
import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { ChannelModelBadge } from '../channel-model-badge'

describe('ChannelModelBadge', () => {
  it('marks an auto-disabled model route without renaming the model', () => {
    render(<ChannelModelBadge model='gpt-5' disabled />)

    const badge = screen.getByText('gpt-5').closest('[data-disabled]')
    expect(badge).toHaveAttribute('data-disabled', 'true')
    expect(badge).toHaveAttribute('aria-label', 'gpt-5 · Auto Disabled')
  })

  it('keeps an enabled model route unmarked', () => {
    render(<ChannelModelBadge model='gpt-5' disabled={false} />)

    const badge = screen.getByText('gpt-5').closest('[data-slot=status-badge]')
    expect(badge).not.toHaveAttribute('data-disabled')
    expect(badge).toHaveAttribute('aria-label', 'gpt-5')
  })
})
