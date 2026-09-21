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
import { zodResolver } from '@hookform/resolvers/zod'
import { useMemo, useRef, useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import * as z from 'zod'

import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { PasswordInput } from '@/components/password-input'
import { Separator } from '@/components/ui/separator'
import { Switch } from '@/components/ui/switch'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { parseHttpStatusCodeRules } from '@/lib/http-status-code-rules'

import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { useResetForm } from '../hooks/use-reset-form'
import { useUpdateOption } from '../hooks/use-update-option'
import { safeNumberFieldProps } from '../utils/numeric-field'
import { ChannelModelEventLogs } from './channel-model-event-logs'

const MAX_SUCCESS_THRESHOLD = 100
const MAX_DELAY_MINUTES = 43200
const MAX_INTERVAL_MINUTES = 10080
const MAX_CONCURRENCY = 32
const MAX_RETENTION_DAYS = 3650
const MAX_EXCLUDED_CHANNEL_IDS = 1000

function parseExcludedChannelIds(value: string) {
  const tokens = value.replaceAll(/[，；;]/g, ',').split(',')
  const ids = new Set<number>()
  const invalid: string[] = []
  for (const rawToken of tokens) {
    const token = rawToken.trim()
    if (!token) continue
    if (!/^\d+$/.test(token)) {
      invalid.push(token)
      continue
    }
    const channelId = Number(token)
    if (!Number.isSafeInteger(channelId) || channelId <= 0) {
      invalid.push(token)
      continue
    }
    ids.add(channelId)
  }
  const sortedIds = [...ids].sort((left, right) => left - right)
  return {
    ids: sortedIds,
    invalid,
    normalized: sortedIds.join(','),
  }
}

const createSchema = (
  t: (key: string, options?: Record<string, unknown>) => string
) =>
  z
    .object({
      monitor_setting: z.object({
        channel_model_circuit_breaker_enabled: z.boolean(),
        channel_model_excluded_channel_ids: z.string(),
        channel_error_window_minutes: z.coerce.number().int().min(1),
        channel_error_threshold: z.coerce.number().int().min(0),
        channel_error_consecutive_threshold: z.coerce.number().int().min(0),
        channel_error_status_codes: z.string(),
        channel_model_recovery_enabled: z.boolean(),
        channel_model_recovery_success_threshold: z.coerce
          .number()
          .int()
          .min(1)
          .max(MAX_SUCCESS_THRESHOLD),
        channel_model_recovery_delay_minutes: z.coerce
          .number()
          .int()
          .min(0)
          .max(MAX_DELAY_MINUTES),
        channel_model_recovery_interval_minutes: z.coerce
          .number()
          .int()
          .min(1)
          .max(MAX_INTERVAL_MINUTES),
        channel_model_recovery_concurrency: z.coerce
          .number()
          .int()
          .min(1)
          .max(MAX_CONCURRENCY),
        channel_model_event_retention_days: z.coerce
          .number()
          .int()
          .min(1)
          .max(MAX_RETENTION_DAYS),
        channel_model_wecom_bot_enabled: z.boolean(),
        channel_model_wecom_bot_url: z.url(),
        channel_model_wecom_bot_webhook_key: z.string().max(1024),
        channel_model_wecom_bot_webhook_key_configured: z.boolean(),
        channel_model_wecom_bot_contact: z.string().max(2048),
      }),
    })
    .superRefine((values, ctx) => {
      const excludedChannels = parseExcludedChannelIds(
        values.monitor_setting.channel_model_excluded_channel_ids
      )
      if (excludedChannels.invalid.length > 0) {
        ctx.addIssue({
          code: 'custom',
          path: ['monitor_setting', 'channel_model_excluded_channel_ids'],
          message: t('Invalid channel IDs: {{ids}}', {
            ids: excludedChannels.invalid.join(', '),
          }),
        })
      } else if (excludedChannels.ids.length > MAX_EXCLUDED_CHANNEL_IDS) {
        ctx.addIssue({
          code: 'custom',
          path: ['monitor_setting', 'channel_model_excluded_channel_ids'],
          message: t('At most {{count}} channel IDs are allowed', {
            count: MAX_EXCLUDED_CHANNEL_IDS,
          }),
        })
      }
      if (!values.monitor_setting.channel_model_circuit_breaker_enabled) return
      const parsed = parseHttpStatusCodeRules(
        values.monitor_setting.channel_error_status_codes
      )
      if (!parsed.ok) {
        ctx.addIssue({
          code: 'custom',
          path: ['monitor_setting', 'channel_error_status_codes'],
          message: t('Invalid status code rules: {{tokens}}', {
            tokens: parsed.invalidTokens.join(', '),
          }),
        })
      }
      if (
        values.monitor_setting.channel_model_wecom_bot_enabled &&
        !values.monitor_setting.channel_model_wecom_bot_webhook_key.trim() &&
        !values.monitor_setting.channel_model_wecom_bot_webhook_key_configured
      ) {
        ctx.addIssue({
          code: 'custom',
          path: [
            'monitor_setting',
            'channel_model_wecom_bot_webhook_key',
          ],
          message: t('Webhook Key is required before enabling notifications'),
        })
      }
    })

type CircuitBreakerSchema = ReturnType<typeof createSchema>
type CircuitBreakerFormInput = z.input<CircuitBreakerSchema>
type CircuitBreakerFormValues = z.output<CircuitBreakerSchema>

export type ChannelModelCircuitBreakerDefaults = {
  'monitor_setting.channel_model_circuit_breaker_enabled': boolean
  'monitor_setting.channel_model_excluded_channel_ids': string
  'monitor_setting.channel_error_window_minutes': number
  'monitor_setting.channel_error_threshold': number
  'monitor_setting.channel_error_consecutive_threshold': number
  'monitor_setting.channel_error_status_codes': string
  'monitor_setting.channel_model_recovery_enabled': boolean
  'monitor_setting.channel_model_recovery_success_threshold': number
  'monitor_setting.channel_model_recovery_delay_minutes': number
  'monitor_setting.channel_model_recovery_interval_minutes': number
  'monitor_setting.channel_model_recovery_concurrency': number
  'monitor_setting.channel_model_event_retention_days': number
  'monitor_setting.channel_model_wecom_bot_enabled': boolean
  'monitor_setting.channel_model_wecom_bot_url': string
  'monitor_setting.channel_model_wecom_bot_webhook_key_configured': boolean
  'monitor_setting.channel_model_wecom_bot_contact': string
}

const buildDefaults = (
  defaults: ChannelModelCircuitBreakerDefaults
): CircuitBreakerFormInput => ({
  monitor_setting: {
    channel_model_circuit_breaker_enabled:
      defaults['monitor_setting.channel_model_circuit_breaker_enabled'],
    channel_model_excluded_channel_ids:
      defaults['monitor_setting.channel_model_excluded_channel_ids'],
    channel_error_window_minutes:
      defaults['monitor_setting.channel_error_window_minutes'],
    channel_error_threshold:
      defaults['monitor_setting.channel_error_threshold'],
    channel_error_consecutive_threshold:
      defaults['monitor_setting.channel_error_consecutive_threshold'],
    channel_error_status_codes:
      defaults['monitor_setting.channel_error_status_codes'],
    channel_model_recovery_enabled:
      defaults['monitor_setting.channel_model_recovery_enabled'],
    channel_model_recovery_success_threshold:
      defaults['monitor_setting.channel_model_recovery_success_threshold'],
    channel_model_recovery_delay_minutes:
      defaults['monitor_setting.channel_model_recovery_delay_minutes'],
    channel_model_recovery_interval_minutes:
      defaults['monitor_setting.channel_model_recovery_interval_minutes'],
    channel_model_recovery_concurrency:
      defaults['monitor_setting.channel_model_recovery_concurrency'],
    channel_model_event_retention_days:
      defaults['monitor_setting.channel_model_event_retention_days'],
    channel_model_wecom_bot_enabled:
      defaults['monitor_setting.channel_model_wecom_bot_enabled'],
    channel_model_wecom_bot_url:
      defaults['monitor_setting.channel_model_wecom_bot_url'],
    channel_model_wecom_bot_webhook_key: '',
    channel_model_wecom_bot_webhook_key_configured:
      defaults[
        'monitor_setting.channel_model_wecom_bot_webhook_key_configured'
      ],
    channel_model_wecom_bot_contact:
      defaults['monitor_setting.channel_model_wecom_bot_contact'],
  },
})

const normalize = (values: CircuitBreakerFormValues) => ({
  'monitor_setting.channel_model_circuit_breaker_enabled':
    values.monitor_setting.channel_model_circuit_breaker_enabled,
  'monitor_setting.channel_model_excluded_channel_ids':
    parseExcludedChannelIds(
      values.monitor_setting.channel_model_excluded_channel_ids
    ).normalized,
  'monitor_setting.channel_error_window_minutes':
    values.monitor_setting.channel_error_window_minutes,
  'monitor_setting.channel_error_threshold':
    values.monitor_setting.channel_error_threshold,
  'monitor_setting.channel_error_consecutive_threshold':
    values.monitor_setting.channel_error_consecutive_threshold,
  'monitor_setting.channel_error_status_codes': parseHttpStatusCodeRules(
    values.monitor_setting.channel_error_status_codes
  ).normalized,
  'monitor_setting.channel_model_recovery_enabled':
    values.monitor_setting.channel_model_recovery_enabled,
  'monitor_setting.channel_model_recovery_success_threshold':
    values.monitor_setting.channel_model_recovery_success_threshold,
  'monitor_setting.channel_model_recovery_delay_minutes':
    values.monitor_setting.channel_model_recovery_delay_minutes,
  'monitor_setting.channel_model_recovery_interval_minutes':
    values.monitor_setting.channel_model_recovery_interval_minutes,
  'monitor_setting.channel_model_recovery_concurrency':
    values.monitor_setting.channel_model_recovery_concurrency,
  'monitor_setting.channel_model_event_retention_days':
    values.monitor_setting.channel_model_event_retention_days,
  'monitor_setting.channel_model_wecom_bot_url':
    values.monitor_setting.channel_model_wecom_bot_url.trim(),
  'monitor_setting.channel_model_wecom_bot_contact':
    values.monitor_setting.channel_model_wecom_bot_contact.trim(),
  'monitor_setting.channel_model_wecom_bot_enabled':
    values.monitor_setting.channel_model_wecom_bot_enabled,
})

type NormalizedValues = ReturnType<typeof normalize>

type ChannelModelCircuitBreakerSectionProps = {
  defaultValues: ChannelModelCircuitBreakerDefaults
}

export function ChannelModelCircuitBreakerSection(
  props: ChannelModelCircuitBreakerSectionProps
) {
  const { t } = useTranslation()
  const [activeTab, setActiveTab] = useState('configuration')
  const updateOption = useUpdateOption()
  const schema = createSchema(t)
  const formDefaults = useMemo(
    () => buildDefaults(props.defaultValues),
    [props.defaultValues]
  )
  const baselineRef = useRef<NormalizedValues>(
    normalize(formDefaults as CircuitBreakerFormValues)
  )
  const form = useForm<
    CircuitBreakerFormInput,
    unknown,
    CircuitBreakerFormValues
  >({
    resolver: zodResolver(schema),
    defaultValues: formDefaults,
  })
  useResetForm(form, formDefaults)

  const circuitBreakerEnabled = form.watch(
    'monitor_setting.channel_model_circuit_breaker_enabled'
  )
  const recoveryEnabled = form.watch(
    'monitor_setting.channel_model_recovery_enabled'
  )
  const recoveryFieldsEnabled = circuitBreakerEnabled && recoveryEnabled
  const weComBotEnabled = form.watch(
    'monitor_setting.channel_model_wecom_bot_enabled'
  )
  const weComBotWebhookKeyConfigured = form.watch(
    'monitor_setting.channel_model_wecom_bot_webhook_key_configured'
  )
  const statusCodes = form.watch(
    'monitor_setting.channel_error_status_codes'
  )
  const parsedStatusCodes = useMemo(
    () => parseHttpStatusCodeRules(statusCodes),
    [statusCodes]
  )

  const onSubmit = async (values: CircuitBreakerFormValues) => {
    const normalized = normalize(values)
    const changedKeys: Array<keyof NormalizedValues | 'monitor_setting.channel_model_wecom_bot_webhook_key'> = (
      Object.keys(normalized) as Array<keyof NormalizedValues>
    ).filter((key) => normalized[key] !== baselineRef.current[key])
    const enableKey = 'monitor_setting.channel_model_wecom_bot_enabled'
    const enableKeyIndex = changedKeys.indexOf(enableKey)
    if (enableKeyIndex >= 0) {
      changedKeys.splice(enableKeyIndex, 1)
    }
    const webhookKey =
      values.monitor_setting.channel_model_wecom_bot_webhook_key.trim()
    if (webhookKey) {
      changedKeys.push('monitor_setting.channel_model_wecom_bot_webhook_key')
    }
    if (enableKeyIndex >= 0) {
      changedKeys.push(enableKey)
    }
    if (changedKeys.length === 0) {
      toast.info(t('No changes to save'))
      return
    }
    for (const key of changedKeys) {
      await updateOption.mutateAsync({
        key,
        value:
          key === 'monitor_setting.channel_model_wecom_bot_webhook_key'
            ? webhookKey
            : normalized[key],
      })
    }
    baselineRef.current = normalized
    if (webhookKey) {
      form.setValue(
        'monitor_setting.channel_model_wecom_bot_webhook_key',
        ''
      )
      form.setValue(
        'monitor_setting.channel_model_wecom_bot_webhook_key_configured',
        true
      )
    }
  }

  return (
    <Tabs value={activeTab} onValueChange={setActiveTab}>
      <TabsList className='grid w-full grid-cols-2'>
        <TabsTrigger value='configuration'>
          {t('Parameter Configuration')}
        </TabsTrigger>
        <TabsTrigger value='logs'>{t('Log Query')}</TabsTrigger>
      </TabsList>

      <TabsContent value='configuration' className='pt-4'>
        <Form {...form}>
          <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
            <SettingsPageFormActions
              onSave={form.handleSubmit(onSubmit)}
              isSaving={updateOption.isPending}
            />

          <div className='space-y-4'>
            <div>
              <h3 className='text-sm font-semibold'>{t('Circuit breaker')}</h3>
              <p className='text-muted-foreground mt-1 text-xs'>
                {t(
                  'Failures are isolated by channel and requested model without changing permanent model mappings.'
                )}
              </p>
            </div>
            <div className='grid gap-6 lg:grid-cols-2'>
              <FormField
                control={form.control}
                name='monitor_setting.channel_model_circuit_breaker_enabled'
                render={({ field }) => (
                  <SettingsSwitchItem className='lg:col-span-2'>
                    <SettingsSwitchContent>
                      <FormLabel>
                        {t('Enable channel-model circuit breaker')}
                      </FormLabel>
                      <FormDescription>
                        {t(
                          'Turning this off restores all disabled channel-model routes and clears their failure counters.'
                        )}
                      </FormDescription>
                    </SettingsSwitchContent>
                    <FormControl>
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                      />
                    </FormControl>
                  </SettingsSwitchItem>
                )}
              />
              <FormField
                control={form.control}
                name='monitor_setting.channel_model_excluded_channel_ids'
                render={({ field }) => (
                  <FormItem className='lg:col-span-2'>
                    <FormLabel>{t('Excluded channel IDs')}</FormLabel>
                    <FormControl>
                      <Input
                        placeholder={t('e.g. 1,2,10')}
                        value={field.value}
                        onChange={(event) => field.onChange(event.target.value)}
                        disabled={!circuitBreakerEnabled}
                      />
                    </FormControl>
                    <FormDescription>
                      {t(
                        'Models on these channels do not participate in channel-model circuit breaking. Existing circuit breakers on newly excluded channels are restored when saved.'
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='monitor_setting.channel_error_window_minutes'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Failure window (minutes)')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={1}
                        {...safeNumberFieldProps(field)}
                        disabled={!circuitBreakerEnabled}
                      />
                    </FormControl>
                    <FormDescription>{t('Default: {{value}}', { value: 5 })}</FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='monitor_setting.channel_error_threshold'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Window failure threshold')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={0}
                        {...safeNumberFieldProps(field)}
                        disabled={!circuitBreakerEnabled}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Default: {{value}}; 0 disables this trigger.', {
                        value: 20,
                      })}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='monitor_setting.channel_error_consecutive_threshold'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Consecutive failure threshold')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={0}
                        {...safeNumberFieldProps(field)}
                        disabled={!circuitBreakerEnabled}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Default: {{value}}; 0 disables this trigger.', {
                        value: 10,
                      })}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='monitor_setting.channel_error_status_codes'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Failure status codes')}</FormLabel>
                    <FormControl>
                      <Input
                        value={field.value}
                        onChange={(event) => field.onChange(event.target.value)}
                        disabled={!circuitBreakerEnabled}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Default: {{value}}', { value: '429,503' })}{' '}
                      {parsedStatusCodes.ok &&
                        parsedStatusCodes.normalized !== field.value.trim() && (
                          <span>
                            {t('Normalized:')} {parsedStatusCodes.normalized}
                          </span>
                        )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>
          </div>

          <Separator />

          <div className='space-y-4'>
            <div>
              <h3 className='text-sm font-semibold'>
                {t('WeCom bot notifications')}
              </h3>
              <p className='text-muted-foreground mt-1 text-xs'>
                {t(
                  'Send text alerts when a channel-model route is disabled or recovered.'
                )}
              </p>
            </div>
            <div className='grid gap-6 lg:grid-cols-2'>
              <FormField
                control={form.control}
                name='monitor_setting.channel_model_wecom_bot_enabled'
                render={({ field }) => (
                  <SettingsSwitchItem className='lg:col-span-2'>
                    <SettingsSwitchContent>
                      <FormLabel>{t('Enable WeCom bot notifications')}</FormLabel>
                      <FormDescription>
                        {t('Each bot is limited to 20 messages per rolling minute.')}
                      </FormDescription>
                    </SettingsSwitchContent>
                    <FormControl>
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                      />
                    </FormControl>
                  </SettingsSwitchItem>
                )}
              />
              <FormField
                control={form.control}
                name='monitor_setting.channel_model_wecom_bot_url'
                render={({ field }) => (
                  <FormItem className='lg:col-span-2'>
                    <FormLabel>{t('Alert webhook URL')}</FormLabel>
                    <FormControl>
                      <Input
                        type='url'
                        {...field}
                        disabled={!weComBotEnabled}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Default: {{value}}', {
                        value:
                          'https://alertplus.intsig.net/api/alarm/receive/',
                      })}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='monitor_setting.channel_model_wecom_bot_webhook_key'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Webhook Key')}</FormLabel>
                    <FormControl>
                      <PasswordInput
                        autoComplete='new-password'
                        placeholder={t('Enter new key to update')}
                        {...field}
                        disabled={!weComBotEnabled}
                      />
                    </FormControl>
                    <FormDescription>
                      {weComBotWebhookKeyConfigured
                        ? t('Configured; leave blank to keep the existing key')
                        : t('Not configured')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='monitor_setting.channel_model_wecom_bot_contact'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Mention contacts')}</FormLabel>
                    <FormControl>
                      <Input
                        placeholder={t('e.g. user@example.com|@all')}
                        {...field}
                        disabled={!weComBotEnabled}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Separate multiple contacts with |; use @all to mention everyone.')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>
          </div>

          <Separator />

          <div className='space-y-4'>
            <div>
              <h3 className='text-sm font-semibold'>{t('Automatic recovery')}</h3>
              <p className='text-muted-foreground mt-1 text-xs'>
                {t(
                  'Recovery probes run independently from regular channel health checks.'
                )}
              </p>
            </div>
            <div className='grid gap-6 lg:grid-cols-2'>
              <FormField
                control={form.control}
                name='monitor_setting.channel_model_recovery_enabled'
                render={({ field }) => (
                  <SettingsSwitchItem className='lg:col-span-2'>
                    <SettingsSwitchContent>
                      <FormLabel>{t('Enable automatic recovery')}</FormLabel>
                      <FormDescription>
                        {t(
                          'Probe disabled model routes and restore them after enough consecutive successes.'
                        )}
                      </FormDescription>
                    </SettingsSwitchContent>
                    <FormControl>
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                        disabled={!circuitBreakerEnabled}
                      />
                    </FormControl>
                  </SettingsSwitchItem>
                )}
              />
              <FormField
                control={form.control}
                name='monitor_setting.channel_model_recovery_success_threshold'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Consecutive successes to recover')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={1}
                        max={MAX_SUCCESS_THRESHOLD}
                        {...safeNumberFieldProps(field)}
                        disabled={!recoveryFieldsEnabled}
                      />
                    </FormControl>
                    <FormDescription>{t('Default: {{value}}', { value: 2 })}</FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='monitor_setting.channel_model_recovery_delay_minutes'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Wait before first probe (minutes)')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={0}
                        max={MAX_DELAY_MINUTES}
                        {...safeNumberFieldProps(field)}
                        disabled={!recoveryFieldsEnabled}
                      />
                    </FormControl>
                    <FormDescription>{t('Default: {{value}}', { value: 5 })}</FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='monitor_setting.channel_model_recovery_interval_minutes'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Recovery probe interval (minutes)')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={1}
                        max={MAX_INTERVAL_MINUTES}
                        {...safeNumberFieldProps(field)}
                        disabled={!recoveryFieldsEnabled}
                      />
                    </FormControl>
                    <FormDescription>{t('Default: {{value}}', { value: 1 })}</FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='monitor_setting.channel_model_recovery_concurrency'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Recovery probe concurrency')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={1}
                        max={MAX_CONCURRENCY}
                        {...safeNumberFieldProps(field)}
                        disabled={!recoveryFieldsEnabled}
                      />
                    </FormControl>
                    <FormDescription>{t('Default: {{value}}', { value: 4 })}</FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='monitor_setting.channel_model_event_retention_days'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Event log retention (days)')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={1}
                        max={MAX_RETENTION_DAYS}
                        {...safeNumberFieldProps(field)}
                      />
                    </FormControl>
                    <FormDescription>{t('Default: {{value}}', { value: 30 })}</FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>
          </div>
          </SettingsForm>
        </Form>
      </TabsContent>

      <TabsContent value='logs' className='pt-4'>
        <ChannelModelEventLogs />
      </TabsContent>
    </Tabs>
  )
}
