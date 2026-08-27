/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { cn } from '@/lib/utils'

import type { ChannelHealthBucket } from '../types'

/**
 * Count of requests this node is currently relaying for a channel. Rendered in
 * both the table's status cell and the card view, hence a shared component.
 */
export function ChannelInFlightBadge({ count }: { count: number }) {
  const { t } = useTranslation()
  if (count <= 0) {
    return null
  }
  return (
    <TooltipProvider delay={100}>
      <Tooltip>
        <TooltipTrigger
          render={
            <span className='flex shrink-0 items-center gap-1 rounded bg-sky-50 px-1.5 py-0.5 text-xs font-medium text-sky-600 tabular-nums dark:bg-sky-500/10 dark:text-sky-400'>
              <span className='size-1.5 rounded-full bg-sky-500' />
              {count}
            </span>
          }
        />
        <TooltipContent side='top'>
          {t('In-flight requests on the current node')}
        </TooltipContent>
      </Tooltip>
    </TooltipProvider>
  )
}

/** Red -> amber -> green, interpolated on the success rate of a block. */
const COLOR_STOPS = [
  { r: 239, g: 68, b: 68 },
  { r: 250, g: 204, b: 21 },
  { r: 34, g: 197, b: 94 },
] as const

/** A block with no traffic at all, kept visually distinct from a failing one. */
const IDLE_RATE = -1

function rateToColor(rate: number): string {
  const clamped = Math.min(1, Math.max(0, rate))
  const scaled = clamped * (COLOR_STOPS.length - 1)
  const lowerIndex = Math.floor(scaled)
  const upperIndex = Math.min(COLOR_STOPS.length - 1, lowerIndex + 1)
  const weight = scaled - lowerIndex
  const lower = COLOR_STOPS[lowerIndex]
  const upper = COLOR_STOPS[upperIndex]
  const channel = (from: number, to: number) =>
    Math.round(from + (to - from) * weight)
  return `rgb(${channel(lower.r, upper.r)}, ${channel(lower.g, upper.g)}, ${channel(lower.b, upper.b)})`
}

function successRateBadgeClass(hasTraffic: boolean, successRate: number) {
  if (!hasTraffic) {
    return 'bg-muted text-muted-foreground'
  }
  if (successRate >= 90) {
    return 'bg-emerald-50 text-emerald-600 dark:bg-emerald-500/10 dark:text-emerald-400'
  }
  if (successRate >= 50) {
    return 'bg-amber-50 text-amber-600 dark:bg-amber-500/10 dark:text-amber-400'
  }
  return 'bg-red-50 text-red-600 dark:bg-red-500/10 dark:text-red-400'
}

interface ChannelHealthBarProps {
  /** Oldest block first. Shorter arrays are left-padded with idle blocks. */
  buckets?: ChannelHealthBucket[] | null
  blockCount: number
  blockSeconds: number
  /** Unix seconds at which the first block starts. */
  startTs: number
  /** Called with the block start when a user selects a time slice. */
  onBlockClick?: (startTs: number) => void
  className?: string
}

interface HealthBlock {
  success: number
  failed: number
  total: number
  /** Success ratio in [0, 1], or IDLE_RATE when the block saw no traffic. */
  rate: number
  /** Unix seconds at which this block starts; also its render key. */
  startTs: number
}

function formatHealthTime(unixSeconds: number): string {
  return new Date(unixSeconds * 1000).toLocaleTimeString(undefined, {
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  })
}

/**
 * Rolling availability bar for a single channel: one block per time slice,
 * coloured by that slice's success rate, with an aggregate percentage badge.
 */
