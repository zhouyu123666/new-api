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
import { useTranslation } from 'react-i18next'

import { StatusBadge } from '@/components/status-badge'

type ChannelModelBadgeProps = {
  disabled: boolean
  model: string
}

export function ChannelModelBadge(props: ChannelModelBadgeProps) {
  const { t } = useTranslation()

  return (
    <StatusBadge
      label={props.model}
      autoColor={props.disabled ? undefined : props.model}
      variant={props.disabled ? 'warning' : undefined}
      size='sm'
      className='font-mono'
      data-disabled={props.disabled || undefined}
      aria-label={
        props.disabled ? `${props.model} · ${t('Auto Disabled')}` : props.model
      }
    />
  )
}
