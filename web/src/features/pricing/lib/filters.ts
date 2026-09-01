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
  SORT_OPTIONS,
  FILTER_ALL,
  QUOTA_TYPES,
  QUOTA_TYPE_VALUES,
  ENDPOINT_TYPES,
} from '../constants'
import type { PricingModel, PricingProvider } from '../types'

export type PricingAdvancedFilters = {
  contextLength: string
  parameterCount: string
  releaseDate: string
  free: string
  batch: string
  region: string
  quantization: string
}

const RANGE_LIMITS = [8192, 32768, 131072]

function parseParameterCount(value?: string): number | null {
  if (!value) return null
  const match = value
    .trim()
    .toUpperCase()
    .match(/([\d.]+)\s*([KMBT])?/)
  if (!match) return null
  const amount = Number(match[1])
  if (!Number.isFinite(amount)) return null
  const multiplier =
    { K: 1e3, M: 1e6, B: 1e9, T: 1e12 }[match[2] as 'K' | 'M' | 'B' | 'T'] ?? 1
  return amount * multiplier
}

function matchesRange(value: number | null, range: string): boolean {
  if (value == null || range === FILTER_ALL) return range === FILTER_ALL
  if (range === 'lte-8k') return value <= RANGE_LIMITS[0]
  if (range === '8k-32k') {
    return value > RANGE_LIMITS[0] && value <= RANGE_LIMITS[1]
  }
  if (range === '32k-128k') {
    return value > RANGE_LIMITS[1] && value <= RANGE_LIMITS[2]
  }
  if (range === 'gte-128k') return value >= RANGE_LIMITS[2]
  return true
}

function filterByRange(
  models: PricingModel[],
  range: string,
  getValue: (model: PricingModel) => number | null
): PricingModel[] {
  if (range === FILTER_ALL) return models
  return models.filter((model) => matchesRange(getValue(model), range))
}

function filterByParameterRange(
  models: PricingModel[],
  range: string
): PricingModel[] {
  if (range === FILTER_ALL) return models
  return models.filter((model) => {
    const value = parseParameterCount(model.parameter_count)
    if (value == null) return false
    if (range === 'lte-10b') return value <= 10e9
    if (range === '10b-50b') return value > 10e9 && value <= 50e9
    if (range === '50b-100b') return value > 50e9 && value <= 100e9
    if (range === 'gte-100b') return value >= 100e9
    return true
  })
}

function getAvailableProviders(model: PricingModel): PricingProvider[] {
  return (model.providers ?? []).filter((provider) => provider.available)
}

export function isFreeProvider(provider: PricingProvider): boolean {
  if (provider.metadata?.free === true) return true
  return (
    provider.pricing?.input_price === 0 && provider.pricing?.output_price === 0
  )
}

function hasProviderCapability(
  model: PricingModel,
  capability: 'free' | 'batch'
): boolean {
  return getAvailableProviders(model).some((provider) =>
    capability === 'free'
      ? isFreeProvider(provider)
      : provider.metadata?.batch === true
  )
}

function hasProviderValue(
  model: PricingModel,
  field: 'region' | 'quantization',
  value: string
): boolean {
  return getAvailableProviders(model).some(
    (provider) => provider.metadata?.[field] === value
  )
}

export function getProviderRegions(models: PricingModel[]): string[] {
  return collectProviderValues(models, 'region')
}

export function getProviderQuantizations(models: PricingModel[]): string[] {
  return collectProviderValues(models, 'quantization')
}

function collectProviderValues(
  models: PricingModel[],
  field: 'region' | 'quantization'
): string[] {
  const values = new Set<string>()
  for (const model of models) {
    for (const provider of getAvailableProviders(model)) {
      const value = provider.metadata?.[field]?.trim()
      if (value) values.add(value)
    }
  }
  return [...values].sort((left, right) => left.localeCompare(right))
}

export function hasValues(
  models: PricingModel[],
  getValues: (model: PricingModel) => unknown
): boolean {
  return models.some((model) => {
    const value = getValues(model)
    return Array.isArray(value)
      ? value.length > 0
      : value != null && value !== ''
  })
}

export const ADVANCED_RANGE_OPTIONS = [
  { value: 'lte-8k', label: '≤ 8K' },
  { value: '8k-32k', label: '8K–32K' },
  { value: '32k-128k', label: '32K–128K' },
  { value: 'gte-128k', label: '≥ 128K' },
] as const

export const PARAMETER_RANGE_OPTIONS = [
  { value: 'lte-10b', label: '≤ 10B' },
  { value: '10b-50b', label: '10B–50B' },
  { value: '50b-100b', label: '50B–100B' },
  { value: 'gte-100b', label: '≥ 100B' },
] as const

// ----------------------------------------------------------------------------
// Filter Utilities
// ----------------------------------------------------------------------------

/**
 * Filter models by search query
 */
export function filterBySearch(
  models: PricingModel[],
  query: string
): PricingModel[] {
  if (!query) return models

  const lowerQuery = query.toLowerCase()
  return models.filter(
    (m) =>
      m.model_name?.toLowerCase().includes(lowerQuery) ||
      m.description?.toLowerCase().includes(lowerQuery) ||
      m.tags?.toLowerCase().includes(lowerQuery) ||
      m.vendor_name?.toLowerCase().includes(lowerQuery)
  )
}