export function ChannelHealthBar({
  buckets,
  blockCount,
  blockSeconds,
  startTs,
  onBlockClick,
  className,
}: ChannelHealthBarProps) {
  const { t } = useTranslation()

  const { blocks, successRate, hasTraffic } = useMemo(() => {
    const source = buckets ?? []
    // Keep the newest blocks when the backend returns more than we render, and
    // pad older slots when it returns fewer, so the right edge is always "now".
    const trimmed = source.slice(-blockCount)
    const padded: HealthBlock[] = Array.from(
      { length: Math.max(0, blockCount - trimmed.length) },
      () => ({ success: 0, failed: 0, total: 0, rate: IDLE_RATE, startTs: 0 })
    )

    let totalSuccess = 0
    let totalRequests = 0
    for (const bucket of trimmed) {
      const success = Math.max(0, bucket?.success ?? 0)
      const failed = Math.max(0, bucket?.failed ?? 0)
      const total = success + failed
      totalSuccess += success
      totalRequests += total
      padded.push({
        success,
        failed,
        total,
        rate: total > 0 ? success / total : IDLE_RATE,
        startTs: 0,
      })
    }
    // A block is identified by the instant it starts, which is stable across
    // refreshes as long as the block still belongs to the window.
    for (const [index, block] of padded.entries()) {
      block.startTs = startTs + index * blockSeconds
    }

    return {
      blocks: padded,
      successRate: totalRequests > 0 ? (totalSuccess / totalRequests) * 100 : 0,
      hasTraffic: totalRequests > 0,
    }
  }, [buckets, blockCount, blockSeconds, startTs])

  const badgeClass = successRateBadgeClass(hasTraffic, successRate)

  return (
    <div className={cn('flex items-center gap-1.5', className)}>
      <TooltipProvider delay={0}>
        <div className='flex min-w-0 flex-1 items-center gap-[2px]'>
          {blocks.map((block, index) => {
            const range = `${formatHealthTime(block.startTs)} — ${formatHealthTime(block.startTs + blockSeconds)}`

            return (
              <Tooltip key={block.startTs}>
                <TooltipTrigger
                  render={
                    <div
                      aria-label={range}
                      data-health-block-index={index}
                      role={onBlockClick ? 'button' : undefined}
                      tabIndex={onBlockClick ? 0 : undefined}
                      className={cn(
                        'h-3.5 min-w-[3px] flex-1 origin-center cursor-pointer rounded-[2px] transition-[transform,opacity] duration-150 hover:scale-y-[1.8] hover:opacity-90',
                        block.rate === IDLE_RATE && 'bg-border'
                      )}
                      style={
                        block.rate === IDLE_RATE
                          ? undefined
                          : { backgroundColor: rateToColor(block.rate) }
                      }
                      onClick={() => onBlockClick?.(block.startTs)}
                      onKeyDown={(event) => {
                        if (event.key === 'Enter' || event.key === ' ') {
                          event.preventDefault()
                          onBlockClick?.(block.startTs)
                        }
                      }}
                    />
                  }
                />
                <TooltipContent
                  side='top'
                  sideOffset={6}
                  showArrow={false}
                  className='bg-popover text-popover-foreground border px-2 py-1 whitespace-nowrap shadow-md'
                >
                  <div className='space-y-0.5'>
                    <div
                      data-health-tooltip-range
                      className='text-muted-foreground'
                    >
                      {range}
                    </div>
                    {block.total > 0 ? (
                      <div className='flex items-center gap-1.5'>
                        <span className='text-emerald-600 dark:text-emerald-400'>
                          {t('OK')} {block.success}
                        </span>
                        <span className='text-red-600 dark:text-red-400'>
                          {t('Fail')} {block.failed}
                        </span>
                        <span className='text-muted-foreground'>
                          ({((block.success / block.total) * 100).toFixed(1)}%)
                        </span>
                      </div>
                    ) : (
                      <div className='text-muted-foreground'>
                        {t('No requests')}
                      </div>
                    )}
                  </div>
                </TooltipContent>
              </Tooltip>
            )
          })}
        </div>
      </TooltipProvider>

      <span
        className={cn(
          'shrink-0 rounded px-1.5 py-0.5 text-xs font-medium tabular-nums',
          badgeClass
        )}
      >
        {hasTraffic ? `${successRate.toFixed(1)}%` : '--'}
      </span>
    </div>
  )
}
