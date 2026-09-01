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
import { useTranslation } from 'react-i18next'

import { ModelProviderPricingSection } from './model-provider-pricing-section'

export function ModelProviderConfigDrawer(props: {
  open: boolean
  onOpenChange: (open: boolean) => void
  modelId?: number
  modelName?: string
  providerSlug?: string
  providerName?: string
}) {
  const { t } = useTranslation()
  const title = props.modelName
    ? `${props.modelName} · ${props.providerName || props.providerSlug || t('Provider')}`
    : t('Model-Provider configuration')

  return (
    <Sheet open={props.open} onOpenChange={props.onOpenChange}>
      <SheetContent
        side='right'
        className={sideDrawerContentClassName('sm:max-w-2xl')}
      >
        <SheetHeader className={sideDrawerHeaderClassName()}>
          <SheetTitle>{title}</SheetTitle>
          <SheetDescription>
            {t('Configure prices and metadata for this model and provider.')}
          </SheetDescription>
        </SheetHeader>
        <div className='flex-1 overflow-y-auto px-4 pb-6 sm:px-6'>
          <ModelProviderPricingSection
            modelId={props.modelId}
            enabled={props.open}
            providerSlug={props.providerSlug}
          />
        </div>
      </SheetContent>
    </Sheet>
  )
}
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
