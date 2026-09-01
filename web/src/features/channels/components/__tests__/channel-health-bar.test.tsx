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
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, test, vi } from 'vitest'

import { ChannelHealthBar } from '../channel-health-bar'

describe('ChannelHealthBar', () => {
  test('anchors the tooltip to the hovered block and keeps it outside clipped containers', async () => {
    const user = userEvent.setup()
    const { container } = render(
      <div className='overflow-hidden'>
        <ChannelHealthBar
          buckets={[
            { success: 1, failed: 0 },
            { success: 0, failed: 1 },
          ]}
          blockCount={2}
          blockSeconds={600}
          startTs={0}
        />
      </div>
    )

    const blocks = container.querySelectorAll('[data-health-block-index]')
    expect(blocks).toHaveLength(2)
    expect(blocks[0]).toHaveAttribute('data-slot', 'tooltip-trigger')
    expect(blocks[1]).toHaveAttribute('data-slot', 'tooltip-trigger')
    expect(blocks[1]).toHaveClass('cursor-pointer')
    expect(blocks[1]).toHaveClass('hover:scale-y-[1.8]')

    await user.hover(blocks[1])

    const failCount = await screen.findByText('Fail 1')
    await waitFor(() => {
      const tooltip = failCount.closest('[data-slot="tooltip-content"]')
      expect(tooltip).not.toBeNull()
      expect(tooltip).toHaveAttribute('data-side', 'top')
      expect(tooltip?.closest('.overflow-hidden')).toBeNull()
      expect(tooltip?.querySelector('[data-slot="tooltip-arrow"]')).toBeNull()

      const range = tooltip?.querySelector('[data-health-tooltip-range]')
      expect(range?.parentElement).toHaveClass('space-y-0.5')
      expect(range?.parentElement?.children).toHaveLength(2)
    })
  })

  test('invokes the block click handler with the selected block start', async () => {
    const user = userEvent.setup()
    const onBlockClick = vi.fn()
    const { container } = render(
      <ChannelHealthBar
        buckets={[
          { success: 1, failed: 0 },
          { success: 0, failed: 1 },
        ]}
        blockCount={2}
        blockSeconds={600}
        startTs={1_000}
        onBlockClick={onBlockClick}
      />
    )

    const secondBlock = container.querySelector('[data-health-block-index="1"]')
    if (!secondBlock) {
      throw new Error('second health block was not rendered')
    }

    await user.click(secondBlock)

    expect(onBlockClick).toHaveBeenCalledWith(1_600)
  })
})
