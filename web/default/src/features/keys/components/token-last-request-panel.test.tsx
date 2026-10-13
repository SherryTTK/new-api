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
import assert from 'node:assert/strict'
import { test } from 'node:test'

import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { createInstance } from 'i18next'
import { renderToStaticMarkup } from 'react-dom/server'
import { I18nextProvider } from 'react-i18next'

import { TokenLastRequestPanel } from './token-last-request-panel'

for (const [language, locale] of [
  ['zhCN', 'zh-CN'],
  ['zhTW', 'zh-TW'],
  ['en', 'en'],
]) {
  test(`renders recorded question, answer and timestamp in ${language}`, async () => {
    const i18n = createInstance()
    await i18n.init({
      lng: language,
      fallbackLng: false,
      resources: { [language]: { translation: { Refresh: 'Refresh' } } },
    })
    const client = new QueryClient()
    const createdAt = Date.UTC(2026, 8, 27, 12)
    client.setQueryData(['token-last-request', 1], {
      token_id: 1,
      question: 'What is 2 + 2?',
      answer: 'The answer is 4.',
      model: 'test-model',
      created_at: createdAt,
      status: 'completed',
      truncated: false,
    })

    try {
      const html = renderToStaticMarkup(
        <I18nextProvider i18n={i18n}>
          <QueryClientProvider client={client}>
            <TokenLastRequestPanel tokenId={1} open={false} />
          </QueryClientProvider>
        </I18nextProvider>
      )
      assert.ok(html.includes('What is 2 + 2?'))
      assert.ok(html.includes('The answer is 4.'))
      assert.ok(html.includes('test-model'))
      assert.ok(html.includes(new Date(createdAt).toLocaleString(locale)))
    } finally {
      client.clear()
    }
  })
}
