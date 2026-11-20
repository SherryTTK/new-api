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
import { useQuery } from '@tanstack/react-query'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'

import { ErrorState } from '@/components/error-state'
import { MultiSelect } from '@/components/multi-select'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { getGroups } from '@/features/users/api'
import { handleServerError } from '@/lib/handle-server-error'
import { requireServerSuccess } from '@/lib/server-error-message'

import { SettingsForm } from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import type { ClaudeCodeGateSettings } from './defaults'
import { useSavePolicy } from './use-save-policy'

const modeSchema = z.enum(['off', 'shadow', 'enforce'])
const gateSchema = z.object({
  mode: modeSchema,
  groups: z.array(z.string().min(1)),
  bodyMode: modeSchema,
})
type GateFormValues = z.infer<typeof gateSchema>

export function ClaudeCodeGateSection(props: {
  defaultValues: ClaudeCodeGateSettings
}) {
  const { t } = useTranslation()
  const save = useSavePolicy()
  const groups = useQuery({
    queryKey: ['groups'],
    queryFn: async () => requireServerSuccess(await getGroups()),
  })
  const form = useForm<GateFormValues>({
    resolver: zodResolver(gateSchema),
    values: {
      mode: props.defaultValues['claude_code_gate.mode'],
      groups: props.defaultValues['claude_code_gate.groups'],
      bodyMode: props.defaultValues['claude_code_gate.body_mode'],
    },
  })
  const mode = form.watch('mode')
  const options = (groups.data?.data ?? [])
    .filter((group) => group !== 'auto')
    .map((group) => ({ value: group, label: group }))
  const onSubmit = async (values: GateFormValues) => {
    try {
      await save.mutateAsync({
        'claude_code_gate.mode': values.mode,
        'claude_code_gate.groups': JSON.stringify(values.groups),
        'claude_code_gate.body_mode': values.bodyMode,
      })
      form.reset(values)
    } catch (error) {
      handleServerError(error)
    }
  }

  return (
    <SettingsSection title={t('Claude Code access gate')}>
      <p className='text-muted-foreground text-sm'>
        {t(
          'In selected groups, only Claude models require POST /v1/messages with Claude Code request fingerprints. Other models and model lists are unaffected. These fingerprints can be copied and do not authenticate the client.'
        )}
      </p>
      <p className='text-muted-foreground text-sm'>
        {t(
          'Claude models are identified by the requested name (claude-* or a provider-prefixed Claude name), before channel model mapping.'
        )}
      </p>
      {groups.isError && (
        <ErrorState
          description={t('Failed to load groups')}
          onRetry={() => void groups.refetch()}
        />
      )}
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={form.formState.isSubmitting}
            isSaveDisabled={!form.formState.isDirty}
          />
          <FormField
            control={form.control}
            name='mode'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Gate mode')}</FormLabel>
                <FormControl>
                  <NativeSelect {...field}>
                    <NativeSelectOption value='off'>
                      {t('Disabled')}
                    </NativeSelectOption>
                    <NativeSelectOption value='shadow'>
                      {t('Observe only')}
                    </NativeSelectOption>
                    <NativeSelectOption value='enforce'>
                      {t('Enforce')}
                    </NativeSelectOption>
                  </NativeSelect>
                </FormControl>
                <FormDescription>
                  {t(
                    'Observe only records mismatches without blocking requests or changing group selection.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='groups'
            render={({ field }) => (
              <FormItem data-settings-form-span='full'>
                <FormLabel>{t('Restricted groups')}</FormLabel>
                <FormControl>
                  <MultiSelect
                    aria-label={t('Restricted groups')}
                    options={options}
                    selected={field.value}
                    onChange={field.onChange}
                    placeholder={t('Select groups')}
                    disabled={
                      groups.isPending ||
                      groups.isError ||
                      form.formState.isSubmitting
                    }
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'An empty selection disables the gate. For Claude requests, enforced groups are excluded from Auto selection and cannot be used in Playground. Other models can use these groups normally.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='bodyMode'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Body fingerprint checks')}</FormLabel>
                <FormControl>
                  <NativeSelect {...field} disabled={mode === 'off'}>
                    <NativeSelectOption value='off'>
                      {t('Disabled')}
                    </NativeSelectOption>
                    <NativeSelectOption value='shadow'>
                      {t('Observe only')}
                    </NativeSelectOption>
                    <NativeSelectOption value='enforce'>
                      {t('Enforce')}
                    </NativeSelectOption>
                  </NativeSelect>
                </FormControl>
                <FormDescription>
                  {t(
                    'Requires both metadata.user_id containing device_id and session_id, and a billing marker in system text. The gate must be enforced to block body mismatches.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
