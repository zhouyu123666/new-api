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
import { ArrowDown, ArrowUp, ChevronsUpDown, Route } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Checkbox } from '@/components/ui/checkbox'
import { Button } from '@/components/ui/button'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'
import { Switch } from '@/components/ui/switch'
import { getLobeIcon } from '@/lib/lobe-icon'
import { cn } from '@/lib/utils'

import type { PlaygroundProviderOption } from '../../types'

export function PlaygroundProviderSelector(props: {
  providers: PlaygroundProviderOption[]
  providerOrder: string[]
  allowFallbacks: boolean
  loading?: boolean
  disabled?: boolean
  onProviderOrderChange: (order: string[]) => void
  onAllowFallbacksChange: (value: boolean) => void
}) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const orderedProviders = useMemo(() => {
    const providersBySlug = new Map(
      props.providers.map((provider) => [provider.slug, provider])
    )
    return props.providerOrder
      .map((slug) => providersBySlug.get(slug))
      .filter((provider): provider is PlaygroundProviderOption => Boolean(provider))
  }, [props.providerOrder, props.providers])

  const toggleProvider = (slug: string) => {
    if (props.providerOrder.includes(slug)) {
      props.onProviderOrderChange(
        props.providerOrder.filter((providerSlug) => providerSlug !== slug)
      )
      return
    }
    props.onProviderOrderChange([...props.providerOrder, slug])
  }

  const moveProvider = (slug: string, direction: -1 | 1) => {
    const index = props.providerOrder.indexOf(slug)
    const target = index + direction
    if (index < 0 || target < 0 || target >= props.providerOrder.length) return
    const next = [...props.providerOrder]
    const moved = next[index]
    next[index] = next[target]
    next[target] = moved
    props.onProviderOrderChange(next)
  }

  let summary = t('Default')
  if (orderedProviders.length === 1) {
    summary = orderedProviders[0].name
  } else if (orderedProviders.length > 1) {
    summary = `${orderedProviders[0].name} +${orderedProviders.length - 1}`
  }

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger
        render={
          <Button
            type='button'
            variant='outline'
            size='sm'
            role='combobox'
            aria-expanded={open}
            aria-label={t('Select provider')}
            disabled={props.disabled || props.loading || props.providers.length === 0}
            className='flex h-8 min-w-0 max-w-52 items-center gap-1.5 px-2.5 text-xs'
          />
        }
      >
        <Route className='text-muted-foreground size-3.5 shrink-0' />
        <span className='truncate'>{summary}</span>
        <ChevronsUpDown className='text-muted-foreground size-3.5 shrink-0 opacity-60' />
      </PopoverTrigger>
      <PopoverContent align='end' className='w-80 p-2'>
        <div className='mb-2 flex items-center justify-between gap-2 px-1'>
          <div className='min-w-0'>
            <div className='text-sm font-medium'>{t('Provider')}</div>
            <div className='text-muted-foreground truncate text-xs'>
              {t('Select provider')}
            </div>
          </div>
          <Button
            type='button'
            variant='ghost'
            size='sm'
            className='h-7 px-2 text-xs'
            disabled={props.providerOrder.length === 0}
            onClick={() => {
              props.onProviderOrderChange([])
              props.onAllowFallbacksChange(true)
            }}
          >
            {t('Default')}
          </Button>
        </div>

        {props.providers.length === 0 ? (
          <div className='text-muted-foreground px-2 py-5 text-center text-xs'>
            {t('No providers available')}
          </div>
        ) : (
          <div className='space-y-1'>
            {props.providers.map((provider) => {
              const index = props.providerOrder.indexOf(provider.slug)
              const selected = index >= 0
              return (
                <div
                  key={provider.slug}
                  className={cn(
                    'flex items-center gap-2 rounded-md px-2 py-1.5',
                    selected && 'bg-muted/70'
                  )}
                >
                  <Checkbox
                    checked={selected}
                    onCheckedChange={() => toggleProvider(provider.slug)}
                    aria-label={provider.name}
                  />
                  {provider.icon ? (
                    <span className='flex size-5 shrink-0 items-center justify-center'>
                      {getLobeIcon(provider.icon, 16)}
                    </span>
                  ) : (
                    <span className='bg-muted text-muted-foreground flex size-5 shrink-0 items-center justify-center rounded text-[10px] font-semibold'>
                      {provider.name.charAt(0).toUpperCase()}
                    </span>
                  )}
                  <button
                    type='button'
                    className='min-w-0 flex-1 truncate text-left text-xs font-medium'
                    onClick={() => toggleProvider(provider.slug)}
                  >
                    {provider.name}
                  </button>
                  {selected && (
                    <span className='text-muted-foreground mr-1 font-mono text-[10px]'>
                      {index + 1}
                    </span>
                  )}
                  <Button
                    type='button'
                    variant='ghost'
                    size='icon-sm'
                    className='size-6'
                    disabled={!selected || index === 0}
                    aria-label={t('Move {{group}} up', { group: provider.name })}
                    onClick={() => moveProvider(provider.slug, -1)}
                  >
                    <ArrowUp className='size-3' />
                  </Button>
                  <Button
                    type='button'
                    variant='ghost'
                    size='icon-sm'
                    className='size-6'
                    disabled={!selected || index === props.providerOrder.length - 1}
                    aria-label={t('Move {{group}} down', { group: provider.name })}
                    onClick={() => moveProvider(provider.slug, 1)}
                  >
                    <ArrowDown className='size-3' />
                  </Button>
                </div>
              )
            })}
          </div>
        )}

        <div className='border-border mt-2 flex items-center justify-between border-t px-2 pt-2'>
          <span className='text-muted-foreground text-xs'>{t('Fallback')}</span>
          <Switch
            size='sm'
            checked={props.allowFallbacks}
            onCheckedChange={(checked) =>
              props.onAllowFallbacksChange(checked === true)
            }
          />
        </div>
      </PopoverContent>
    </Popover>
  )
}
