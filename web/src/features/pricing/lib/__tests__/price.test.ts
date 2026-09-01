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
import { describe, expect, it } from 'vitest'

import type { PricingModel } from '../../types'
import { getPrimaryProvider } from '../price'

describe('model square provider pricing', () => {
  it('uses the standard top provider as the model card price source', () => {
    const model: PricingModel = {
      id: 1,
      model_name: 'deepseek-v4-flash',
      quota_type: 0,
      model_ratio: 1,
      completion_ratio: 1,
      enable_groups: ['default'],
      providers: [
        {
          slug: 'siliconflow',
          name: 'SiliconFlow',
          available: false,
          pricing: {
            input_price: 1,
            output_price: 2,
            source: 'model-provider',
          },
        },
        {
          slug: 'deepseek',
          name: 'DeepSeek',
          available: true,
          pricing: {
            input_price: 3,
            output_price: 9,
            source: 'model-provider',
          },
        },
      ],
    }

    expect(getPrimaryProvider(model)?.slug).toBe('deepseek')
  })

  it('uses the lowest configured price among available providers', () => {
    const model: PricingModel = {
      id: 3,
      model_name: 'deepseek-v4-flash',
      quota_type: 0,
      model_ratio: 1,
      completion_ratio: 1,
      enable_groups: ['default'],
      providers: [
        {
          slug: 'deepseek',
          name: 'DeepSeek',
          available: true,
          pricing: {
            input_price: 3,
            output_price: 9,
            source: 'model-provider',
          },
        },
        {
          slug: 'siliconflow',
          name: 'SiliconFlow',
          available: true,
          pricing: {
            input_price: 1,
            output_price: 2,
            source: 'model-provider',
          },
        },
      ],
    }

    expect(getPrimaryProvider(model)?.slug).toBe('siliconflow')
  })

  it('falls back to the first provider when none are available', () => {
    const model: PricingModel = {
      id: 2,
      model_name: 'model-without-health-status',
      quota_type: 0,
      model_ratio: 1,
      completion_ratio: 1,
      enable_groups: [],
      providers: [
        { slug: 'first', name: 'First', available: false },
        { slug: 'second', name: 'Second', available: false },
      ],
    }

    expect(getPrimaryProvider(model)?.slug).toBe('first')
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
