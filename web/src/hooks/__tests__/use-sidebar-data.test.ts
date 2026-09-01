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
import { renderHook } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { useStatus } from '@/hooks/use-status'

import { useSidebarData } from '../use-sidebar-data'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))

vi.mock('@/hooks/use-status', () => ({
  useStatus: vi.fn(),
}))

describe('useSidebarData', () => {
  beforeEach(() => {
    vi.mocked(useStatus).mockReturnValue({
      status: {
        HeaderNavModules: JSON.stringify({
          pricing: { enabled: true, requireAuth: false },
        }),
      } as ReturnType<typeof useStatus>['status'],
      loading: false,
      error: null,
    })
  })

  it('shows the model square link when pricing is enabled', () => {
    const { result } = renderHook(() => useSidebarData())
    const models = result.current.navGroups.find(
      (group) => group.id === 'models'
    )

    expect(models?.items[0]).toMatchObject({
      title: 'Model Square',
      url: '/model-square',
    })
  })

  it('hides the model square link when pricing is disabled', () => {
    vi.mocked(useStatus).mockReturnValue({
      status: {
        HeaderNavModules: JSON.stringify({
          pricing: { enabled: false, requireAuth: false },
        }),
      } as ReturnType<typeof useStatus>['status'],
      loading: false,
      error: null,
    })

    const { result } = renderHook(() => useSidebarData())
    const general = result.current.navGroups.find(
      (group) => group.id === 'general'
    )

    expect(general?.items[0]).toMatchObject({
      title: 'Overview',
      url: '/dashboard/overview',
    })
  })
})
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