/**
 * Filter models by vendor
 */
export function filterByVendor(
  models: PricingModel[],
  vendor: string
): PricingModel[] {
  if (vendor === FILTER_ALL) return models
  return models.filter((m) => m.vendor_name === vendor)
}

/**
 * Filter models by group
 */
export function filterByGroup(
  models: PricingModel[],
  group: string
): PricingModel[] {
  if (group === FILTER_ALL) return models
  return models.filter((m) => m.enable_groups?.includes(group))
}

/**
 * Filter models by quota type
 */
export function filterByQuotaType(
  models: PricingModel[],
  quotaType: string
): PricingModel[] {
  if (quotaType === QUOTA_TYPES.ALL) return models
  const targetType =
    quotaType === QUOTA_TYPES.TOKEN
      ? QUOTA_TYPE_VALUES.TOKEN
      : QUOTA_TYPE_VALUES.REQUEST
  return models.filter((m) => m.quota_type === targetType)
}

/**
 * Filter models by endpoint type
 */
export function filterByEndpointType(
  models: PricingModel[],
  endpointType: string
): PricingModel[] {
  if (endpointType === ENDPOINT_TYPES.ALL) return models
  return models.filter((m) =>
    m.supported_endpoint_types?.includes(endpointType)
  )
}

/**
 * Get model price for sorting
 */
function getModelPrice(model: PricingModel): number {
  return model.quota_type === 0 ? model.model_ratio : model.model_price || 0
}

/**
 * Sort models by specified option
 */
export function sortModels(
  models: PricingModel[],
  sortBy: string
): PricingModel[] {
  const sorted = [...models]

  switch (sortBy) {
    case SORT_OPTIONS.NAME:
      sorted.sort((a, b) =>
        (a.model_name || '').localeCompare(b.model_name || '')
      )
      break
    case SORT_OPTIONS.PRICE_LOW:
      sorted.sort((a, b) => getModelPrice(a) - getModelPrice(b))
      break
    case SORT_OPTIONS.PRICE_HIGH:
      sorted.sort((a, b) => getModelPrice(b) - getModelPrice(a))
      break
  }

  return sorted
}

/**
 * Apply all filters and sorting to models
 */
export function filterAndSortModels(
  models: PricingModel[],
  filters: {
    search: string
    vendor: string
    group: string
    quotaType: string
    endpointType: string
    tag: string
    sortBy: string
  }
): PricingModel[] {
  let result = filterBySearch(models, filters.search)
  result = filterByVendor(result, filters.vendor)
  result = filterByGroup(result, filters.group)
  result = filterByQuotaType(result, filters.quotaType)
  result = filterByEndpointType(result, filters.endpointType)
  result = filterByTag(result, filters.tag)
  result = sortModels(result, filters.sortBy)

  return result
}

/**
 * Parse tags from comma-separated string
 */
export function parseTags(tagsString?: string): string[] {
  if (!tagsString) return []
  return tagsString
    .split(/[,;|\s]+/)
    .map((t) => t.trim())
    .filter(Boolean)
}

/**
 * Extract all unique tags from models
 */
export function extractAllTags(models: PricingModel[]): string[] {
  const tagSet = new Set<string>()

  models.forEach((model) => {
    if (model.tags) {
      const tags = parseTags(model.tags)
      tags.forEach((tag) => {
        tagSet.add(tag.toLowerCase())
      })
    }
  })

  return [...tagSet].sort((a, b) => a.localeCompare(b))
}

/**
 * Filter models by tag
 */
export function filterByTag(
  models: PricingModel[],
  tag: string
): PricingModel[] {
  if (tag === FILTER_ALL) return models

  const tagLower = tag.toLowerCase()
  return models.filter((m) => {
    if (!m.tags) return false
    const modelTags = parseTags(m.tags).map((t) => t.toLowerCase())
    return modelTags.includes(tagLower)
  })
}

export function filterByAdvanced(
  models: PricingModel[],
  filters: PricingAdvancedFilters
): PricingModel[] {
  let result = filterByRange(
    models,
    filters.contextLength,
    (model) => model.context_length ?? null
  )
  result = filterByParameterRange(result, filters.parameterCount)
  if (filters.releaseDate !== FILTER_ALL) {
    const days = Number(filters.releaseDate)
    const cutoff = Date.now() - days * 24 * 60 * 60 * 1000
    result = result.filter((model) => {
      const time = model.release_date
        ? Date.parse(model.release_date)
        : Number.NaN
      return Number.isFinite(time) && time >= cutoff
    })
  }
  if (filters.free !== FILTER_ALL) {
    result = result.filter((model) => hasProviderCapability(model, 'free'))
  }
  if (filters.batch !== FILTER_ALL) {
    result = result.filter((model) => hasProviderCapability(model, 'batch'))
  }
  if (filters.region !== FILTER_ALL) {
    result = result.filter((model) =>
      hasProviderValue(model, 'region', filters.region)
    )
  }
  if (filters.quantization !== FILTER_ALL) {
    result = result.filter((model) =>
      hasProviderValue(model, 'quantization', filters.quantization)
    )
  }
  return result
}
