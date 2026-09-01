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
import { useQuery } from '@tanstack/react-query'
import { useMemo, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from '@/components/ui/command'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import {
  sideDrawerContentClassName,
  sideDrawerHeaderClassName,
} from '@/components/drawer-layout'
import { getLobeIcon } from '@/lib/lobe-icon'
import { cn } from '@/lib/utils'

import { getAllModels } from '../../api'
import { modelsQueryKeys } from '../../lib'
import type { Model, Provider } from '../../types'

export function ProviderModelAddDrawer(props: {
  open: boolean
  onOpenChange: (open: boolean) => void
  provider: Provider
  configuredModelIds: ReadonlySet<number>
  configuredModelsLoading?: boolean
  configuredModelsError?: boolean
  onSelect: (model: Model) => void
}) {
  const { t } = useTranslation()
  const [searchValue, setSearchValue] = useState('')
  const modelsQuery = useQuery({
    queryKey: modelsQueryKeys.list({
      status: 'enabled',
      page_size: 100,
    }),
    queryFn: () => getAllModels({ status: 'enabled' }),
    enabled: props.open,
  })

  const candidates = useMemo(() => {
    const keyword = searchValue.trim().toLowerCase()
    return (modelsQuery.data ?? [])
      .filter((model) => !props.configuredModelIds.has(model.id))
      .filter((model) => model.name_rule === 0)
      .filter((model) => {
        if (!keyword) return true
        return (
          model.model_name.toLowerCase().includes(keyword) ||
          model.description?.toLowerCase().includes(keyword) ||
          model.tags?.toLowerCase().includes(keyword)
        )
      })
  }, [modelsQuery.data, props.configuredModelIds, searchValue])

  const handleOpenChange = (open: boolean) => {
    if (!open) setSearchValue('')
    props.onOpenChange(open)
  }

  let modelsContent: ReactNode
  if (props.configuredModelsLoading || modelsQuery.isLoading) {
    modelsContent = (
      <div className='text-muted-foreground flex h-full items-center justify-center p-6 text-sm'>
        {t('Loading...')}
      </div>
    )
  } else if (props.configuredModelsError || modelsQuery.isError) {
    modelsContent = (
      <div className='text-destructive flex h-full items-center justify-center p-6 text-sm'>
        {t('Operation failed')}
      </div>
    )
  } else {
    modelsContent = (
      <Command shouldFilter={false} className='rounded-none'>
        <CommandInput
          placeholder={t('Search models...')}
          value={searchValue}
          onValueChange={setSearchValue}
        />
        <CommandList className='max-h-full'>
          <CommandEmpty>{t('No models found')}</CommandEmpty>
          <CommandGroup>
            {candidates.map((model) => {
              const icon = model.icon ? getLobeIcon(model.icon, 20) : null
              return (
                <CommandItem
                  key={model.id}
                  value={model.model_name}
                  onSelect={() => props.onSelect(model)}
                  className={cn(
                    'items-center gap-3 rounded-lg px-3 py-2.5',
                    'data-[selected=true]:bg-muted'
                  )}
                >
                  <div className='bg-muted flex size-8 shrink-0 items-center justify-center rounded-md'>
                    {icon || (
                      <span className='text-muted-foreground text-xs font-semibold'>
                        {model.model_name.charAt(0).toUpperCase()}
                      </span>
                    )}
                  </div>
                  <div className='min-w-0 flex-1'>
                    <div className='truncate font-mono text-sm font-medium'>
                      {model.model_name}
                    </div>
                    {model.description && (
                      <div className='text-muted-foreground truncate text-xs'>
                        {model.description}
                      </div>
                    )}
                  </div>
                </CommandItem>
              )
            })}
          </CommandGroup>
        </CommandList>
      </Command>
    )
  }

  return (
    <Sheet open={props.open} onOpenChange={handleOpenChange}>
      <SheetContent
        side='right'
        className={sideDrawerContentClassName('sm:max-w-xl')}
      >
        <SheetHeader className={sideDrawerHeaderClassName()}>
          <SheetTitle>{t('Add model')}</SheetTitle>
          <SheetDescription>
            {props.provider.display_name} · {t('Select a model to edit pricing')}
          </SheetDescription>
        </SheetHeader>

        <div className='flex min-h-0 flex-1 flex-col gap-3 px-4 pb-6 sm:px-6'>
          <div className='min-h-0 flex-1 overflow-hidden rounded-lg border'>
            {modelsContent}
          </div>
        </div>
      </SheetContent>
    </Sheet>
  )
}
