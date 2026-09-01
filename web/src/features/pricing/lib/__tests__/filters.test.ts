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
import {
  filterByAdvanced,
  getProviderQuantizations,
  getProviderRegions,
  hasValues,
} from '../filters'

const models: PricingModel[] = [
  {
    id: 1,
    model_name: 'small',
    quota_type: 0,
    model_ratio: 1,
    completion_ratio: 1,
    enable_groups: ['default'],
    context_length: 8192,
    parameter_count: '7B',
  },
  {
    id: 2,
    model_name: 'large',
    quota_type: 0,
    model_ratio: 1,
    completion_ratio: 1,
    enable_groups: ['default'],
    context_length: 131072,
    parameter_count: '70B',
  },
]

describe('pricing advanced filters', () => {
  it('filters models by metadata ranges', () => {
    const filtered = filterByAdvanced(models, {
      contextLength: 'gte-128k',
      parameterCount: '50b-100b',
      releaseDate: 'all',
      free: 'all',
      batch: 'all',
      region: 'all',
      quantization: 'all',
    })

    expect(filtered.map((model) => model.model_name)).toEqual(['large'])
  })

  it('hides metadata filter sections when no values are present', () => {
    expect(hasValues(models, (model) => model.release_date)).toBe(false)
  })

  it('filters by available provider metadata and exposes dynamic values', () => {
    const providerModels: PricingModel[] = [
      {
        ...models[0],
        model_name: 'free-model',
        providers: [
          {
            slug: 'siliconflow',
            name: 'SiliconFlow',
            available: true,
            pricing: { input_price: 0, output_price: 0, source: 'model-provider' },
            metadata: { region: 'CN', quantization: 'FP8', batch: true },
          },
        ],
      },
      {
        ...models[1],
        model_name: 'paid-model',
        providers: [
          {
            slug: 'openai',
            name: 'OpenAI',
            available: true,
            pricing: { input_price: 1, output_price: 2, source: 'model-provider' },
            metadata: { region: 'US', quantization: 'FP16', batch: false },
          },
        ],
      },
    ]

    expect(
      filterByAdvanced(providerModels, {
        contextLength: 'all',
        parameterCount: 'all',
        releaseDate: 'all',
        free: 'supported',
        batch: 'supported',
        region: 'CN',
        quantization: 'FP8',
      }).map((model) => model.model_name)
    ).toEqual(['free-model'])
    expect(getProviderRegions(providerModels)).toEqual(['CN', 'US'])
    expect(getProviderQuantizations(providerModels)).toEqual(['FP16', 'FP8'])
  })
})
