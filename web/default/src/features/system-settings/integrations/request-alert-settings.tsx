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
import { useMutation, useQuery } from '@tanstack/react-query'
import { nanoid } from 'nanoid'
import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Field,
  FieldContent,
  FieldDescription,
  FieldGroup,
  FieldLabel,
} from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Spinner } from '@/components/ui/spinner'
import { Switch } from '@/components/ui/switch'

import {
  getRequestAlertSetting,
  testRequestAlert,
  updateRequestAlertSetting,
  type RequestAlertLevel,
  type RequestAlertRule,
  type RequestAlertSetting,
} from './request-alert-api'
import { RequestAlertRuleEditor } from './request-alert-rule-editor'

const requestAlertQueryKey = ['request-alert-setting'] as const

function createRequestAlertRule(): RequestAlertRule {
  return {
    id: nanoid(),
    name: '',
    enabled: true,
    target_type: 'user',
    target_ids: [],
    categories: [
      'http_429',
      'http_5xx',
      'network_error',
      'response_parse_error',
      'empty_response',
      'stream_error',
      'insufficient_quota',
      'token_expired',
      'token_exhausted',
    ],
    level: 'warning',
  }
}

export function RequestAlertSettings() {
  const { t } = useTranslation()
  const initializedRef = useRef(false)
  const [draft, setDraft] = useState<RequestAlertSetting | null>(null)
  const [webhookReplacement, setWebhookReplacement] = useState('')
  const [secretReplacement, setSecretReplacement] = useState('')
  const [clearCredentials, setClearCredentials] = useState(false)

  const settingQuery = useQuery({
    queryKey: requestAlertQueryKey,
    queryFn: getRequestAlertSetting,
    staleTime: 5 * 60 * 1000,
  })

  useEffect(() => {
    if (!initializedRef.current && settingQuery.data?.data) {
      initializedRef.current = true
      setDraft(settingQuery.data.data)
    }
  }, [settingQuery.data])

  const updateMutation = useMutation({
    mutationFn: updateRequestAlertSetting,
    onSuccess: async (response) => {
      if (!response.success) return
      toast.success(t('Request alert settings saved'))
      setWebhookReplacement('')
      setSecretReplacement('')
      setClearCredentials(false)
      const refreshed = await settingQuery.refetch()
      if (refreshed.data?.data) setDraft(refreshed.data.data)
    },
  })

  const testMutation = useMutation({
    mutationFn: testRequestAlert,
    onSuccess: (response) => {
      if (response.success) toast.success(t('Test alert sent'))
    },
  })

  if (settingQuery.isLoading || !draft) {
    return (
      <div className='flex items-center gap-2 py-4 text-sm'>
        <Spinner />
        {t('Loading request alert settings...')}
      </div>
    )
  }

  const updateRule = (index: number, rule: RequestAlertRule) => {
    setDraft((current) => {
      if (!current) return current
      const rules = [...current.rules]
      rules[index] = rule
      return { ...current, rules }
    })
  }

  const deleteRule = (index: number) => {
    setDraft((current) => {
      if (!current) return current
      return {
        ...current,
        rules: current.rules.filter((_, ruleIndex) => ruleIndex !== index),
      }
    })
  }

  const save = () => {
    const hasWebhook =
      webhookReplacement.trim() !== '' ||
      (draft.webhook_configured && !clearCredentials)
    if (draft.enabled && !hasWebhook) {
      toast.error(t('Configure a Feishu webhook before enabling alerts'))
      return
    }
    if (
      !Number.isInteger(draft.aggregation_window_minutes) ||
      draft.aggregation_window_minutes < 1 ||
      draft.aggregation_window_minutes > 60
    ) {
      toast.error(t('Aggregation window must be between 1 and 60 minutes'))
      return
    }
    const invalidRule = draft.rules.find(
      (rule) =>
        !rule.name.trim() ||
        rule.target_ids.length === 0 ||
        rule.categories.length === 0
    )
    if (invalidRule) {
      toast.error(
        t('Complete the name, targets, and categories for every rule')
      )
      return
    }

    updateMutation.mutate({
      enabled: draft.enabled,
      aggregation_window_minutes: draft.aggregation_window_minutes,
      rules: draft.rules,
      feishu_webhook_url: webhookReplacement.trim() || undefined,
      feishu_secret: secretReplacement.trim() || undefined,
      clear_webhook: clearCredentials,
      clear_secret: clearCredentials,
    })
  }

  const sendTest = (level: RequestAlertLevel) => {
    testMutation.mutate(level)
  }

  return (
    <FieldGroup className='gap-5'>
      <div>
        <div className='flex flex-wrap items-center gap-2'>
          <h4 className='font-medium'>{t('Request anomaly alerts')}</h4>
          <Badge variant={draft.enabled ? 'default' : 'secondary'}>
            {draft.enabled ? t('Enabled') : t('Disabled')}
          </Badge>
        </div>
        <p className='text-muted-foreground mt-1 text-xs'>
          {t(
            'Alert on final Chat, Responses, Claude Messages, and Gemini text-generation anomalies.'
          )}
        </p>
      </div>

      <Alert>
        <AlertTitle>{t('Feishu custom bot')}</AlertTitle>
        <AlertDescription>
          {draft.webhook_configured && !clearCredentials
            ? t('Configured webhook: {{webhook}}', {
                webhook: draft.webhook_masked,
              })
            : t('No Feishu webhook is configured')}
          {' · '}
          {draft.secret_configured && !clearCredentials
            ? t('Signature verification is configured')
            : t('Signature verification is not configured')}
        </AlertDescription>
      </Alert>

      <Field orientation='horizontal'>
        <FieldContent>
          <FieldLabel htmlFor='request-alert-enabled'>
            {t('Enable request anomaly alerts')}
          </FieldLabel>
          <FieldDescription>
            {t('Rules use OR matching; urgent wins when multiple rules match.')}
          </FieldDescription>
        </FieldContent>
        <Switch
          id='request-alert-enabled'
          checked={draft.enabled}
          onCheckedChange={(enabled) =>
            setDraft((current) => (current ? { ...current, enabled } : current))
          }
        />
      </Field>

      <div className='grid grid-cols-1 gap-4 md:grid-cols-3'>
        <Field>
          <FieldLabel htmlFor='request-alert-window'>
            {t('Aggregation window (minutes)')}
          </FieldLabel>
          <Input
            id='request-alert-window'
            type='number'
            min={1}
            max={60}
            value={draft.aggregation_window_minutes}
            onChange={(event) =>
              setDraft((current) =>
                current
                  ? {
                      ...current,
                      aggregation_window_minutes: Number(event.target.value),
                    }
                  : current
              )
            }
          />
          <FieldDescription>
            {t(
              'The first alert is immediate; similar alerts are summarized later.'
            )}
          </FieldDescription>
        </Field>

        <Field>
          <FieldLabel htmlFor='request-alert-webhook'>
            {t('New Feishu webhook URL')}
          </FieldLabel>
          <Input
            id='request-alert-webhook'
            type='url'
            value={webhookReplacement}
            onChange={(event) => setWebhookReplacement(event.target.value)}
            placeholder={t('Leave blank to keep the configured webhook')}
            autoComplete='off'
          />
        </Field>

        <Field>
          <FieldLabel htmlFor='request-alert-secret'>
            {t('New signing secret')}
          </FieldLabel>
          <Input
            id='request-alert-secret'
            type='password'
            value={secretReplacement}
            onChange={(event) => setSecretReplacement(event.target.value)}
            placeholder={t('Leave blank to keep the configured secret')}
            autoComplete='new-password'
          />
        </Field>
      </div>

      <div className='flex flex-wrap gap-2'>
        <Button
          type='button'
          variant='outline'
          onClick={() => sendTest('warning')}
          disabled={
            testMutation.isPending ||
            !draft.webhook_configured ||
            clearCredentials
          }
        >
          {testMutation.isPending && <Spinner data-icon='inline-start' />}
          {t('Test warning')}
        </Button>
        <Button
          type='button'
          variant='outline'
          onClick={() => sendTest('urgent')}
          disabled={
            testMutation.isPending ||
            !draft.webhook_configured ||
            clearCredentials
          }
        >
          {testMutation.isPending && <Spinner data-icon='inline-start' />}
          {t('Test urgent')}
        </Button>
        <Button
          type='button'
          variant='destructive'
          onClick={() => {
            setClearCredentials(true)
            setWebhookReplacement('')
            setSecretReplacement('')
            setDraft((current) =>
              current ? { ...current, enabled: false } : current
            )
          }}
          disabled={!draft.webhook_configured && !draft.secret_configured}
        >
          {t('Clear bot configuration')}
        </Button>
      </div>

      <div className='flex items-center justify-between gap-3'>
        <div>
          <h5 className='text-sm font-medium'>{t('Alert rules')}</h5>
          <p className='text-muted-foreground text-xs'>
            {t(
              'User, API key, and channel rules are configured independently.'
            )}
          </p>
        </div>
        <Button
          type='button'
          variant='outline'
          onClick={() =>
            setDraft((current) =>
              current
                ? {
                    ...current,
                    rules: [...current.rules, createRequestAlertRule()],
                  }
                : current
            )
          }
        >
          {t('Add rule')}
        </Button>
      </div>

      {draft.rules.map((rule, index) => (
        <RequestAlertRuleEditor
          key={rule.id || index}
          rule={rule}
          onChange={(nextRule) => updateRule(index, nextRule)}
          onDelete={() => deleteRule(index)}
        />
      ))}

      {draft.rules.length === 0 && (
        <Alert>
          <AlertTitle>{t('No alert rules')}</AlertTitle>
          <AlertDescription>
            {t('Add a rule to select users, API keys, or channels to monitor.')}
          </AlertDescription>
        </Alert>
      )}

      <div className='flex justify-end'>
        <Button
          type='button'
          onClick={save}
          disabled={updateMutation.isPending}
        >
          {updateMutation.isPending && <Spinner data-icon='inline-start' />}
          {t('Save request alert settings')}
        </Button>
      </div>
    </FieldGroup>
  )
}
