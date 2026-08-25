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
import { api } from '@/lib/api'

export type RequestAlertTargetType = 'user' | 'token' | 'channel'
export type RequestAlertLevel = 'warning' | 'urgent'

export type RequestAlertCategory =
  | 'http_429'
  | 'http_5xx'
  | 'network_error'
  | 'response_parse_error'
  | 'empty_response'
  | 'stream_error'
  | 'insufficient_quota'
  | 'token_expired'
  | 'token_exhausted'

export type RequestAlertRule = {
  id: string
  name: string
  enabled: boolean
  target_type: RequestAlertTargetType
  target_ids: number[]
  categories: RequestAlertCategory[]
  level: RequestAlertLevel
}

export type RequestAlertSetting = {
  enabled: boolean
  aggregation_window_minutes: number
  rules: RequestAlertRule[]
  webhook_configured: boolean
  webhook_masked: string
  secret_configured: boolean
}

type RequestAlertRuleResponse = Omit<
  RequestAlertRule,
  'target_ids' | 'categories'
> & {
  target_ids: number[] | null
  categories: RequestAlertCategory[] | null
}

type RequestAlertSettingResponse = Omit<RequestAlertSetting, 'rules'> & {
  rules: RequestAlertRuleResponse[] | null
}

export type RequestAlertTarget = {
  id: number
  label: string
  description: string
}

type ApiResponse<T> = {
  success: boolean
  message: string
  data?: T
}

export type UpdateRequestAlertSetting = {
  enabled: boolean
  aggregation_window_minutes: number
  rules: RequestAlertRule[]
  feishu_webhook_url?: string
  feishu_secret?: string
  clear_webhook?: boolean
  clear_secret?: boolean
}

export async function getRequestAlertSetting() {
  const response = await api.get<ApiResponse<RequestAlertSettingResponse>>(
    '/api/option/request-alert'
  )
  const payload = response.data
  return {
    ...payload,
    data: payload.data
      ? {
          ...payload.data,
          rules: (payload.data.rules ?? []).map((rule) => ({
            ...rule,
            target_ids: rule.target_ids ?? [],
            categories: rule.categories ?? [],
          })),
        }
      : undefined,
  }
}

export async function updateRequestAlertSetting(
  request: UpdateRequestAlertSetting
) {
  const response = await api.put<ApiResponse<undefined>>(
    '/api/option/request-alert',
    request
  )
  return response.data
}

export async function testRequestAlert(level: RequestAlertLevel) {
  const response = await api.post<ApiResponse<undefined>>(
    '/api/option/request-alert/test',
    { level }
  )
  return response.data
}

export async function searchRequestAlertTargets(
  type: RequestAlertTargetType,
  keyword: string
) {
  const response = await api.get<ApiResponse<RequestAlertTarget[]>>(
    '/api/option/request-alert/targets',
    { params: { type, keyword } }
  )
  return response.data
}
