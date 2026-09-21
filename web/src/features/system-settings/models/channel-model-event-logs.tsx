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
import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { RefreshCw, Search } from 'lucide-react'
import { useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { formatTimestampToDate } from '@/lib/format'
import { cn } from '@/lib/utils'

import { getChannelModelEvents } from '../api'
import type { ChannelModelEventType } from '../types'

const PAGE_SIZE = 20
const ALL_EVENTS = 'all'

type EventFilter = {
  channelId: string
  model: string
  event: ChannelModelEventType | typeof ALL_EVENTS
  startTime: string
  endTime: string
}

const emptyFilter: EventFilter = {
  channelId: '',
  model: '',
  event: ALL_EVENTS,
  startTime: '',
  endTime: '',
}

const eventVariants: Record<
  ChannelModelEventType,
  'warning' | 'success' | 'danger' | 'info'
> = {
  disabled: 'warning',
  probe_success: 'info',
  probe_failed: 'danger',
  recovered: 'success',
}

const eventLabels: Record<ChannelModelEventType, string> = {
  disabled: 'Route disabled',
  probe_success: 'Recovery probe succeeded',
  probe_failed: 'Recovery probe failed',
  recovered: 'Route recovered',
}

function toTimestamp(value: string): number | undefined {
  if (!value) return undefined
  const milliseconds = new Date(value).getTime()
  return Number.isNaN(milliseconds) ? undefined : Math.floor(milliseconds / 1000)
}

export function ChannelModelEventLogs() {
  const { t } = useTranslation()
  const [draft, setDraft] = useState<EventFilter>(emptyFilter)
  const [filter, setFilter] = useState<EventFilter>(emptyFilter)
  const [page, setPage] = useState(1)

  const eventsQuery = useQuery({
    queryKey: ['channel-model-events', page, filter],
    queryFn: async () => {
      const channelId = Number.parseInt(filter.channelId, 10)
      const response = await getChannelModelEvents({
        p: page,
        page_size: PAGE_SIZE,
        channel_id: Number.isNaN(channelId) ? undefined : channelId,
        model: filter.model.trim() || undefined,
        event: filter.event === ALL_EVENTS ? undefined : filter.event,
        start_time: toTimestamp(filter.startTime),
        end_time: toTimestamp(filter.endTime),
      })
      if (!response.success || !response.data) {
        throw new Error(response.message || t('Failed to load event logs'))
      }
      return response.data
    },
    placeholderData: keepPreviousData,
    retry: false,
  })

  const total = eventsQuery.data?.total ?? 0
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE))
  const events = eventsQuery.data?.items ?? []

  const applyFilter = () => {
    setPage(1)
    setFilter(draft)
  }

  const resetFilter = () => {
    setDraft(emptyFilter)
    setFilter(emptyFilter)
    setPage(1)
  }

  let eventContent: ReactNode
  if (eventsQuery.isLoading) {
    eventContent = (
      <div className='space-y-2'>
        {['first', 'second', 'third', 'fourth'].map((key) => (
          <Skeleton key={key} className='h-12 w-full' />
        ))}
      </div>
    )
  } else if (eventsQuery.isError) {
    eventContent = (
      <div className='text-destructive rounded-md border border-dashed p-8 text-center text-sm'>
        {eventsQuery.error instanceof Error
          ? eventsQuery.error.message
          : t('Failed to load event logs')}
      </div>
    )
  } else if (events.length === 0) {
    eventContent = (
      <div className='text-muted-foreground rounded-md border border-dashed p-8 text-center text-sm'>
        {t('No circuit breaker events found')}
      </div>
    )
  } else {
    eventContent = (
      <div className='overflow-hidden rounded-md border'>
        <Table className='min-w-[980px]'>
          <TableHeader>
            <TableRow className='bg-muted/40 hover:bg-muted/40'>
              <TableHead>{t('Time')}</TableHead>
              <TableHead>{t('Channel')}</TableHead>
              <TableHead>{t('Model')}</TableHead>
              <TableHead>{t('Event')}</TableHead>
              <TableHead>{t('Status code')}</TableHead>
              <TableHead>{t('Consecutive successes')}</TableHead>
              <TableHead className='min-w-[260px]'>{t('Reason')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {events.map((event) => (
              <TableRow key={event.id}>
                <TableCell>{formatTimestampToDate(event.created_at)}</TableCell>
                <TableCell>
                  <div className='font-medium'>{event.channel_name || '-'}</div>
                  <div className='text-muted-foreground text-xs'>
                    #{event.channel_id}
                  </div>
                </TableCell>
                <TableCell className='font-mono'>{event.model}</TableCell>
                <TableCell>
                  <StatusBadge
                    label={t(eventLabels[event.event])}
                    variant={eventVariants[event.event]}
                    copyable={false}
                  />
                </TableCell>
                <TableCell>{event.status_code || '-'}</TableCell>
                <TableCell>{event.success_count}</TableCell>
                <TableCell
                  className='max-w-[360px] truncate'
                  title={event.reason || undefined}
                >
                  {event.reason || '-'}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
    )
  }

  return (
    <section className='bg-card overflow-hidden rounded-lg border shadow-xs'>
      <div className='flex flex-col gap-3 border-b px-4 py-3 sm:flex-row sm:items-center sm:justify-between sm:px-5'>
        <div>
          <h3 className='text-sm font-semibold'>
            {t('Circuit breaker and recovery logs')}
          </h3>
          <p className='text-muted-foreground mt-0.5 text-xs'>
            {t(
              'Search route disablement, recovery probes, and successful recovery events.'
            )}
          </p>
        </div>
        <Button
          type='button'
          variant='outline'
          size='sm'
          onClick={() => void eventsQuery.refetch()}
          disabled={eventsQuery.isFetching}
        >
          <RefreshCw
            data-icon='inline-start'
            className={cn('size-3.5', eventsQuery.isFetching && 'animate-spin')}
            aria-hidden='true'
          />
          {t('Refresh')}
        </Button>
      </div>

      <div className='space-y-4 p-4 sm:p-5'>
        <div className='grid gap-3 md:grid-cols-2 xl:grid-cols-5'>
          <div className='space-y-1.5'>
            <Label htmlFor='event-channel-id'>{t('Channel ID')}</Label>
            <Input
              id='event-channel-id'
              type='number'
              min={1}
              value={draft.channelId}
              onChange={(event) =>
                setDraft((current) => ({
                  ...current,
                  channelId: event.target.value,
                }))
              }
            />
          </div>
          <div className='space-y-1.5'>
            <Label htmlFor='event-model'>{t('Model')}</Label>
            <Input
              id='event-model'
              value={draft.model}
              onChange={(event) =>
                setDraft((current) => ({
                  ...current,
                  model: event.target.value,
                }))
              }
            />
          </div>
          <div className='space-y-1.5'>
            <Label htmlFor='event-type'>{t('Event')}</Label>
            <Select
              value={draft.event}
              onValueChange={(value) =>
                setDraft((current) => ({
                  ...current,
                  event: value as EventFilter['event'],
                }))
              }
            >
              <SelectTrigger id='event-type'>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectGroup>
                  <SelectItem value={ALL_EVENTS}>{t('All events')}</SelectItem>
                  {(Object.keys(eventLabels) as ChannelModelEventType[]).map(
                    (event) => (
                      <SelectItem key={event} value={event}>
                        {t(eventLabels[event])}
                      </SelectItem>
                    )
                  )}
                </SelectGroup>
              </SelectContent>
            </Select>
          </div>
          <div className='space-y-1.5'>
            <Label htmlFor='event-start-time'>{t('Start time')}</Label>
            <Input
              id='event-start-time'
              type='datetime-local'
              value={draft.startTime}
              onChange={(event) =>
                setDraft((current) => ({
                  ...current,
                  startTime: event.target.value,
                }))
              }
            />
          </div>
          <div className='space-y-1.5'>
            <Label htmlFor='event-end-time'>{t('End time')}</Label>
            <Input
              id='event-end-time'
              type='datetime-local'
              value={draft.endTime}
              onChange={(event) =>
                setDraft((current) => ({
                  ...current,
                  endTime: event.target.value,
                }))
              }
            />
          </div>
        </div>
        <div className='flex justify-end gap-2'>
          <Button type='button' variant='outline' onClick={resetFilter}>
            {t('Reset')}
          </Button>
          <Button type='button' onClick={applyFilter}>
            <Search data-icon='inline-start' aria-hidden='true' />
            {t('Search')}
          </Button>
        </div>

        {eventContent}

        <div className='flex flex-col items-center justify-between gap-3 sm:flex-row'>
          <p className='text-muted-foreground text-xs'>
            {t('{{total}} events', { total })}
          </p>
          <div className='flex items-center gap-2'>
            <Button
              type='button'
              variant='outline'
              size='sm'
              disabled={page <= 1}
              onClick={() => setPage((current) => Math.max(1, current - 1))}
            >
              {t('Previous')}
            </Button>
            <span className='text-muted-foreground text-xs tabular-nums'>
              {t('Page {{page}} of {{total}}', { page, total: totalPages })}
            </span>
            <Button
              type='button'
              variant='outline'
              size='sm'
              disabled={page >= totalPages}
              onClick={() =>
                setPage((current) => Math.min(totalPages, current + 1))
              }
            >
              {t('Next')}
            </Button>
          </div>
        </div>
      </div>
    </section>
  )
}
