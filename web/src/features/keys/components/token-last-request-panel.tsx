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
import { useId } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { toIntlLocale } from '@/i18n/languages'
import { copyToClipboard } from '@/lib/copy-to-clipboard'

import { getTokenLastRequest } from '../api'

type TokenLastRequestPanelProps = { tokenId: number; open: boolean }

export function TokenLastRequestPanel(props: TokenLastRequestPanelProps) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const questionId = useId()
  const answerId = useId()
  const {
    data: record,
    isPending,
    isError,
    isFetching,
    refetch,
  } = useQuery({
    queryKey: ['token-last-request', props.tokenId],
    queryFn: () => getTokenLastRequest(props.tokenId),
    enabled: props.open,
    staleTime: 0,
    refetchInterval: props.open ? 5000 : false,
  })

  async function copyText(text: string) {
    if (await copyToClipboard(text)) toast.success(t('Copied'))
    else toast.error(t('Copy failed'))
  }

  let statusLabel = t('Request in progress')
  if (record?.status === 'completed') statusLabel = t('Request completed')
  if (record?.status === 'failed') statusLabel = t('Request failed')
  if (record?.status === 'cancelled') statusLabel = t('Request cancelled')

  return (
    <section
      className='space-y-3 rounded-lg border p-3'
      aria-label={t('Latest question and answer')}
    >
      <div className='flex flex-wrap items-center justify-between gap-2'>
        <h3 className='text-sm font-medium'>
          {t('Latest question and answer')}
        </h3>
        <Button
          type='button'
          variant='outline'
          size='sm'
          disabled={isFetching}
          onClick={() => void refetch()}
        >
          {t('Refresh')}
        </Button>
      </div>
      {isPending && (
        <p className='text-muted-foreground text-sm'>{t('Loading...')}</p>
      )}
      {isError && (
        <p role='alert' className='text-destructive text-sm'>
          {t('Failed to load latest request')}
        </p>
      )}
      {!isPending && !isError && !record && (
        <p className='text-muted-foreground text-sm'>
          {t('No request recorded yet')}
        </p>
      )}
      {record && (
        <>
          <p className='text-muted-foreground text-xs break-all'>
            {record.model} ·{' '}
            {new Date(record.created_at).toLocaleString(locale)} · {statusLabel}
          </p>
          <div className='space-y-2'>
            <div className='flex items-center justify-between gap-2'>
              <Label htmlFor={questionId}>{t('Latest question')}</Label>
              <Button
                type='button'
                variant='ghost'
                size='sm'
                disabled={!record.question}
                onClick={() => void copyText(record.question)}
              >
                {t('Copy question')}
              </Button>
            </div>
            <Textarea
              id={questionId}
              value={record.question}
              readOnly
              rows={4}
              placeholder={t('No text content')}
            />
          </div>
          <div className='space-y-2'>
            <div className='flex items-center justify-between gap-2'>
              <Label htmlFor={answerId}>{t('Model answer')}</Label>
              <Button
                type='button'
                variant='ghost'
                size='sm'
                disabled={!record.answer}
                onClick={() => void copyText(record.answer)}
              >
                {t('Copy answer')}
              </Button>
            </div>
            <Textarea
              id={answerId}
              value={record.answer}
              readOnly
              rows={6}
              placeholder={t('No text content')}
            />
          </div>
          {record.truncated && (
            <p className='text-muted-foreground text-xs'>
              {t(
                'This record was truncated because it exceeded the recording limit.'
              )}
            </p>
          )}
          <Button
            type='button'
            variant='outline'
            size='sm'
            disabled={!record.question && !record.answer}
            onClick={() =>
              void copyText(
                `${t('Latest question')}:\n${record.question}\n\n${t('Model answer')}:\n${record.answer}`
              )
            }
          >
            {t('Copy question and answer')}
          </Button>
        </>
      )}
    </section>
  )
}
