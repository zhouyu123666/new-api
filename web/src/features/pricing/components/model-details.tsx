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
import { useNavigate, useParams, useSearch } from '@tanstack/react-router'
import {
  ArrowLeft,
  ArrowDown,
  ArrowUp,
  CalendarClock,
  ChevronRight,
  Code2,
  FileText,
  Gift,
  HeartPulse,
  Info,
  Layers,
  Maximize2,
  ShieldCheck,
  Sparkles,
  Timer,
  Zap,
} from 'lucide-react'
import { useCallback, useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { CopyButton } from '@/components/copy-button'
import { StaticDataTable } from '@/components/data-table'
import { sideDrawerContentClassName } from '@/components/drawer-layout'
import { GroupBadge } from '@/components/group-badge'
import { PublicLayout } from '@/components/layout'
import { Button } from '@/components/ui/button'
import { Switch } from '@/components/ui/switch'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { getPerfMetrics } from '@/features/performance-metrics/api'
import {
  formatLatency,
  formatThroughput,
  formatUptimePct,
  getSuccessRateTextClass,
} from '@/features/performance-metrics/lib/format'
import { getLobeIcon } from '@/lib/lobe-icon'
import { cn } from '@/lib/utils'
import { formatBillingCurrencyFromUSD } from '@/lib/currency'
import { useStatus } from '@/hooks/use-status'

import {
  getModelSquareDetail,
  getModelSquareProviderDetail,
} from '../api'
import { DEFAULT_TOKEN_UNIT } from '../constants'
import { usePricingData } from '../hooks/use-pricing-data'
import {
  getDynamicPriceEntries,
  getDynamicPricingSummary,
  getDynamicPricingTiers,
  isDynamicPricingModel,
} from '../lib/dynamic-price'
import { isFreeProvider, parseTags } from '../lib/filters'
import { getAvailableGroups, isTokenBasedModel } from '../lib/model-helpers'
import {
  formatFixedPrice,
  formatGroupPrice,
  formatPrice,
  sortProvidersForStandard,
} from '../lib/price'
import type {
  ModelCapability,
  PriceType,
  PricingProvider,
  PricingProviderMetadata,
  PricingModel,
  TokenUnit,
} from '../types'
import { DynamicPricingBreakdown } from './dynamic-pricing-breakdown'
import { ModelBillingModeBadge } from './model-billing-mode-badge'
import { ModelDetailsApi } from './model-details-api'
import { ModelDetailsPerformance } from './model-details-performance'

// ----------------------------------------------------------------------------
// Local UI helpers
// ----------------------------------------------------------------------------

function SectionTitle(props: { children: React.ReactNode }) {
  return (
    <h2 className='text-muted-foreground mb-3 text-xs font-semibold tracking-wider uppercase'>
      {props.children}
    </h2>
  )
}

const CAPABILITY_LABEL_KEYS: Record<ModelCapability, string> = {
  function_calling: 'Function calling',
  streaming: 'Streaming',
  vision: 'Vision',
  json_mode: 'JSON mode',
  structured_output: 'Structured output',
  reasoning: 'Reasoning',
  tools: 'Tools',
  system_prompt: 'System prompt',
  web_search: 'Web search',
  code_interpreter: 'Code interpreter',
  caching: 'Prompt caching',
  embeddings: 'Embeddings',
}

const MODALITY_LABEL_KEYS: Record<string, string> = {
  text: 'Text',
  image: 'Image',
  audio: 'Audio',
  video: 'Video',
  file: 'File',
}

const TOKEN_FORMAT = new Intl.NumberFormat(undefined, {
  maximumFractionDigits: 1,
})
const MODEL_DETAILS_SKELETON_KEYS = ['first', 'second', 'third', 'fourth']
const EMPTY_PROVIDERS: PricingProvider[] = []

function formatCatalogTokenCount(tokens: number): string {
  if (!Number.isFinite(tokens) || tokens <= 0) return ''
  if (tokens >= 1_000_000) {
    return `${TOKEN_FORMAT.format(tokens / 1_000_000)}M`
  }
  if (tokens >= 1_000) {
    return `${TOKEN_FORMAT.format(tokens / 1_000)}K`
  }
  return TOKEN_FORMAT.format(tokens)
}

function formatCatalogYearMonth(value?: string): string {
  if (!value) return ''
  const [yearStr, monthStr] = value.split('-')
  const year = Number(yearStr)
  const month = Number(monthStr)
  if (!Number.isFinite(year) || !Number.isFinite(month)) return value
  const date = new Date(Date.UTC(year, month - 1, 1))
  return date.toLocaleString(undefined, { year: 'numeric', month: 'short' })
}

function normalizeCatalogItems(items?: readonly string[]): string[] {
  if (!items) return []
  return items.filter((item) => item.trim().length > 0)
}

function OverviewMetric(props: {
  icon: React.ComponentType<{ className?: string }>
  label: string
  value: React.ReactNode
  valueClassName?: string
}) {
  const Icon = props.icon

  return (
    <div className='flex min-w-0 items-center gap-2 px-3 py-2'>
      <Icon className='text-muted-foreground/70 size-3.5 shrink-0' />
      <div className='min-w-0 flex-1'>
        <div className='text-muted-foreground truncate text-[10px] font-medium tracking-wider uppercase'>
          {props.label}
        </div>
        <div
          className={cn(
            'text-foreground truncate font-mono text-sm font-semibold tabular-nums',
            props.valueClassName
          )}
        >
          {props.value}
        </div>
      </div>
    </div>
  )
}

function OverviewSummaryGrid(props: { model: PricingModel }) {
  const { t } = useTranslation()
  const metricsQuery = useQuery({
    queryKey: ['perf-metrics', props.model.model_name],
    queryFn: () => getPerfMetrics(props.model.model_name, 24),
    staleTime: 60 * 1000,
  })

  const groups = metricsQuery.data?.data.groups ?? []
  const successRates = groups
    .map((group) => group.success_rate)
    .filter((rate) => Number.isFinite(rate))
  const successRate =
    successRates.length > 0
      ? successRates.reduce((sum, rate) => sum + rate, 0) / successRates.length
      : Number.NaN
  const tpsValues = groups
    .map((group) => group.avg_tps)
    .filter((value) => value > 0)
  const avgTps =
    tpsValues.length > 0
      ? tpsValues.reduce((sum, value) => sum + value, 0) / tpsValues.length
      : 0
  const latencyValues = groups
    .map((group) => group.avg_latency_ms)
    .filter((value) => value > 0)
  const avgLatency =
    latencyValues.length > 0
      ? Math.round(
          latencyValues.reduce((sum, value) => sum + value, 0) /
            latencyValues.length
        )
      : 0

  return (
    <div className='bg-muted/20 grid overflow-hidden rounded-lg border sm:grid-cols-3 sm:divide-x'>
      <OverviewMetric
        icon={Timer}
        label='TPS'
        value={formatThroughput(avgTps)}
      />
      <OverviewMetric
        icon={Timer}
        label={t('Average latency')}
        value={formatLatency(avgLatency)}
      />
      <OverviewMetric
        icon={HeartPulse}
        label={t('Success rate')}
        value={formatUptimePct(successRate)}
        valueClassName={getSuccessRateTextClass(successRate)}
      />
    </div>
  )
}

function CatalogPillList(props: { items: string[] }) {
  return (
    <div className='flex min-w-0 flex-wrap gap-1.5'>
      {props.items.map((item) => (
        <span
          key={item}
          className='bg-muted text-muted-foreground rounded-md px-2 py-1 text-xs font-medium'
        >
          {item}
        </span>
      ))}
    </div>
  )
}

function CatalogTextValue(props: { children: React.ReactNode }) {
  return (
    <span className='text-foreground min-w-0 truncate text-sm font-semibold'>
      {props.children}
    </span>
  )
}

function CatalogInfoCell(props: { label: string; children: React.ReactNode }) {
  return (
    <div className='bg-card flex min-w-0 flex-col gap-1 px-3 py-2.5'>
      <span className='text-muted-foreground text-[10px] font-medium tracking-wider uppercase'>
        {props.label}
      </span>
      {props.children}
    </div>
  )
}

function formatProviderPrice(value: number | null | undefined): string {
  return value == null
    ? '—'
    : formatBillingCurrencyFromUSD(value, {
        digitsLarge: 4,
        digitsSmall: 6,
        abbreviate: false,
      })
}

function formatProviderCachePrice(
  provider: PricingProvider,
  t: (key: string) => string
): string {
  if (provider.pricing?.cache_read_price != null) {
    return formatBillingCurrencyFromUSD(provider.pricing.cache_read_price)
  }
  return t('To be added')
}

function formatProviderCapability(
  value: boolean | null | undefined,
  t: (key: string) => string
): string {
  if (value == null) return t('To be added')
  return value ? t('Supported') : t('Not supported')
}

function formatProviderEffectiveAt(
  value: number | undefined,
  t: (key: string) => string
): string {
  if (!value) return t('To be added')
  const date = new Date(value * 1000)
  return Number.isNaN(date.getTime()) ? t('To be added') : date.toLocaleString()
}

function ModalityLabels(props: { items: string[] }) {
  const { t } = useTranslation()
  if (props.items.length === 0) return null

  return (
    <span className='inline-flex items-center gap-1 align-middle'>
      {props.items.map((item) => (
        <span key={item} className='font-medium'>
          {t(MODALITY_LABEL_KEYS[item] ?? item)}
        </span>
      ))}
    </span>
  )
}

function ModelBackendQuickStats(props: { model: PricingModel }) {
  const { t } = useTranslation()
  const model = props.model
  const inputModalities = normalizeCatalogItems(model.input_modalities)
  const outputModalities = normalizeCatalogItems(model.output_modalities)
  const contextLength = model.context_length ?? 0
  const maxOutput = model.max_output_tokens ?? 0
  const knowledgeCutoff = formatCatalogYearMonth(model.knowledge_cutoff)
  const releaseDate = formatCatalogYearMonth(model.release_date)

  const stats: {
    key: string
    icon: React.ComponentType<{ className?: string }>
    label: string
    value: React.ReactNode
    hint?: string
  }[] = []

  if (contextLength > 0) {
    stats.push({
      key: 'context',
      icon: Layers,
      label: t('Context'),
      value: formatCatalogTokenCount(contextLength),
      hint: t('Maximum input window'),
    })
  }

  if (maxOutput > 0) {
    stats.push({
      key: 'max-output',
      icon: Maximize2,
      label: t('Max output'),
      value: formatCatalogTokenCount(maxOutput),
      hint: t('Maximum tokens per response'),
    })
  }

  if (inputModalities.length > 0 || outputModalities.length > 0) {
    stats.push({
      key: 'modalities',
      icon: FileText,
      label: t('Modalities'),
      value: (
        <span className='inline-flex items-center gap-1'>
          <ModalityLabels items={inputModalities} />
          {inputModalities.length > 0 && outputModalities.length > 0 && (
            <span className='text-muted-foreground/40'>→</span>
          )}
          <ModalityLabels items={outputModalities} />
        </span>
      ),
    })
  }

  if (knowledgeCutoff) {
    stats.push({
      key: 'knowledge',
      icon: Sparkles,
      label: t('Knowledge cutoff'),
      value: knowledgeCutoff,
    })
  }

  if (releaseDate) {
    stats.push({
      key: 'release',
      icon: CalendarClock,
      label: t('Released'),
      value: releaseDate,
    })
  }

  if (stats.length === 0) return null

  return (
    <div className='bg-muted/20 grid grid-cols-2 gap-px overflow-hidden rounded-lg border @md/details:grid-cols-3 @2xl/details:grid-cols-5'>
      {stats.map((stat) => {
        const Icon = stat.icon
        return (
          <div
            key={stat.key}
            className='bg-background flex min-w-0 flex-col gap-0.5 px-3 py-2.5'
          >
            <span className='text-muted-foreground inline-flex min-w-0 items-center gap-1 text-[10px] font-medium tracking-wider uppercase'>
              <Icon className='size-3 shrink-0' />
              <span className='truncate'>{stat.label}</span>
            </span>
            <span className='text-foreground truncate text-sm font-semibold tabular-nums'>
              {stat.value}
            </span>
            {stat.hint && (
              <span className='text-muted-foreground/60 truncate text-[10px]'>
                {stat.hint}
              </span>
            )}
          </div>
        )
      })}
    </div>
  )
}

function ModelBackendSignalsSection(props: { model: PricingModel }) {
  const { t } = useTranslation()
  const capabilities = normalizeCatalogItems(props.model.capabilities)
  const inputModalities = normalizeCatalogItems(props.model.input_modalities)
  const outputModalities = normalizeCatalogItems(props.model.output_modalities)

  if (
    capabilities.length === 0 &&
    inputModalities.length === 0 &&
    outputModalities.length === 0
  ) {
    return null
  }

  return (
    <section>
      <SectionTitle>
        {t('Capabilities')} / {t('Supported modalities')}
      </SectionTitle>
      <div className='grid gap-3 rounded-xl border p-3 @2xl/details:grid-cols-[minmax(0,1.5fr)_minmax(260px,1fr)]'>
        {capabilities.length > 0 ? (
          <CatalogPillList
            items={capabilities.map((capability) =>
              t(
                CAPABILITY_LABEL_KEYS[capability as ModelCapability] ??
                  capability
              )
            )}
          />
        ) : (
          <div />
        )}
        {(inputModalities.length > 0 || outputModalities.length > 0) && (
          <div className='grid gap-2 sm:grid-cols-2'>
            {inputModalities.length > 0 && (
              <div className='flex items-center justify-between gap-3 rounded-lg border px-3 py-2'>
                <span className='text-muted-foreground text-xs font-medium'>
                  {t('Input')}
                </span>
                <CatalogTextValue>
                  <ModalityLabels items={inputModalities} />
                </CatalogTextValue>
              </div>
            )}
            {outputModalities.length > 0 && (
              <div className='flex items-center justify-between gap-3 rounded-lg border px-3 py-2'>
                <span className='text-muted-foreground text-xs font-medium'>
                  {t('Output')}
                </span>
                <CatalogTextValue>
                  <ModalityLabels items={outputModalities} />
                </CatalogTextValue>
              </div>
            )}
          </div>
        )}
      </div>
    </section>
  )
}

function ModelBackendProviderSection(props: { model: PricingModel }) {
  const { t } = useTranslation()
  const model = props.model
  const groups = normalizeCatalogItems(model.enable_groups)
  const endpoints = normalizeCatalogItems(model.supported_endpoint_types)
  const tags = parseTags(model.tags)
  const cells: React.ReactElement[] = []

  if (model.vendor_name) {
    cells.push(
      <CatalogInfoCell key='provider' label={t('Provider')}>
        <CatalogTextValue>{model.vendor_name}</CatalogTextValue>
      </CatalogInfoCell>
    )
  }

  cells.push(
    <CatalogInfoCell key='type' label={t('Type')}>
      <ModelBillingModeBadge model={model} />
    </CatalogInfoCell>
  )

  if (groups.length > 0) {
    cells.push(
      <CatalogInfoCell key='groups' label={t('Groups')}>
        <CatalogPillList items={groups} />
      </CatalogInfoCell>
    )
  }

  if (endpoints.length > 0) {
    cells.push(
      <CatalogInfoCell key='endpoints' label={t('Endpoints')}>
        <CatalogPillList items={endpoints} />
      </CatalogInfoCell>
    )
  }

  if (tags.length > 0) {
    cells.push(
      <CatalogInfoCell key='tags' label={t('Tags')}>
        <CatalogPillList items={tags} />
      </CatalogInfoCell>
    )
  }

  if (model.parameter_count) {
    cells.push(
      <CatalogInfoCell key='parameters' label={t('Parameters')}>
        <CatalogTextValue>{model.parameter_count}</CatalogTextValue>
      </CatalogInfoCell>
    )
  }

  if (cells.length === 0) return null

  return (
    <section>
      <SectionTitle>{t('Model')}</SectionTitle>
      <div className='border-border/60 bg-border/60 grid grid-cols-1 gap-px overflow-hidden rounded-lg border sm:grid-cols-2'>
        {cells.map((cell, index) => (
          <div
            key={cell.key ?? index}
            className={cn(
              'min-w-0',
              cells.length % 2 === 1 && index === cells.length - 1
                ? 'sm:col-span-2'
                : undefined
            )}
          >
            {cell}
          </div>
        ))}
      </div>
    </section>
  )
}

function ModelProvidersSection(props: {
  model: PricingModel
  onProviderClick?: (provider: PricingProvider) => void
}) {
  const { t } = useTranslation()
  const providers = props.model.providers ?? []
  if (providers.length === 0) return null

  return (
    <section>
      <SectionTitle>{t('Providers')}</SectionTitle>
      <div className='grid gap-2 sm:grid-cols-2'>
        {providers.map((provider) => (
          <button
            key={provider.slug}
            type='button'
            onClick={() => props.onProviderClick?.(provider)}
            className='bg-card flex items-center justify-between gap-3 rounded-lg border px-3 py-2.5'
          >
            <div className='flex min-w-0 items-center gap-2'>
              <div className='bg-muted flex size-7 shrink-0 items-center justify-center rounded-md'>
                {provider.icon ? (
                  getLobeIcon(provider.icon, 18)
                ) : (
                  <span className='text-muted-foreground text-xs font-semibold'>
                    {provider.name.charAt(0).toUpperCase()}
                  </span>
                )}
              </div>
              <div className='min-w-0'>
                <div className='truncate text-sm font-semibold'>
                  {provider.name}
                </div>
                <div className='text-muted-foreground text-xs'>
                  {provider.slug}
                </div>
              </div>
            </div>
            <span className='text-muted-foreground shrink-0 rounded-md border px-2 py-1 font-mono text-[11px]'>
              {provider.available ? t('Available') : t('Unavailable')}
            </span>
          </button>
        ))}
      </div>
    </section>
  )
}

type ModelProviderDetailsDrawerProps = {
  model: PricingModel
  provider: PricingProvider | null
  open: boolean
  onOpenChange: (open: boolean) => void
  groupRatio?: Record<string, number>
  usableGroup?: Record<string, { desc: string; ratio: number }>
  autoGroups?: string[]
  priceRate?: number
  usdExchangeRate?: number
  tokenUnit?: TokenUnit
  showRechargePrice?: boolean
}

function ModelProviderDetailsDrawer(props: ModelProviderDetailsDrawerProps) {
  const { t } = useTranslation()
  const detailQuery = useQuery({
    queryKey: ['model-square-provider', props.model.id, props.provider?.slug],
    queryFn: () =>
      getModelSquareProviderDetail(
        String(props.model.id),
        props.provider?.slug || ''
      ),
    enabled:
      props.open && Boolean(props.provider?.slug) && Number(props.model.id) > 0,
    staleTime: 60 * 1000,
  })
  const detail = detailQuery.data?.data
  const model = detail?.model ?? props.model
  const provider = detail?.provider ?? props.provider
  if (!provider) return null
  const groups = detail?.groups ?? model.enable_groups ?? []
  const endpoints = detail?.endpoints ?? model.supported_endpoint_types ?? []
  const groupRatio = props.groupRatio ?? {}
  const usableGroup = props.usableGroup ?? {}
  const autoGroups = props.autoGroups ?? []
  const priceRate = props.priceRate ?? 1
  const usdExchangeRate = props.usdExchangeRate ?? 1
  const tokenUnit = props.tokenUnit ?? DEFAULT_TOKEN_UNIT
  const showRechargePrice = props.showRechargePrice ?? false
  const modelIconKey = model.icon || model.vendor_icon
  const modelIcon = modelIconKey ? getLobeIcon(modelIconKey, 24) : null
  const routingValue = JSON.stringify(
    {
      model: model.model_name,
      provider: {
        order: [provider.slug],
        allow_fallbacks: false,
      },
    },
    null,
    2
  )
  const providerPricing = provider.pricing

  return (
    <Sheet open={props.open} onOpenChange={props.onOpenChange}>
      <SheetContent
        side='right'
        className={sideDrawerContentClassName(
          'sm:max-w-2xl lg:max-w-3xl xl:max-w-4xl'
        )}
      >
        <SheetHeader className='border-b pb-4'>
          <div className='flex items-start gap-3'>
            <div className='bg-muted flex size-10 shrink-0 items-center justify-center rounded-lg'>
              {modelIcon || (
                <span className='font-mono text-sm font-bold'>
                  {model.model_name.charAt(0).toUpperCase()}
                </span>
              )}
            </div>
            <div className='min-w-0'>
              <SheetTitle className='whitespace-normal font-mono break-all'>
                {model.model_name}
              </SheetTitle>
              <SheetDescription className='mt-1 flex flex-wrap items-center gap-1.5 text-xs'>
                <span className='inline-flex items-center gap-1'>
                  {provider.icon ? getLobeIcon(provider.icon, 14) : null}
                  {provider.name}
                </span>
                <span className='text-muted-foreground/40'>·</span>
                <ModelBillingModeBadge model={model} />
                <span className='text-muted-foreground/40'>·</span>
                <code className='font-mono'>{provider.slug}</code>
              </SheetDescription>
            </div>
          </div>
        </SheetHeader>
        <div className='flex-1 overflow-y-auto px-4 pb-6 sm:px-6'>
          <Tabs defaultValue='overview' className='gap-4'>
            <TabsList className='bg-muted/60 grid w-full min-w-0 grid-cols-2 gap-1 overflow-hidden rounded-lg p-1'>
              {(['overview', 'performance'] as const).map((value) => {
                const Icon = TAB_META[value].icon
                return (
                  <TabsTrigger
                    key={value}
                    value={value}
                    className='h-8 min-w-0 gap-1.5 rounded-md px-2 text-xs sm:px-3 sm:text-sm'
                  >
                    <Icon className='size-3.5' />
                    <span className='truncate'>
                      {t(TAB_META[value].labelKey)}
                    </span>
                  </TabsTrigger>
                )
              })}
            </TabsList>

            <TabsContent value='overview' className='space-y-6 outline-none'>
              <section className='space-y-2'>
                <OverviewSummaryGrid model={model} />
                <p className='text-muted-foreground text-[11px]'>
                  {t('Model-wide metrics')} ·{' '}
                  {t('Provider-specific metrics are not available yet')}
                </p>
              </section>

              <section className='bg-card/60 space-y-5 rounded-xl border p-4 shadow-sm'>
                <div className='flex items-center justify-between gap-3'>
                  <SectionTitle>{t('Pricing')}</SectionTitle>
                  <span className='text-muted-foreground text-[11px]'>
                    {t('Platform billing price')}
                  </span>
                </div>
                {providerPricing ? (
                  <div className='grid gap-2 sm:grid-cols-2'>
                    <CatalogInfoCell label={`${t('Input')} / 1M`}>
                      <CatalogTextValue>
                        {formatProviderPrice(providerPricing.input_price)}
                      </CatalogTextValue>
                    </CatalogInfoCell>
                    <CatalogInfoCell label={`${t('Output')} / 1M`}>
                      <CatalogTextValue>
                        {formatProviderPrice(providerPricing.output_price)}
                      </CatalogTextValue>
                    </CatalogInfoCell>
                    {providerPricing.cache_read_price != null && (
                      <CatalogInfoCell label={`${t('Cache')} / 1M`}>
                        <CatalogTextValue>
                          {formatProviderPrice(providerPricing.cache_read_price)}
                        </CatalogTextValue>
                      </CatalogInfoCell>
                    )}
                    {providerPricing.cache_write_price != null && (
                      <CatalogInfoCell label={`${t('Cache Write')} / 1M`}>
                        <CatalogTextValue>
                          {formatProviderPrice(providerPricing.cache_write_price)}
                        </CatalogTextValue>
                      </CatalogInfoCell>
                    )}
                  </div>
                ) : (
                  <PriceSection
                    model={model}
                    priceRate={priceRate}
                    usdExchangeRate={usdExchangeRate}
                    tokenUnit={tokenUnit}
                    showRechargePrice={showRechargePrice}
                  />
                )}
                {isDynamicPricingModel(model) && (
                  <DynamicPricingBreakdown billingExpr={model.billing_expr} />
                )}
                <GroupPricingSection
                  model={model}
                  groupRatio={groupRatio}
                  usableGroup={usableGroup}
                  autoGroups={autoGroups}
                  priceRate={priceRate}
                  usdExchangeRate={usdExchangeRate}
                  tokenUnit={tokenUnit}
                  showRechargePrice={showRechargePrice}
                />
              </section>

              <ModelBackendDetailsSection model={model} />

              <section>
                <SectionTitle>{t('Provider info')}</SectionTitle>
                <div className='border-border/60 bg-border/60 grid grid-cols-1 gap-px overflow-hidden rounded-lg border sm:grid-cols-2'>
                  <CatalogInfoCell label={t('Provider')}>
                    <CatalogTextValue>{provider.name}</CatalogTextValue>
                  </CatalogInfoCell>
                  <CatalogInfoCell label={t('Provider slug')}>
                    <CatalogTextValue>{provider.slug}</CatalogTextValue>
                  </CatalogInfoCell>
                  <CatalogInfoCell label={t('Status')}>
                    <CatalogTextValue>
                      {provider.available ? t('Available') : t('Unavailable')}
                    </CatalogTextValue>
                  </CatalogInfoCell>
                  <CatalogInfoCell label={t('Groups')}>
                    <CatalogPillList items={groups} />
                  </CatalogInfoCell>
                  <CatalogInfoCell label={t('Endpoints')}>
                    <CatalogPillList items={endpoints} />
                  </CatalogInfoCell>
                  {provider.website_url && (
                    <CatalogInfoCell label={t('Website URL')}>
                      <a
                        href={provider.website_url}
                        target='_blank'
                        rel='noreferrer'
                        className='text-primary truncate text-sm font-medium hover:underline'
                      >
                        {provider.website_url}
                      </a>
                    </CatalogInfoCell>
                  )}
                  {provider.status_page_url && (
                    <CatalogInfoCell label={t('Status page URL')}>
                      <a
                        href={provider.status_page_url}
                        target='_blank'
                        rel='noreferrer'
                        className='text-primary truncate text-sm font-medium hover:underline'
                      >
                        {provider.status_page_url}
                      </a>
                    </CatalogInfoCell>
                  )}
                </div>
              </section>

              <ProviderModelMetadataSection
                metadata={provider.metadata}
                t={t}
              />

              <section>
                <SectionTitle>{t('Routing')}</SectionTitle>
                <div className='bg-muted/20 rounded-lg border p-3'>
                  <div className='mb-2 text-xs font-medium'>
                    {t('Selected provider routing')}
                  </div>
                  <pre className='bg-background overflow-x-auto rounded-md border p-3 font-mono text-xs leading-relaxed'>
                    {routingValue}
                  </pre>
                  <CopyButton
                    value={routingValue}
                    variant='outline'
                    size='sm'
                    className='mt-3'
                  >
                    {t('Copy')}
                  </CopyButton>
                </div>
              </section>
            </TabsContent>

            <TabsContent
              value='performance'
              className='space-y-6 outline-none'
            >
              <ProviderPerformanceTab provider={provider} />
            </TabsContent>
          </Tabs>
        </div>
      </SheetContent>
    </Sheet>
  )
}

function ProviderPerformanceTab(props: { provider: PricingProvider }) {
  const { t } = useTranslation()
  const metrics = [
    { label: 'TPS', icon: Timer },
    { label: t('Average latency'), icon: Timer },
    { label: t('Success rate'), icon: HeartPulse },
    { label: t('Uptime'), icon: HeartPulse },
  ]

  return (
    <div className='space-y-6'>
      <section className='space-y-2'>
        <SectionTitle>{t('Performance')}</SectionTitle>
        <p className='text-muted-foreground text-xs'>
          {t('Provider performance data is not available yet')}
        </p>
      </section>

      <section className='grid gap-3 sm:grid-cols-3'>
        {metrics.map((metric) => {
          const Icon = metric.icon
          return (
            <div key={metric.label} className='bg-card rounded-xl border p-4'>
              <div className='text-muted-foreground flex items-center gap-1.5 text-xs'>
                <Icon className='size-3.5' />
                {metric.label}
              </div>
              <div className='mt-2 font-mono text-lg font-semibold'>
                {t('To be added')}
              </div>
            </div>
          )
        })}
      </section>

      <section className='bg-card rounded-xl border p-4'>
        <div className='flex items-center justify-between gap-3'>
          <div>
          <h3 className='text-sm font-semibold'>{t('Performance trend')}</h3>
            <p className='text-muted-foreground mt-1 text-xs'>
              {props.provider.name} · {t('Provider-specific metrics are not available yet')}
            </p>
          </div>
          <HeartPulse className='text-muted-foreground size-4' />
        </div>
        <div className='bg-muted/20 text-muted-foreground mt-4 flex min-h-48 items-center justify-center rounded-lg border border-dashed text-sm'>
          {t('To be added')}
        </div>
      </section>
    </div>
  )
}

function ProviderModelMetadataSection(props: {
  metadata?: PricingProviderMetadata
  t: (key: string) => string
}) {
  const metadata = props.metadata
  if (!metadata) return null

  const cells: React.ReactElement[] = []
  const addTextCell = (key: string, label: string, value?: string | number) => {
    if (value == null || value === '') return
    cells.push(
      <CatalogInfoCell key={key} label={label}>
        <CatalogTextValue>{value}</CatalogTextValue>
      </CatalogInfoCell>
    )
  }

  addTextCell('model-name', props.t('Provider model name'), metadata.model_name)
  addTextCell(
    'context-length',
    props.t('Context length'),
    metadata.context_length
      ? formatCatalogTokenCount(metadata.context_length)
      : undefined
  )
  addTextCell(
    'max-output',
    props.t('Max output tokens'),
    metadata.max_output_tokens
  )
  addTextCell('region', props.t('Region'), metadata.region)
  addTextCell('precision', props.t('Precision'), metadata.precision)
  addTextCell('quantization', props.t('Quantization'), metadata.quantization)
  if (metadata.stream_cancellation != null) {
    addTextCell(
      'stream-cancellation',
      props.t('Stream cancellation support'),
      formatProviderCapability(metadata.stream_cancellation, props.t)
    )
  }
  if (metadata.free != null) {
    addTextCell(
      'free',
      props.t('Free capability'),
      formatProviderCapability(metadata.free, props.t)
    )
  }
  if (metadata.batch != null) {
    addTextCell(
      'batch',
      props.t('Batch capability'),
      formatProviderCapability(metadata.batch, props.t)
    )
  }
  if (metadata.supported_parameters?.length) {
    cells.push(
      <CatalogInfoCell
        key='supported-parameters'
        label={props.t('Supported parameters')}
      >
        <CatalogPillList items={metadata.supported_parameters} />
      </CatalogInfoCell>
    )
  }
  if (metadata.effective_at) {
    addTextCell(
      'effective-at',
      props.t('Effective time'),
      formatProviderEffectiveAt(metadata.effective_at, props.t)
    )
  }
  if (metadata.source_url) {
    cells.push(
      <CatalogInfoCell key='source-url' label={props.t('Source URL')}>
        <a
          href={metadata.source_url}
          target='_blank'
          rel='noreferrer'
          className='text-primary truncate text-sm font-medium hover:underline'
        >
          {metadata.source_url}
        </a>
      </CatalogInfoCell>
    )
  }
  if (cells.length === 0) return null

  return (
    <section>
      <SectionTitle>{props.t('Provider metadata')}</SectionTitle>
      <div className='border-border/60 bg-border/60 grid grid-cols-1 gap-px overflow-hidden rounded-lg border sm:grid-cols-2'>
        {cells.map((cell) => cell)}
      </div>
    </section>
  )
}

function ModelBackendDetailsSection(props: { model: PricingModel }) {
  return (
    <>
      <ModelBackendQuickStats model={props.model} />
      <ModelBackendSignalsSection model={props.model} />
      <ModelBackendProviderSection model={props.model} />
    </>
  )
}

function ModelSquareMetric(props: {
  label: string
  value: React.ReactNode
  icon: React.ComponentType<{ className?: string }>
  hint?: React.ReactNode
}) {
  const Icon = props.icon
  return (
    <div className='bg-card rounded-xl border px-4 py-3'>
      <div className='text-muted-foreground flex items-center gap-1.5 text-[11px] font-medium tracking-wider uppercase'>
        <Icon className='size-3.5' />
        {props.label}
      </div>
      <div className='text-foreground mt-1.5 truncate font-mono text-lg font-semibold tabular-nums'>
        {props.value}
      </div>
      {props.hint && (
        <div className='text-muted-foreground mt-1 truncate text-[11px]'>
          {props.hint}
        </div>
      )}
    </div>
  )
}

function ModelSquareProviderTable(props: {
  model: PricingModel
  providers: PricingProvider[]
  primaryProviderSlug?: string
  onProviderClick: (provider: PricingProvider) => void
  onMoveProvider?: (providerSlug: string, direction: -1 | 1) => void
}) {
  const { t } = useTranslation()
  const providers = props.providers
  return (
    <div className='overflow-x-auto rounded-xl border'>
      <table className='w-full min-w-[640px] text-sm'>
        <thead className='bg-muted/40 text-muted-foreground text-xs'>
          <tr>
            <th className='px-4 py-3 text-left font-medium'>{t('Provider')}</th>
            <th className='px-4 py-3 text-left font-medium'>{t('Status')}</th>
            <th className='px-4 py-3 text-right font-medium'>{t('Input')}</th>
            <th className='px-4 py-3 text-right font-medium'>{t('Output')}</th>
            <th className='px-4 py-3 text-right font-medium'>{t('Cache')}</th>
            <th className='px-4 py-3' />
          </tr>
        </thead>
        <tbody className='divide-y'>
          {providers.length > 0 ? (
            providers.map((provider) => (
              <tr
                key={provider.slug}
                className={cn(
                  'hover:bg-muted/30 cursor-pointer transition-colors',
                  provider.slug === props.primaryProviderSlug &&
                    'bg-primary/5'
                )}
                onClick={() => props.onProviderClick(provider)}
              >
                <td className='px-4 py-3'>
                  <div className='flex items-center gap-2'>
                    <div className='bg-muted flex size-7 shrink-0 items-center justify-center rounded-md'>
                      {provider.icon ? (
                        getLobeIcon(provider.icon, 18)
                      ) : (
                        <span className='text-muted-foreground text-xs font-semibold'>
                          {provider.name.charAt(0).toUpperCase()}
                        </span>
                      )}
                    </div>
                    <div className='min-w-0'>
                      <div className='font-medium'>{provider.name}</div>
                      <div className='text-muted-foreground font-mono text-xs'>
                        {provider.slug}
                      </div>
                    </div>
                  </div>
                </td>
                <td className='px-4 py-3'>
                  <span
                    className={cn(
                      'inline-flex items-center rounded-full px-2 py-1 text-xs font-medium',
                      provider.available
                        ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'
                        : 'bg-muted text-muted-foreground'
                    )}
                  >
                    {provider.available ? t('Available') : t('Unavailable')}
                  </span>
                </td>
                <td className='px-4 py-3 text-right font-mono tabular-nums'>
                  {provider.pricing
                    ? formatBillingCurrencyFromUSD(provider.pricing.input_price)
                    : t('To be added')}
                </td>
                <td className='px-4 py-3 text-right font-mono tabular-nums'>
                  {provider.pricing
                    ? formatBillingCurrencyFromUSD(provider.pricing.output_price)
                    : t('To be added')}
                </td>
                <td className='text-muted-foreground px-4 py-3 text-right font-mono tabular-nums'>
                  {formatProviderCachePrice(provider, t)}
                </td>
                <td className='px-4 py-3 text-right'>
                  <div className='flex items-center justify-end gap-2'>
                    {provider.website_url && (
                      <a
                        href={provider.website_url}
                        target='_blank'
                        rel='noreferrer'
                        className='text-muted-foreground hover:text-foreground text-xs hover:underline'
                        onClick={(event) => event.stopPropagation()}
                      >
                        {t('Website')}
                      </a>
                    )}
                    <div className='flex items-center justify-end gap-1'>
                      <Button
                        type='button'
                        variant='ghost'
                        size='icon-sm'
                        aria-label={t('Move {{group}} up', {
                          group: provider.name,
                        })}
                        disabled={providers.indexOf(provider) === 0}
                        onClick={(event) => {
                          event.stopPropagation()
                          props.onMoveProvider?.(provider.slug, -1)
                        }}
                      >
                        <ArrowUp className='size-3.5' />
                      </Button>
                      <Button
                        type='button'
                        variant='ghost'
                        size='icon-sm'
                        aria-label={t('Move {{group}} down', {
                          group: provider.name,
                        })}
                        disabled={providers.indexOf(provider) === providers.length - 1}
                        onClick={(event) => {
                          event.stopPropagation()
                          props.onMoveProvider?.(provider.slug, 1)
                        }}
                      >
                        <ArrowDown className='size-3.5' />
                      </Button>
                      <ChevronRight className='text-muted-foreground size-4' />
                    </div>
                  </div>
                </td>
              </tr>
            ))
          ) : (
            <tr>
              <td
                colSpan={6}
                className='text-muted-foreground px-4 py-8 text-center text-sm'
              >
                {t('No providers available')}
              </td>
            </tr>
          )}
        </tbody>
      </table>
    </div>
  )
}

type ProviderRoutingMode = 'free' | 'standard' | 'nitro' | 'exact'

function getRoutedProviders(
  providers: PricingProvider[],
  mode: ProviderRoutingMode
): PricingProvider[] {
  const available = providers.filter((provider) => provider.available)
  if (mode === 'free') {
    return available.filter(isFreeProvider)
  }

  if (mode === 'exact') {
    return available
  }

  if (mode === 'nitro') {
    // Provider-level performance data is not available yet, so preserve the
    // backend provider order instead of presenting price sorting as Nitro.
    return available
  }

  return sortProvidersForStandard(available)
}

function getRoutingModeDescription(
  mode: ProviderRoutingMode,
  t: (key: string) => string
): string {
  if (mode === 'free') return t('Only providers with zero configured price')
  if (mode === 'nitro') {
    return t('Provider performance data is not available yet; using provider order')
  }
  if (mode === 'exact') {
    return t('Quality data is not available yet; using provider order')
  }
  return t('Balanced sorting by availability and configured price')
}

function getRoutingModeLabel(
  mode: ProviderRoutingMode,
  t: (key: string) => string
): string {
  if (mode === 'free') return t('Free routing mode')
  if (mode === 'nitro') return t('Nitro')
  if (mode === 'exact') return t('Exact routing mode')
  return t('Standard')
}

function ProviderRoutingControls(props: {
  mode: ProviderRoutingMode
  onModeChange: (mode: ProviderRoutingMode) => void
  allowFallbacks: boolean
  onAllowFallbacksChange: (value: boolean) => void
}) {
  const { t } = useTranslation()
  return (
    <div className='bg-muted/20 flex flex-col gap-3 rounded-lg border p-3 sm:flex-row sm:items-center sm:justify-between'>
      <div className='flex flex-wrap items-center gap-2'>
        <span className='text-muted-foreground text-xs'>{t('Routing mode')}</span>
        <Select
          value={props.mode}
          onValueChange={(value) =>
            props.onModeChange(value as ProviderRoutingMode)
          }
        >
          <SelectTrigger className='h-8 w-32 text-xs'>
            <SelectValue>{getRoutingModeLabel(props.mode, t)}</SelectValue>
          </SelectTrigger>
          <SelectContent>
            <SelectItem value='free'>
              <Gift className='size-3.5' />
              {t('Free routing mode')}
            </SelectItem>
            <SelectItem value='standard'>
              <ShieldCheck className='size-3.5' />
              {t('Standard')}
            </SelectItem>
            <SelectItem value='nitro'>
              <Zap className='size-3.5' />
              {t('Nitro')}
            </SelectItem>
            <SelectItem value='exact'>
              <ShieldCheck className='size-3.5' />
              {t('Exact routing mode')}
            </SelectItem>
          </SelectContent>
        </Select>
        <label className='text-muted-foreground inline-flex items-center gap-2 text-xs'>
          <Switch
            size='sm'
            checked={props.allowFallbacks}
            onCheckedChange={props.onAllowFallbacksChange}
          />
          {t('Fallback')}
        </label>
      </div>
      <span className='text-muted-foreground text-xs'>
        {getRoutingModeDescription(props.mode, t)}
      </span>
    </div>
  )
}

export function ModelSquareDetailPage(props: ModelDetailsContentProps) {
  const { t } = useTranslation()
  const [selectedProvider, setSelectedProvider] =
    useState<PricingProvider | null>(null)
  const [routingMode, setRoutingMode] =
    useState<ProviderRoutingMode>('standard')
  const [allowFallbacks, setAllowFallbacks] = useState(true)
  const [providerOrder, setProviderOrder] = useState<string[]>([])
  const model = props.model
  const modelIcon = model.icon || model.vendor_icon
  const providers = model.providers ?? EMPTY_PROVIDERS
  const sortedProviders = useMemo(
    () => getRoutedProviders(providers, routingMode),
    [providers, routingMode]
  )
  useEffect(() => {
    setProviderOrder((current) => {
      const available = new Set(sortedProviders.map((provider) => provider.slug))
      const retained = current.filter((slug) => available.has(slug))
      const appended = sortedProviders
        .map((provider) => provider.slug)
        .filter((slug) => !retained.includes(slug))
      const next = [...retained, ...appended]
      if (
        next.length === current.length &&
        next.every((slug, index) => slug === current[index])
      ) {
        return current
      }
      return next
    })
  }, [sortedProviders])
  const routedProviders = useMemo(() => {
    const providersBySlug = new Map(
      sortedProviders.map((provider) => [provider.slug, provider])
    )
    const ordered = providerOrder
      .map((slug) => providersBySlug.get(slug))
      .filter((provider): provider is PricingProvider => Boolean(provider))
    const orderedSlugs = new Set(ordered.map((provider) => provider.slug))
    return [
      ...ordered,
      ...sortedProviders.filter((provider) => !orderedSlugs.has(provider.slug)),
    ]
  }, [providerOrder, sortedProviders])
  const moveProvider = useCallback((providerSlug: string, direction: -1 | 1) => {
    setProviderOrder((current) => {
      const index = current.indexOf(providerSlug)
      const target = index + direction
      if (index < 0 || target < 0 || target >= current.length) return current
      const next = [...current]
      const moved = next[index]
      next[index] = next[target]
      next[target] = moved
      return next
    })
  }, [])
  const primaryProvider = routedProviders[0]
  const routingValue = useMemo(
    () =>
      JSON.stringify(
        {
          model: model.model_name,
          provider: {
            order: routedProviders.map((provider) => provider.slug),
            allow_fallbacks: allowFallbacks,
          },
        },
        null,
        2
      ),
    [allowFallbacks, model.model_name, routedProviders]
  )
  const inputPrice = primaryProvider?.pricing
    ? formatProviderPrice(primaryProvider.pricing.input_price)
    : formatPrice(model, 'input', 'M', false, 1, 1)
  const outputPrice = primaryProvider?.pricing
    ? formatProviderPrice(primaryProvider.pricing.output_price)
    : formatPrice(model, 'output', 'M', false, 1, 1)
  let priceSource = t('Platform default price')
  if (primaryProvider?.pricing) {
    priceSource = `${primaryProvider.name} · ${t('Provider price')}`
  } else if (routedProviders.length === 0) {
    priceSource = t('No matching providers')
  }
  const displayInputPrice =
    routedProviders.length === 0 ? '—' : inputPrice
  const displayOutputPrice =
    routedProviders.length === 0 ? '—' : outputPrice
  const context = model.context_length
    ? formatCatalogTokenCount(model.context_length)
    : '—'
  const releaseDate = formatCatalogYearMonth(model.release_date) || '—'
  const inputModalities = normalizeCatalogItems(model.input_modalities)
  const outputModalities = normalizeCatalogItems(model.output_modalities)
  const hasModalities =
    inputModalities.length > 0 || outputModalities.length > 0

  return (
    <div className='space-y-8 pb-10'>
      <header className='border-b pb-6'>
        <div className='flex flex-wrap items-start justify-between gap-4'>
          <div className='flex min-w-0 items-start gap-3'>
            <div className='bg-muted flex size-11 shrink-0 items-center justify-center rounded-xl'>
              {(modelIcon ? getLobeIcon(modelIcon, 28) : null) || (
                <span className='font-mono text-lg font-bold'>
                  {model.model_name.charAt(0).toUpperCase()}
                </span>
              )}
            </div>
            <div className='min-w-0'>
              <div className='text-muted-foreground mb-1 text-sm'>
                {model.vendor_name || t('Model')}
              </div>
              <h1 className='whitespace-normal text-2xl font-bold tracking-tight break-all sm:text-3xl'>
                {model.model_name}
              </h1>
              <div className='mt-2 flex flex-wrap items-center gap-2'>
                <code className='text-muted-foreground text-sm'>
                  {model.model_name}
                </code>
                <CopyButton
                  value={model.model_name}
                  size='icon'
                  className='size-7'
                  iconClassName='size-3.5'
                  aria-label={t('Copy model name')}
                />
              </div>
            </div>
          </div>
          <Button variant='outline' size='sm' className='gap-1.5'>
            <Code2 className='size-4' />
            {t('API')}
          </Button>
        </div>
        {model.description && (
          <p className='text-muted-foreground mt-5 max-w-4xl text-sm leading-6'>
            {model.description}
          </p>
        )}
        <div className='mt-5 grid grid-cols-2 gap-3 xl:grid-cols-5'>
          <ModelSquareMetric
            label={t('Modalities')}
            value={
              hasModalities ? (
                <span className='flex items-center gap-2 text-sm'>
                  {inputModalities.length > 0 ? (
                    <ModalityLabels items={inputModalities} />
                  ) : (
                    <span>—</span>
                  )}
                  <span className='text-muted-foreground'>→</span>
                  {outputModalities.length > 0 ? (
                    <ModalityLabels items={outputModalities} />
                  ) : (
                    <span>—</span>
                  )}
                </span>
              ) : (
                t('To be added')
              )
            }
            icon={Layers}
          />
              <ModelSquareMetric
                label={t('Input')}
                value={`${displayInputPrice} / 1M`}
                icon={Zap}
                hint={priceSource}
          />
              <ModelSquareMetric
                label={t('Output')}
                value={`${displayOutputPrice} / 1M`}
            icon={Zap}
            hint={priceSource}
          />
          <ModelSquareMetric
            label={t('Context')}
            value={context}
            icon={Layers}
          />
          <ModelSquareMetric
            label={t('Released')}
            value={releaseDate}
            icon={CalendarClock}
          />
        </div>
      </header>

      <div className='grid gap-8 lg:grid-cols-[180px_minmax(0,1fr)]'>
        <aside className='lg:sticky lg:top-4 lg:self-start'>
          <nav className='flex gap-1 overflow-x-auto lg:flex-col'>
            {[
              ['providers', t('Providers'), Info],
              ['api', t('API'), Code2],
            ].map(([id, label, Icon]) => {
              const NavIcon = Icon as React.ComponentType<{
                className?: string
              }>
              return (
                <a
                  key={id as string}
                  href={`#${id}`}
                  className='text-muted-foreground hover:bg-muted hover:text-foreground flex shrink-0 items-center gap-2 rounded-lg px-3 py-2 text-sm transition-colors'
                >
                  <NavIcon className='size-4' />
                  {label as string}
                </a>
              )
            })}
          </nav>
        </aside>

        <main className='min-w-0 space-y-10'>
          <section id='providers' className='scroll-mt-6 space-y-4'>
            <div>
              <h2 className='flex items-center gap-2 text-xl font-semibold'>
                <Info className='size-5' />
                {t('Providers')}
              </h2>
              <p className='text-muted-foreground mt-1 text-sm'>
                {t('Different companies host the same model.')}
              </p>
            </div>
            <ProviderRoutingControls
              mode={routingMode}
              onModeChange={setRoutingMode}
              allowFallbacks={allowFallbacks}
              onAllowFallbacksChange={setAllowFallbacks}
            />
            <ModelSquareProviderTable
              model={model}
              providers={routedProviders}
              primaryProviderSlug={primaryProvider?.slug}
              onProviderClick={setSelectedProvider}
              onMoveProvider={moveProvider}
            />
          </section>

          <section id='api' className='scroll-mt-6 space-y-4'>
            <h2 className='flex items-center gap-2 text-xl font-semibold'>
              <Code2 className='size-5' />
              {t('API')}
            </h2>
            <div className='bg-muted/20 rounded-lg border p-3'>
              <div className='mb-2 text-xs font-medium'>{t('Routing')}</div>
              <pre className='bg-background overflow-x-auto rounded-md border p-3 font-mono text-xs leading-relaxed'>
                {routingValue}
              </pre>
              <CopyButton
                value={routingValue}
                variant='outline'
                size='sm'
                className='mt-3'
              >
                {t('Copy')}
              </CopyButton>
            </div>
            <div className='bg-card rounded-xl border p-4 sm:p-6'>
              <ModelDetailsApi model={model} endpointMap={props.endpointMap} />
            </div>
          </section>
        </main>
      </div>

      <ModelProviderDetailsDrawer
        model={model}
        provider={selectedProvider}
        open={Boolean(selectedProvider)}
        onOpenChange={(open) => {
          if (!open) setSelectedProvider(null)
        }}
        groupRatio={props.groupRatio}
        usableGroup={props.usableGroup}
        autoGroups={props.autoGroups}
        priceRate={props.priceRate}
        usdExchangeRate={props.usdExchangeRate}
        tokenUnit={props.tokenUnit}
        showRechargePrice={props.showRechargePrice}
      />
    </div>
  )
}

// ----------------------------------------------------------------------------
// Model header (always visible above the detail sections)
// ----------------------------------------------------------------------------

function ModelHeader(props: { model: PricingModel }) {
  const { t } = useTranslation()
  const model = props.model
  const modelIconKey = model.icon || model.vendor_icon
  const modelIcon = modelIconKey ? getLobeIcon(modelIconKey, 20) : null
  const description = model.description || model.vendor_description || null

  return (
    <header className='pb-4'>
      <div className='flex items-center gap-2.5'>
        {modelIcon}
        <h1 className='font-mono text-xl font-bold tracking-tight sm:text-2xl'>
          {model.model_name}
        </h1>
        <CopyButton
          value={model.model_name || ''}
          className='size-6'
          iconClassName='size-3'
          tooltip={t('Copy model name')}
          successTooltip={t('Copied!')}
          aria-label={t('Copy model name')}
        />
      </div>
      <div className='mt-1 flex flex-wrap items-center gap-1.5 text-xs'>
        {model.vendor_name && (
          <span className='text-muted-foreground'>{model.vendor_name}</span>
        )}
        <span className='text-muted-foreground/30'>·</span>
        <ModelBillingModeBadge model={model} />
      </div>
      {description && (
        <p className='text-muted-foreground mt-2 text-sm leading-relaxed'>
          {description}
        </p>
      )}
    </header>
  )
}

// ----------------------------------------------------------------------------
// Base price card (used in the Overview tab)
// ----------------------------------------------------------------------------

function PriceSection(props: {
  model: PricingModel
  priceRate: number
  usdExchangeRate: number
  tokenUnit: TokenUnit
  showRechargePrice: boolean
}) {
  const { t } = useTranslation()
  const isTokenBased = isTokenBasedModel(props.model)
  const tokenUnitLabel = props.tokenUnit === 'K' ? '1K' : '1M'
  const baseGroupKey = '_base'
  const baseGroupRatioMap = { [baseGroupKey]: 1 }
  const dynamicSummary = getDynamicPricingSummary(props.model, {
    tokenUnit: props.tokenUnit,
    showRechargePrice: props.showRechargePrice,
    priceRate: props.priceRate,
    usdExchangeRate: props.usdExchangeRate,
    groupRatioMultiplier: 1,
  })

  const primaryPriceTypes: { label: string; type: PriceType }[] = [
    { label: t('Input'), type: 'input' },
    { label: t('Output'), type: 'output' },
  ]
  const secondaryPriceTypes: {
    label: string
    type: PriceType
    available: boolean
  }[] = [
    {
      label: t('Cached input'),
      type: 'cache',
      available: props.model.cache_ratio != null,
    },
    {
      label: t('Cache write'),
      type: 'create_cache',
      available: props.model.create_cache_ratio != null,
    },
    {
      label: t('Image input'),
      type: 'image',
      available: props.model.image_ratio != null,
    },
    {
      label: t('Audio input'),
      type: 'audio_input',
      available: props.model.audio_ratio != null,
    },
    {
      label: t('Audio output'),
      type: 'audio_output',
      available:
        props.model.audio_ratio != null &&
        props.model.audio_completion_ratio != null,
    },
  ]

  if (dynamicSummary) {
    if (dynamicSummary.isSpecialExpression) {
      return (
        <section>
          <SectionTitle>{t('Base Price')}</SectionTitle>
          <div className='rounded-lg border border-amber-200/70 bg-amber-50/70 p-3 dark:border-amber-500/20 dark:bg-amber-500/10'>
            <div className='text-sm font-medium text-amber-800 dark:text-amber-200'>
              {t('Special billing expression')}
            </div>
            <p className='text-muted-foreground mt-1 text-xs'>
              {t('Unable to parse structured pricing')}
            </p>
            <div className='mt-3'>
              <div className='text-muted-foreground mb-1 text-[10px] font-medium tracking-wider uppercase'>
                {t('Raw expression')}
              </div>
              <code className='text-muted-foreground bg-background/80 block max-h-28 overflow-auto rounded-md border px-2 py-1.5 font-mono text-xs break-all'>
                {dynamicSummary.rawExpression}
              </code>
            </div>
          </div>
        </section>
      )
    }

    return (
      <section>
        <SectionTitle>{t('Base Price')}</SectionTitle>
        {dynamicSummary.primaryEntries.length > 0 ? (
          <div className='grid grid-cols-2 gap-2'>
            {dynamicSummary.primaryEntries.map((entry) => (
              <div
                key={entry.key}
                className='bg-muted/20 rounded-lg border p-3'
              >
                <div className='text-muted-foreground text-xs'>
                  {t(entry.shortLabel)}
                </div>
                <div className='text-foreground mt-1 font-mono text-base font-semibold tabular-nums'>
                  {entry.formatted}
                  <span className='text-muted-foreground/40 ml-1 text-xs font-normal'>
                    / {tokenUnitLabel}
                  </span>
                </div>
              </div>
            ))}
          </div>
        ) : (
          <p className='text-muted-foreground text-sm'>
            {t('Dynamic Pricing')}
          </p>
        )}
        {dynamicSummary.secondaryEntries.length > 0 && (
          <div className='bg-muted/20 mt-3 rounded-lg border px-3 py-2.5'>
            <div className='space-y-1.5'>
              {dynamicSummary.secondaryEntries.map((entry) => (
                <div
                  key={entry.key}
                  className='flex items-baseline justify-between gap-4'
                >
                  <span className='text-muted-foreground/70 text-sm'>
                    {t(entry.shortLabel)}
                  </span>
                  <span className='text-muted-foreground font-mono text-sm tabular-nums'>
                    {entry.formatted}
                    <span className='text-muted-foreground/40 ml-1 text-xs font-normal'>
                      / {tokenUnitLabel}
                    </span>
                  </span>
                </div>
              ))}
            </div>
          </div>
        )}
      </section>
    )
  }

  if (!isTokenBased) {
    return (
      <section>
        <SectionTitle>{t('Base Price')}</SectionTitle>
        <div className='flex items-baseline justify-between'>
          <span className='text-muted-foreground text-sm'>
            {t('Per request')}
          </span>
          <span className='text-foreground font-mono text-sm font-semibold tabular-nums'>
            {formatFixedPrice(
              props.model,
              baseGroupKey,
              props.showRechargePrice,
              props.priceRate,
              props.usdExchangeRate,
              baseGroupRatioMap
            )}
          </span>
        </div>
      </section>
    )
  }

  const secondaryItems = secondaryPriceTypes.filter((p) => p.available)
  const renderPrice = (type: PriceType) => (
    <>
      {formatGroupPrice(
        props.model,
        baseGroupKey,
        type,
        props.tokenUnit,
        props.showRechargePrice,
        props.priceRate,
        props.usdExchangeRate,
        baseGroupRatioMap
      )}
      <span className='text-muted-foreground/40 ml-1 text-xs font-normal'>
        / {tokenUnitLabel}
      </span>
    </>
  )

  return (
    <section>
      <SectionTitle>{t('Base Price')}</SectionTitle>
      <div className='grid grid-cols-2 gap-2'>
        {primaryPriceTypes.map((item) => (
          <div key={item.type} className='bg-muted/20 rounded-lg border p-3'>
            <div className='text-muted-foreground text-xs'>{item.label}</div>
            <div className='text-foreground mt-1 font-mono text-base font-semibold tabular-nums'>
              {renderPrice(item.type)}
            </div>
          </div>
        ))}
      </div>
      {secondaryItems.length > 0 && (
        <div className='bg-muted/20 mt-3 rounded-lg border px-3 py-2.5'>
          <div className='space-y-1.5'>
            {secondaryItems.map((item) => (
              <div
                key={item.type}
                className='flex items-baseline justify-between gap-4'
              >
                <span className='text-muted-foreground/70 text-sm'>
                  {item.label}
                </span>
                <span className='text-muted-foreground font-mono text-sm tabular-nums'>
                  {renderPrice(item.type)}
                </span>
              </div>
            ))}
          </div>
        </div>
      )}
    </section>
  )
}

// ----------------------------------------------------------------------------
// Auto group chain (used inside group pricing section)
// ----------------------------------------------------------------------------

function AutoGroupChain(props: { model: PricingModel; autoGroups: string[] }) {
  const { t } = useTranslation()
  const modelEnableGroups = Array.isArray(props.model.enable_groups)
    ? props.model.enable_groups
    : []
  const autoChain = props.autoGroups.filter((g) =>
    modelEnableGroups.includes(g)
  )

  if (autoChain.length === 0) return null

  return (
    <div className='text-muted-foreground mb-3 flex flex-wrap items-center gap-1 text-xs'>
      <span className='font-medium'>{t('Auto Group Chain')}</span>
      <span className='text-muted-foreground/40'>→</span>
      {autoChain.map((g, idx) => (
        <span key={g} className='flex items-center gap-1'>
          <GroupBadge group={g} size='sm' />
          {idx < autoChain.length - 1 && (
            <span className='text-muted-foreground/40'>→</span>
          )}
        </span>
      ))}
    </div>
  )
}

type DynamicPriceOptions = Parameters<typeof getDynamicPriceEntries>[1]
type DynamicPricingTier = ReturnType<typeof getDynamicPricingTiers>[number]
type DynamicFormattedPricesByTier = Map<DynamicPricingTier, Map<string, string>>

function getDynamicPriceFields(
  tiers: DynamicPricingTier[],
  options: DynamicPriceOptions
) {
  return [
    ...new Map(
      tiers
        .flatMap((tier) => getDynamicPriceEntries(tier, options))
        .map((entry) => [entry.field, entry])
    ).values(),
  ]
}

function getDynamicFormattedPricesByTier(
  tiers: DynamicPricingTier[],
  options: DynamicPriceOptions
): DynamicFormattedPricesByTier {
  return new Map(
    tiers.map((tier) => [
      tier,
      new Map(
        getDynamicPriceEntries(tier, options).map((entry) => [
          entry.field,
          entry.formatted,
        ])
      ),
    ])
  )
}

// ----------------------------------------------------------------------------
// Group pricing table
// ----------------------------------------------------------------------------

function GroupPricingSection(props: {
  model: PricingModel
  groupRatio: Record<string, number>
  usableGroup: Record<string, { desc: string; ratio: number }>
  autoGroups: string[]
  priceRate: number
  usdExchangeRate: number
  tokenUnit: TokenUnit
  showRechargePrice?: boolean
}) {
  const { t } = useTranslation()
  const showRechargePrice = props.showRechargePrice ?? false

  const availableGroups = useMemo(
    () => getAvailableGroups(props.model, props.usableGroup || {}),
    [props.model, props.usableGroup]
  )

  const isTokenBased = isTokenBasedModel(props.model)
  const tokenUnitLabel = props.tokenUnit === 'K' ? '1K' : '1M'

  const extraPriceTypes = useMemo(() => {
    const types: { label: string; type: PriceType }[] = []
    if (props.model.cache_ratio != null) {
      types.push({ label: t('Cache'), type: 'cache' })
    }
    if (props.model.create_cache_ratio != null) {
      types.push({ label: t('Cache Write'), type: 'create_cache' })
    }
    if (props.model.image_ratio != null) {
      types.push({ label: t('Image'), type: 'image' })
    }
    if (props.model.audio_ratio != null) {
      types.push({ label: t('Audio In'), type: 'audio_input' })
    }
    if (
      props.model.audio_ratio != null &&
      props.model.audio_completion_ratio != null
    ) {
      types.push({ label: t('Audio Out'), type: 'audio_output' })
    }
    return types
  }, [props.model, t])

  if (availableGroups.length === 0) {
    return (
      <section>
        <SectionTitle>{t('Pricing by Group')}</SectionTitle>
        <AutoGroupChain model={props.model} autoGroups={props.autoGroups} />
        <p className='text-muted-foreground text-sm'>
          {t(
            'This model is not available in any group, or no group pricing information is configured.'
          )}
        </p>
      </section>
    )
  }

  const thClass =
    'text-muted-foreground py-2 text-[10px] font-medium tracking-wider uppercase'

  if (isDynamicPricingModel(props.model)) {
    const dynamicTiers = getDynamicPricingTiers(props.model)

    if (dynamicTiers.length === 0) {
      return (
        <section>
          <SectionTitle>{t('Pricing by Group')}</SectionTitle>
          <AutoGroupChain model={props.model} autoGroups={props.autoGroups} />
          <div className='rounded-lg border border-amber-200/70 bg-amber-50/70 p-3 dark:border-amber-500/20 dark:bg-amber-500/10'>
            <div className='text-sm font-medium text-amber-800 dark:text-amber-200'>
              {t('Special billing expression')}
            </div>
            <p className='text-muted-foreground mt-1 text-xs'>
              {t(
                'Group prices cannot be expanded because this expression is not a standard tiered pricing expression.'
              )}
            </p>
            <div className='mt-3'>
              <div className='text-muted-foreground mb-1 text-[10px] font-medium tracking-wider uppercase'>
                {t('Raw expression')}
              </div>
              <code className='text-muted-foreground bg-background/80 block max-h-28 overflow-auto rounded-md border px-2 py-1.5 font-mono text-xs break-all'>
                {props.model.billing_expr}
              </code>
            </div>
          </div>
        </section>
      )
    }

    const priceFields = getDynamicPriceFields(dynamicTiers, {
      tokenUnit: props.tokenUnit,
      showRechargePrice,
      priceRate: props.priceRate,
      usdExchangeRate: props.usdExchangeRate,
      groupRatioMultiplier: 1,
    })
    const formattedPricesByGroup = new Map(
      availableGroups.map((group) => {
        const ratio = props.groupRatio[group] || 1
        return [
          group,
          getDynamicFormattedPricesByTier(dynamicTiers, {
            tokenUnit: props.tokenUnit,
            showRechargePrice,
            priceRate: props.priceRate,
            usdExchangeRate: props.usdExchangeRate,
            groupRatioMultiplier: ratio,
          }),
        ] as const
      })
    )

    return (
      <section>
        <SectionTitle>{t('Pricing by Group')}</SectionTitle>
        <AutoGroupChain model={props.model} autoGroups={props.autoGroups} />
        <div className='space-y-3'>
          {availableGroups.map((group) => {
            const ratio = props.groupRatio[group] || 1
            const formattedPricesByTier =
              formattedPricesByGroup.get(group) ??
              new Map<DynamicPricingTier, Map<string, string>>()

            return (
              <div key={group} className='overflow-hidden rounded-lg border'>
                <div className='bg-muted/20 flex items-center justify-between gap-3 border-b px-3 py-2'>
                  <GroupBadge group={group} size='sm' />
                  <span className='text-muted-foreground font-mono text-xs'>
                    {ratio}x
                  </span>
                </div>
                <StaticDataTable
                  className='rounded-none border-0'
                  tableClassName='text-sm'
                  headerRowClassName='hover:bg-transparent'
                  data={dynamicTiers}
                  getRowKey={(tier, tierIndex) =>
                    `${group}-${tier.label || tierIndex}`
                  }
                  columns={[
                    {
                      id: 'tier',
                      header: t('Tier'),
                      className: thClass,
                      cellClassName: 'text-muted-foreground py-2.5',
                      cell: (tier) => tier.label || t('Default'),
                    },
                    ...priceFields.map((fieldEntry) => ({
                      id: fieldEntry.field,
                      header: t(fieldEntry.shortLabel),
                      className: `${thClass} text-right`,
                      cellClassName: 'py-2.5 text-right font-mono',
                      cell: (tier: (typeof dynamicTiers)[number]) =>
                        formattedPricesByTier
                          .get(tier)
                          ?.get(fieldEntry.field) ?? '-',
                    })),
                  ]}
                />
              </div>
            )
          })}
          <p className='text-muted-foreground/40 mt-1.5 text-[10px]'>
            {t('Prices shown per')} {tokenUnitLabel} tokens
          </p>
        </div>
      </section>
    )
  }

  const renderGroupPrice = (group: string, type: PriceType) =>
    formatGroupPrice(
      props.model,
      group,
      type,
      props.tokenUnit,
      showRechargePrice,
      props.priceRate,
      props.usdExchangeRate,
      props.groupRatio
    )
  const renderFixedGroupPrice = (group: string) =>
    formatFixedPrice(
      props.model,
      group,
      showRechargePrice,
      props.priceRate,
      props.usdExchangeRate,
      props.groupRatio
    )

  return (
    <section>
      <SectionTitle>{t('Pricing by Group')}</SectionTitle>
      <AutoGroupChain model={props.model} autoGroups={props.autoGroups} />
      <StaticDataTable
        className='-mx-4 rounded-none border-0 sm:mx-0'
        tableClassName='text-sm'
        headerRowClassName='hover:bg-transparent'
        data={availableGroups}
        getRowKey={(group) => group}
        columns={[
          {
            id: 'group',
            header: t('Group'),
            className: thClass,
            cellClassName: 'py-2.5',
            cell: (group) => <GroupBadge group={group} size='sm' />,
          },
          {
            id: 'ratio',
            header: t('Ratio'),
            className: thClass,
            cellClassName: 'text-muted-foreground py-2.5 font-mono',
            cell: (group) => `${props.groupRatio[group] || 1}x`,
          },
          ...(isTokenBased
            ? [
                {
                  id: 'input',
                  header: t('Input'),
                  className: `${thClass} text-right`,
                  cellClassName: 'py-2.5 text-right font-mono',
                  cell: (group: string) => renderGroupPrice(group, 'input'),
                },
                {
                  id: 'output',
                  header: t('Output'),
                  className: `${thClass} text-right`,
                  cellClassName: 'py-2.5 text-right font-mono',
                  cell: (group: string) => renderGroupPrice(group, 'output'),
                },
                ...extraPriceTypes.map((ep) => ({
                  id: ep.type,
                  header: ep.label,
                  className: `${thClass} text-right`,
                  cellClassName: 'py-2.5 text-right font-mono',
                  cell: (group: string) => renderGroupPrice(group, ep.type),
                })),
              ]
            : [
                {
                  id: 'price',
                  header: t('Price'),
                  className: `${thClass} text-right`,
                  cellClassName: 'py-2.5 text-right font-mono',
                  cell: renderFixedGroupPrice,
                },
              ]),
        ]}
      />
      <div className='-mx-4 sm:mx-0'>
        {isTokenBased && (
          <p className='text-muted-foreground/40 mt-1.5 px-4 text-[10px] sm:px-0'>
            {t('Prices shown per')} {tokenUnitLabel} tokens
          </p>
        )}
      </div>
    </section>
  )
}

const TAB_VALUES = ['overview', 'performance', 'api'] as const
type TabValue = (typeof TAB_VALUES)[number]

const TAB_META: Record<
  TabValue,
  { icon: React.ComponentType<{ className?: string }>; labelKey: string }
> = {
  overview: { icon: Info, labelKey: 'Overview' },
  performance: { icon: HeartPulse, labelKey: 'Performance' },
  api: { icon: Code2, labelKey: 'API' },
}

export interface ModelDetailsContentProps {
  model: PricingModel
  groupRatio: Record<string, number>
  usableGroup: Record<string, { desc: string; ratio: number }>
  endpointMap: Record<string, { path?: string; method?: string }>
  autoGroups: string[]
  priceRate: number
  usdExchangeRate: number
  tokenUnit: TokenUnit
  showRechargePrice?: boolean
}

export function ModelDetailsContent(props: ModelDetailsContentProps) {
  const { t } = useTranslation()
  const [selectedProvider, setSelectedProvider] =
    useState<PricingProvider | null>(null)
  const showRechargePrice = props.showRechargePrice ?? false

  const isDynamic =
    props.model.billing_mode === 'tiered_expr' &&
    Boolean(props.model.billing_expr)

  return (
    <div className='@container/details space-y-4'>
      <ModelHeader model={props.model} />

      <Tabs defaultValue='overview' className='gap-4'>
        <TabsList className='bg-muted/60 grid w-full grid-cols-3 gap-1 rounded-lg p-1 group-data-horizontal/tabs:h-auto'>
          {TAB_VALUES.map((value) => {
            const Icon = TAB_META[value].icon
            return (
              <TabsTrigger
                key={value}
                value={value}
                className='h-8 min-w-0 gap-1.5 rounded-md px-3 text-xs sm:text-sm'
              >
                <Icon className='size-3.5' />
                <span className='truncate'>{t(TAB_META[value].labelKey)}</span>
              </TabsTrigger>
            )
          })}
        </TabsList>

        <TabsContent value='overview' className='space-y-6 outline-none'>
          <OverviewSummaryGrid model={props.model} />

          <section className='bg-card/60 space-y-5 rounded-xl border p-4 shadow-sm'>
            <SectionTitle>{t('Pricing')}</SectionTitle>
            <PriceSection
              model={props.model}
              priceRate={props.priceRate}
              usdExchangeRate={props.usdExchangeRate}
              tokenUnit={props.tokenUnit}
              showRechargePrice={showRechargePrice}
            />
            {isDynamic && (
              <DynamicPricingBreakdown billingExpr={props.model.billing_expr} />
            )}
            <GroupPricingSection
              model={props.model}
              groupRatio={props.groupRatio}
              usableGroup={props.usableGroup}
              autoGroups={props.autoGroups}
              priceRate={props.priceRate}
              usdExchangeRate={props.usdExchangeRate}
              tokenUnit={props.tokenUnit}
              showRechargePrice={showRechargePrice}
            />
          </section>

          <ModelBackendDetailsSection model={props.model} />
          <ModelProvidersSection
            model={props.model}
            onProviderClick={setSelectedProvider}
          />
        </TabsContent>

        <TabsContent value='performance' className='outline-none'>
          <ModelDetailsPerformance model={props.model} />
        </TabsContent>

        <TabsContent value='api' className='outline-none'>
          <ModelDetailsApi
            model={props.model}
            endpointMap={props.endpointMap}
          />
        </TabsContent>
      </Tabs>
      <ModelProviderDetailsDrawer
        model={props.model}
        provider={selectedProvider}
        open={Boolean(selectedProvider)}
        onOpenChange={(open) => {
          if (!open) setSelectedProvider(null)
        }}
        groupRatio={props.groupRatio}
        usableGroup={props.usableGroup}
        autoGroups={props.autoGroups}
        priceRate={props.priceRate}
        usdExchangeRate={props.usdExchangeRate}
        tokenUnit={props.tokenUnit}
        showRechargePrice={props.showRechargePrice}
      />
    </div>
  )
}

// ----------------------------------------------------------------------------
// Drawer & page wrappers
// ----------------------------------------------------------------------------

export interface ModelDetailsDrawerProps extends ModelDetailsContentProps {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function ModelDetailsDrawer(props: ModelDetailsDrawerProps) {
  const { t } = useTranslation()
  const { open, onOpenChange, ...contentProps } = props

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent
        side='right'
        className={sideDrawerContentClassName(
          'sm:max-w-2xl lg:max-w-3xl xl:max-w-4xl 2xl:max-w-5xl'
        )}
      >
        <SheetHeader className='sr-only'>
          <SheetTitle>{props.model.model_name}</SheetTitle>
          <SheetDescription>{t('Model details')}</SheetDescription>
        </SheetHeader>
        <div className='flex-1 overflow-y-auto px-4 pt-11 pb-5 sm:px-6 sm:pt-12 sm:pb-6'>
          <ModelDetailsContent {...contentProps} />
        </div>
      </SheetContent>
    </Sheet>
  )
}

export function ModelDetails() {
  const { t } = useTranslation()
  const { modelId } = useParams({ from: '/pricing/$modelId/' })
  const search = useSearch({ from: '/pricing/$modelId/' })
  const navigate = useNavigate()

  const {
    models,
    groupRatio,
    usableGroup,
    endpointMap,
    autoGroups,
    isLoading,
    priceRate,
    usdExchangeRate,
  } = usePricingData()

  const tokenUnit: TokenUnit =
    search.tokenUnit === 'K' ? 'K' : DEFAULT_TOKEN_UNIT

  const model = useMemo(() => {
    if (!models || !modelId) return null
    return models.find((m) => m.model_name === modelId) || null
  }, [models, modelId])

  const handleBack = () => {
    navigate({ to: '/pricing', search })
  }

  if (isLoading) {
    return (
      <PublicLayout>
        <div className='mx-auto max-w-5xl px-4 sm:px-6'>
          <Skeleton className='mb-4 h-5 w-16' />
          <div className='space-y-2'>
            <Skeleton className='h-7 w-64' />
            <Skeleton className='h-4 w-40' />
            <Skeleton className='h-4 w-full max-w-md' />
          </div>
          <div className='mt-6 grid grid-cols-2 gap-2 sm:grid-cols-4'>
            {MODEL_DETAILS_SKELETON_KEYS.map((key) => (
              <Skeleton key={`metric-${key}`} className='h-16 w-full' />
            ))}
          </div>
          <div className='mt-6 space-y-3'>
            {MODEL_DETAILS_SKELETON_KEYS.map((key) => (
              <Skeleton key={`section-${key}`} className='h-24 w-full' />
            ))}
          </div>
        </div>
      </PublicLayout>
    )
  }

  if (!model) {
    return (
      <PublicLayout>
        <div className='mx-auto max-w-2xl px-4 text-center sm:px-6'>
          <h2 className='mb-1 text-base font-semibold'>
            {t('Model not found')}
          </h2>
          <p className='text-muted-foreground mb-4 text-sm'>
            {t("The model you're looking for doesn't exist.")}
          </p>
          <Button onClick={handleBack} variant='outline' size='sm'>
            {t('Back to Models')}
          </Button>
        </div>
      </PublicLayout>
    )
  }

  return (
    <PublicLayout>
      <div className='mx-auto max-w-5xl px-4 sm:px-6'>
        <Button
          variant='ghost'
          size='sm'
          onClick={handleBack}
          className='text-muted-foreground hover:text-foreground mb-4 h-auto gap-1 px-0 py-1 text-xs'
        >
          <ArrowLeft className='size-3.5' />
          {t('Back')}
        </Button>

        <ModelDetailsContent
          model={model}
          groupRatio={groupRatio || {}}
          usableGroup={usableGroup || {}}
          autoGroups={autoGroups || []}
          priceRate={priceRate ?? 1}
          usdExchangeRate={usdExchangeRate ?? 1}
          tokenUnit={tokenUnit}
          showRechargePrice={search.rechargePrice ?? false}
          endpointMap={
            (endpointMap as Record<
              string,
              { path?: string; method?: string }
            >) || {}
          }
        />
      </div>
    </PublicLayout>
  )
}

export function ModelSquareDetails() {
  const { t } = useTranslation()
  const { modelId } = useParams({ strict: false })
  const navigate = useNavigate()
  const { status, loading: statusLoading } = useStatus()
  const priceRate = Math.max((status?.price as number) ?? 1, 0.001)
  const usdExchangeRate = Math.max(
    (status?.usd_exchange_rate as number) ?? priceRate,
    0.001
  )
  const detailQuery = useQuery({
    queryKey: ['model-square-detail', modelId],
    queryFn: () => getModelSquareDetail(modelId || ''),
    enabled: Boolean(modelId),
    staleTime: 5 * 60 * 1000,
  })

  const model = useMemo(
    () => detailQuery.data?.data ?? null,
    [detailQuery.data]
  )

  if (statusLoading || detailQuery.isLoading) {
    return (
      <div className='h-full overflow-y-auto'>
        <div className='mx-auto w-[96%] max-w-[1600px] space-y-3 px-4 py-6 sm:px-6 sm:py-8'>
          <Skeleton className='h-7 w-64' />
          <Skeleton className='h-4 w-full max-w-2xl' />
          <div className='grid grid-cols-2 gap-2 sm:grid-cols-4'>
            {MODEL_DETAILS_SKELETON_KEYS.map((key) => (
              <Skeleton key={key} className='h-16 w-full' />
            ))}
          </div>
        </div>
      </div>
    )
  }

  if (!model) {
    return (
      <div className='h-full overflow-y-auto'>
        <div className='mx-auto w-[96%] max-w-[1600px] px-4 py-16 text-center sm:px-6'>
          <h2 className='mb-1 text-base font-semibold'>
            {t('Model not found')}
          </h2>
          <p className='text-muted-foreground mb-4 text-sm'>
            {t("The model you're looking for doesn't exist.")}
          </p>
          <Button
            onClick={() => navigate({ to: '/model-square' })}
            variant='outline'
            size='sm'
          >
            {t('Back to Models')}
          </Button>
        </div>
      </div>
    )
  }

  return (
    <div className='h-full overflow-y-auto'>
      <div className='mx-auto w-[96%] max-w-[1600px] px-4 py-6 sm:px-6 sm:py-8'>
        <Button
          variant='ghost'
          size='sm'
          onClick={() => navigate({ to: '/model-square' })}
          className='text-muted-foreground hover:text-foreground mb-4 h-auto gap-1 px-0 py-1 text-xs'
        >
          <ArrowLeft className='size-3.5' />
          {t('Back')}
        </Button>
        <ModelSquareDetailPage
          model={model}
          groupRatio={{}}
          usableGroup={{}}
          autoGroups={[]}
          priceRate={priceRate}
          usdExchangeRate={usdExchangeRate}
          tokenUnit={DEFAULT_TOKEN_UNIT}
          endpointMap={model.endpoint_map ?? {}}
        />
      </div>
    </div>
  )
}
