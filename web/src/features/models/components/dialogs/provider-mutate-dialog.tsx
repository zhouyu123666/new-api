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
import { zodResolver } from '@hookform/resolvers/zod'
import { useQueryClient } from '@tanstack/react-query'
import { Loader2 } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { z } from 'zod'

import {
  SideDrawerSection,
  sideDrawerContentClassName,
  sideDrawerFooterClassName,
  sideDrawerFormClassName,
} from '@/components/drawer-layout'
import { getLobeIcon } from '@/lib/lobe-icon'
import { invalidateModelSquareQueries } from '@/features/pricing/lib/query-keys'
import { Button } from '@/components/ui/button'
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
import { Sheet, SheetContent, SheetDescription, SheetFooter, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'

import { createProvider, updateProvider } from '../../api'
import { providersQueryKeys } from '../../lib'
import type { Provider } from '../../types'

const providerFormSchema = z.object({
  slug: z.string().trim().min(1),
  display_name: z.string().trim().min(1),
  icon: z.string().optional(),
  website_url: z.string().optional(),
  status_page_url: z.string().optional(),
  headquarters: z.string().optional(),
  prompt_training_policy: z.string().optional(),
  retention_policy: z.string().optional(),
  moderation_policy: z.string().optional(),
  metadata_source_url: z.string().optional(),
  status: z.boolean(),
})

type ProviderFormValues = z.infer<typeof providerFormSchema>

export function ProviderMutateDialog(props: {
  open: boolean
  onOpenChange: (open: boolean) => void
  currentProvider?: Provider | null
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const isEdit = Boolean(props.currentProvider?.id)
  const [isSaving, setIsSaving] = useState(false)
  const form = useForm<ProviderFormValues>({
    resolver: zodResolver(providerFormSchema),
    defaultValues: {
      slug: '', display_name: '', icon: '', website_url: '', status_page_url: '',
      headquarters: '', prompt_training_policy: '', retention_policy: '',
      moderation_policy: '', metadata_source_url: '', status: true,
    },
  })

  useEffect(() => {
    if (!props.open) return
    const provider = props.currentProvider
    form.reset(provider ? {
      slug: provider.slug,
      display_name: provider.display_name,
      icon: provider.icon || '',
      website_url: provider.website_url || '',
      status_page_url: provider.status_page_url || '',
      headquarters: provider.headquarters || '',
      prompt_training_policy: provider.prompt_training_policy || '',
      retention_policy: provider.retention_policy || '',
      moderation_policy: provider.moderation_policy || '',
      metadata_source_url: provider.metadata_source_url || '',
      status: provider.status === 1,
    } : {
      slug: '', display_name: '', icon: '', website_url: '', status_page_url: '',
      headquarters: '', prompt_training_policy: '', retention_policy: '',
      moderation_policy: '', metadata_source_url: '', status: true,
    })
  }, [form, props.currentProvider, props.open])

  const onSubmit = async (values: ProviderFormValues) => {
    setIsSaving(true)
    try {
      const payload = { ...values, status: values.status ? 1 : 0 }
      const response = isEdit
        ? await updateProvider({ ...payload, id: props.currentProvider?.id ?? 0 })
        : await createProvider(payload)
      if (!response.success) throw new Error(response.message || t('Operation failed'))
      toast.success(isEdit ? t('Provider updated successfully') : t('Provider created successfully'))
      queryClient.invalidateQueries({ queryKey: providersQueryKeys.lists() })
      await invalidateModelSquareQueries(queryClient)
      props.onOpenChange(false)
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('Operation failed'))
    } finally {
      setIsSaving(false)
    }
  }

  return (
    <Sheet open={props.open} onOpenChange={props.onOpenChange}>
      <SheetContent side='right' className={sideDrawerContentClassName('sm:max-w-xl')}>
        <SheetHeader>
          <SheetTitle>{isEdit ? t('Edit Provider') : t('Create Provider')}</SheetTitle>
          <SheetDescription>{t('Manage provider metadata shown in the model square.')}</SheetDescription>
        </SheetHeader>
        <Form {...form}>
          <form id='provider-mutate-form' onSubmit={form.handleSubmit(onSubmit)} className={sideDrawerFormClassName()}>
            <SideDrawerSection>
              <h3 className='text-sm font-semibold'>{t('Basic Information')}</h3>
              <div className='grid gap-4 sm:grid-cols-2'>
                <FormField control={form.control} name='slug' render={({ field }) => (
                  <FormItem><FormLabel>{t('Provider slug')}</FormLabel><FormControl><Input placeholder='openai' {...field} disabled={isEdit} /></FormControl><FormDescription>{t('Stable identifier used for routing')}</FormDescription><FormMessage /></FormItem>
                )} />
                <FormField control={form.control} name='display_name' render={({ field }) => (
                  <FormItem><FormLabel>{t('Display name')}</FormLabel><FormControl><Input placeholder='OpenAI' {...field} /></FormControl><FormDescription>{t('Name shown to end users')}</FormDescription><FormMessage /></FormItem>
                )} />
              </div>
              <FormField control={form.control} name='icon' render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Icon')}</FormLabel>
                  <div className='flex items-center gap-2'>
                    <div className='bg-muted flex size-9 shrink-0 items-center justify-center rounded-md'>{field.value ? getLobeIcon(field.value, 22) : <span className='text-muted-foreground text-xs'>?</span>}</div>
                    <FormControl><Input placeholder='OpenAI' {...field} /></FormControl>
                  </div>
                  <FormDescription>{t('@lobehub/icons key')}</FormDescription><FormMessage />
                </FormItem>
              )} />
              <div className='grid gap-4 sm:grid-cols-2'>
                {([
                  ['website_url', t('Website URL')], ['status_page_url', t('Status page URL')],
                  ['headquarters', t('Headquarters')], ['metadata_source_url', t('Metadata source URL')],
                ] as const).map(([name, label]) => (
                  <FormField key={name} control={form.control} name={name} render={({ field }) => (
                    <FormItem><FormLabel>{label}</FormLabel><FormControl><Input {...field} /></FormControl><FormMessage /></FormItem>
                  )} />
                ))}
              </div>
            </SideDrawerSection>
            <SideDrawerSection>
              <h3 className='text-sm font-semibold'>{t('Data policy')}</h3>
              {([
                ['prompt_training_policy', t('Prompt training policy')], ['retention_policy', t('Retention policy')], ['moderation_policy', t('Moderation policy')],
              ] as const).map(([name, label]) => (
                <FormField key={name} control={form.control} name={name} render={({ field }) => (
                  <FormItem><FormLabel>{label}</FormLabel><FormControl><Textarea rows={2} {...field} /></FormControl><FormMessage /></FormItem>
                )} />
              ))}
              <FormField control={form.control} name='status' render={({ field }) => (
                <FormItem className='flex items-center justify-between rounded-lg border px-3 py-2'><div><FormLabel>{t('Enabled')}</FormLabel><FormDescription>{t('Show this provider in the model square')}</FormDescription></div><FormControl><Switch checked={field.value} onCheckedChange={field.onChange} /></FormControl></FormItem>
              )} />
            </SideDrawerSection>
          </form>
        </Form>
        <SheetFooter className={sideDrawerFooterClassName()}>
          <Button variant='outline' onClick={() => props.onOpenChange(false)} disabled={isSaving}>{t('Cancel')}</Button>
          <Button type='submit' form='provider-mutate-form' disabled={isSaving}>{isSaving && <Loader2 className='size-4 animate-spin' />}{t('Save')}</Button>
        </SheetFooter>
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
