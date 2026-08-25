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
import { useDeferredValue, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { MultiSelect } from '@/components/multi-select'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  Field,
  FieldDescription,
  FieldGroup,
  FieldLabel,
} from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { Switch } from '@/components/ui/switch'

import {
  searchRequestAlertTargets,
  type RequestAlertCategory,
  type RequestAlertLevel,
  type RequestAlertRule,
  type RequestAlertTargetType,
} from './request-alert-api'

const categoryKeys: Array<{
  value: RequestAlertCategory
  label: string
}> = [
  { value: 'http_429', label: 'Upstream 429' },
  { value: 'http_5xx', label: 'Upstream 5xx' },
  { value: 'network_error', label: 'Network, DNS, or timeout error' },
  { value: 'response_parse_error', label: 'Response read or parse error' },
  { value: 'empty_response', label: 'Empty response' },
  { value: 'stream_error', label: 'Abnormal stream termination' },
  { value: 'insufficient_quota', label: 'Insufficient user quota' },
  { value: 'token_expired', label: 'API key expired' },
  { value: 'token_exhausted', label: 'API key quota exhausted' },
]

type RequestAlertRuleEditorProps = {
  rule: RequestAlertRule
  onChange: (rule: RequestAlertRule) => void
  onDelete: () => void
}

export function RequestAlertRuleEditor(props: RequestAlertRuleEditorProps) {
  const { t } = useTranslation()
  const [targetKeyword, setTargetKeyword] = useState('')
  const deferredKeyword = useDeferredValue(targetKeyword.trim())
  const targetQuery = useQuery({
    queryKey: [
      'request-alert-targets',
      props.rule.target_type,
      deferredKeyword,
    ],
    queryFn: () =>
      searchRequestAlertTargets(props.rule.target_type, deferredKeyword),
    staleTime: 60 * 1000,
  })

  const targetOptions = useMemo(() => {
    const labels = new Map(
      (targetQuery.data?.data ?? []).map((target) => [
        String(target.id),
        target.description
          ? `${target.label} · ${target.description}`
          : target.label,
      ])
    )
    for (const id of props.rule.target_ids) {
      if (!labels.has(String(id))) labels.set(String(id), `#${id}`)
    }
    return Array.from(labels, ([value, label]) => ({ value, label }))
  }, [props.rule.target_ids, targetQuery.data?.data])

  const categoryOptions = useMemo(
    () =>
      categoryKeys.map((category) => ({
        value: category.value,
        label: t(category.label),
      })),
    [t]
  )

  return (
    <Card size='sm'>
      <CardHeader>
        <CardTitle>{props.rule.name || t('New alert rule')}</CardTitle>
        <CardDescription>
          {t('A request is alerted when its category and target both match.')}
        </CardDescription>
        <CardAction className='flex items-center gap-2'>
          <Switch
            checked={props.rule.enabled}
            onCheckedChange={(enabled) =>
              props.onChange({ ...props.rule, enabled })
            }
            aria-label={t('Enable alert rule')}
          />
          <Button type='button' variant='destructive' onClick={props.onDelete}>
            {t('Delete')}
          </Button>
        </CardAction>
      </CardHeader>
      <CardContent>
        <FieldGroup className='gap-4'>
          <Field>
            <FieldLabel htmlFor={`request-alert-name-${props.rule.id}`}>
              {t('Rule name')}
            </FieldLabel>
            <Input
              id={`request-alert-name-${props.rule.id}`}
              value={props.rule.name}
              onChange={(event) =>
                props.onChange({ ...props.rule, name: event.target.value })
              }
              placeholder={t('For example: Production user alerts')}
            />
          </Field>

          <div className='grid grid-cols-1 gap-4 md:grid-cols-2'>
            <Field>
              <FieldLabel
                htmlFor={`request-alert-target-type-${props.rule.id}`}
              >
                {t('Target type')}
              </FieldLabel>
              <NativeSelect
                id={`request-alert-target-type-${props.rule.id}`}
                className='w-full'
                value={props.rule.target_type}
                onChange={(event) => {
                  setTargetKeyword('')
                  props.onChange({
                    ...props.rule,
                    target_type: event.target.value as RequestAlertTargetType,
                    target_ids: [],
                  })
                }}
              >
                <NativeSelectOption value='user'>
                  {t('User')}
                </NativeSelectOption>
                <NativeSelectOption value='token'>
                  {t('API key')}
                </NativeSelectOption>
                <NativeSelectOption value='channel'>
                  {t('Channel')}
                </NativeSelectOption>
              </NativeSelect>
            </Field>

            <Field>
              <FieldLabel htmlFor={`request-alert-level-${props.rule.id}`}>
                {t('Alert level')}
              </FieldLabel>
              <NativeSelect
                id={`request-alert-level-${props.rule.id}`}
                className='w-full'
                value={props.rule.level}
                onChange={(event) =>
                  props.onChange({
                    ...props.rule,
                    level: event.target.value as RequestAlertLevel,
                  })
                }
              >
                <NativeSelectOption value='warning'>
                  {t('Warning')}
                </NativeSelectOption>
                <NativeSelectOption value='urgent'>
                  {t('Urgent (@everyone)')}
                </NativeSelectOption>
              </NativeSelect>
            </Field>
          </div>

          <Field>
            <FieldLabel
              htmlFor={`request-alert-target-search-${props.rule.id}`}
            >
              {t('Search targets')}
            </FieldLabel>
            <Input
              id={`request-alert-target-search-${props.rule.id}`}
              value={targetKeyword}
              onChange={(event) => setTargetKeyword(event.target.value)}
              placeholder={t('Search by ID or name')}
            />
            <MultiSelect
              id={`request-alert-targets-${props.rule.id}`}
              options={targetOptions}
              selected={props.rule.target_ids.map(String)}
              onChange={(values) =>
                props.onChange({
                  ...props.rule,
                  target_ids: values
                    .map((value) => Number(value))
                    .filter((value) => Number.isInteger(value) && value > 0),
                })
              }
              placeholder={
                targetQuery.isFetching
                  ? t('Searching...')
                  : t('Select alert targets')
              }
              emptyText={t('No matching targets')}
              maxVisibleChips={6}
            />
            <FieldDescription>
              {t('Multiple selected targets are matched with OR logic.')}
            </FieldDescription>
          </Field>

          <Field>
            <FieldLabel htmlFor={`request-alert-categories-${props.rule.id}`}>
              {t('Alert categories')}
            </FieldLabel>
            <MultiSelect
              id={`request-alert-categories-${props.rule.id}`}
              options={categoryOptions}
              selected={props.rule.categories}
              onChange={(values) =>
                props.onChange({
                  ...props.rule,
                  categories: values as RequestAlertCategory[],
                })
              }
              placeholder={t('Select alert categories')}
              maxVisibleChips={5}
            />
          </Field>
        </FieldGroup>
      </CardContent>
    </Card>
  )
}
