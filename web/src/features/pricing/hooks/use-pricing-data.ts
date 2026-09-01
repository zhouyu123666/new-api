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
import { useMemo } from 'react'

import { useStatus } from '@/hooks/use-status'

import { getAllModelSquare, getPricing } from '../api'
import type { ModelSquareData, PricingData } from '../types'

export function usePricingData(enabled = true) {
export function usePricingData(options?: {
  modelSquare?: boolean
}) {
  const { status } = useStatus()

  const pricingQuery = useQuery<PricingData>({
    queryKey: ['pricing'],
    queryFn: getPricing,
    enabled: options?.modelSquare !== true,
    staleTime: 5 * 60 * 1000,
    enabled,
  })
  const modelSquareQuery = useQuery<ModelSquareData>({
    queryKey: ['model-square'],
    queryFn: getAllModelSquare,
    enabled: options?.modelSquare === true,
    staleTime: 5 * 60 * 1000,
  })
  const isLoading = options?.modelSquare
    ? modelSquareQuery.isLoading
    : pricingQuery.isLoading
  const error = options?.modelSquare
    ? modelSquareQuery.error
    : pricingQuery.error
  const refetch = options?.modelSquare
    ? modelSquareQuery.refetch
    : pricingQuery.refetch

  // Ensure rates never reach zero to prevent division errors
  const priceRate = useMemo(
    () => Math.max((status?.price as number) ?? 1, 0.001),
    [status?.price]
  )
  const usdExchangeRate = useMemo(
    () => Math.max((status?.usd_exchange_rate as number) ?? priceRate, 0.001),
    [status?.usd_exchange_rate, priceRate]
  )

  const models = useMemo(() => {
    if (options?.modelSquare) {
      return modelSquareQuery.data?.data.items ?? []
    }
    if (!pricingQuery.data?.data || !pricingQuery.data?.vendors) return []

    const vendorMap = new Map(pricingQuery.data.vendors.map((v) => [v.id, v]))

    return pricingQuery.data.data.map((model) => {
      const vendor = model.vendor_id
        ? vendorMap.get(model.vendor_id)
        : undefined
      return {
        ...model,
        key: model.model_name,
        vendor_name: vendor?.name,
        vendor_icon: vendor?.icon,
        vendor_description: vendor?.description,
        group_ratio: pricingQuery.data.group_ratio,
      }
    })
  }, [modelSquareQuery.data, options?.modelSquare, pricingQuery.data])

  const modelSquareVendors = useMemo(() => {
    if (!options?.modelSquare) return []
    const seen = new Map<number, { id: number; name: string; icon?: string }>()
    for (const model of models) {
      if (!model.vendor_id || !model.vendor_name || seen.has(model.vendor_id)) {
        continue
      }
      seen.set(model.vendor_id, {
        id: model.vendor_id,
        name: model.vendor_name,
        icon: model.vendor_icon,
      })
    }
    return [...seen.values()]
  }, [models, options?.modelSquare])

  const modelSquareGroups = useMemo(() => {
    if (!options?.modelSquare) return {}
    const groups: Record<string, { desc: string; ratio: number }> = {}
    for (const model of models) {
      for (const group of model.enable_groups || []) {
        groups[group] ??= { desc: '', ratio: 1 }
      }
    }
    return groups
  }, [models, options?.modelSquare])

  const modelSquareEndpoints = useMemo(() => {
    if (!options?.modelSquare) return {}
    const endpoints: Record<string, string> = {}
    for (const model of models) {
      for (const endpoint of model.supported_endpoint_types || []) {
        endpoints[endpoint] = endpoint
      }
    }
    return endpoints
  }, [models, options?.modelSquare])

  return {
    models,
    vendors: options?.modelSquare
      ? modelSquareVendors
      : (pricingQuery.data?.vendors ?? []),
    groupRatio: options?.modelSquare
      ? Object.fromEntries(
          Object.keys(modelSquareGroups).map((group) => [group, 1])
        )
      : (pricingQuery.data?.group_ratio ?? {}),
    usableGroup: options?.modelSquare
      ? modelSquareGroups
      : (pricingQuery.data?.usable_group ?? {}),
    endpointMap: options?.modelSquare
      ? modelSquareEndpoints
      : (pricingQuery.data?.supported_endpoint ?? {}),
    autoGroups: options?.modelSquare
      ? []
      : (pricingQuery.data?.auto_groups ?? []),
    isLoading,
    error,
    refetch,
    priceRate,
    usdExchangeRate,
    modelSquareTotal: modelSquareQuery.data?.data.total ?? 0,
    modelSquareOffset: modelSquareQuery.data?.data.offset ?? 0,
    modelSquareLimit: modelSquareQuery.data?.data.limit ?? 24,
  }
}
