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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  RouterProvider,
} from '@tanstack/react-router'
import {
  act,
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'
import { Route as ModelsRoute } from '@/routes/_authenticated/system-settings/models/$section'
import { Route as OperationsRoute } from '@/routes/_authenticated/system-settings/operations/$section'
import { Route as PoliciesRoute } from '@/routes/_authenticated/system-settings/request-policies/$section'
import { Route as PolicyIndexRoute } from '@/routes/_authenticated/system-settings/request-policies/index'
import { Route as SecurityRoute } from '@/routes/_authenticated/system-settings/security/$section'

import { RequestPolicies } from '..'
import {
  defaultRequestPolicySettings,
  type RequestPolicySettings,
} from '../defaults'

type PolicyBeforeLoad = (context: { params: { section: string } }) => void

let queryClient: QueryClient
let settings: RequestPolicySettings

function optionsResponse() {
  return {
    success: true,
    data: Object.entries(settings).map(([key, value]) => ({
      key,
      value: Array.isArray(value) ? JSON.stringify(value) : String(value),
    })),
  }
}

async function renderPolicies(path: string) {
  const root = createRootRoute()
  const authenticated = createRoute({
    getParentRoute: () => root,
    id: '_authenticated',
  })
  const policyRoute = createRoute({
    getParentRoute: () => authenticated,
    path: '/system-settings/request-policies/$section',
    beforeLoad: PoliciesRoute.options.beforeLoad as PolicyBeforeLoad,
    component: RequestPolicies,
  })
  const routes = [
    policyRoute,
    createRoute({
      getParentRoute: () => authenticated,
      path: '/system-settings/request-policies/',
      beforeLoad: PolicyIndexRoute.options.beforeLoad as () => void,
    }),
    createRoute({
      getParentRoute: () => authenticated,
      path: '/system-settings/models/$section',
      beforeLoad: ModelsRoute.options.beforeLoad as PolicyBeforeLoad,
    }),
    createRoute({
      getParentRoute: () => authenticated,
      path: '/system-settings/security/$section',
      beforeLoad: SecurityRoute.options.beforeLoad as PolicyBeforeLoad,
    }),
    createRoute({
      getParentRoute: () => authenticated,
      path: '/system-settings/operations/$section',
      beforeLoad: OperationsRoute.options.beforeLoad as PolicyBeforeLoad,
    }),
  ]
  const router = createRouter({
    routeTree: root.addChildren([authenticated.addChildren(routes)]),
    history: createMemoryHistory({ initialEntries: [path] }),
  })
  render(
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  )
  await act(() => router.load())
  return router
}

beforeEach(() => {
  vi.spyOn(window, 'scrollTo').mockImplementation(() => undefined)
  queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  settings = {
    ...defaultRequestPolicySettings,
    RetryTimes: 2,
    AutomaticRetryStatusCodes: '429,500-503',
    AutomaticDisableChannelEnabled: true,
    AutomaticEnableChannelEnabled: true,
    'monitor_setting.auto_test_channel_enabled': true,
    CheckSensitiveEnabled: true,
    CheckSensitiveOnPromptEnabled: true,
    SensitiveWords: 'blocked',
    'channel_affinity_setting.rules': JSON.stringify([
      {
        name: 'Existing session rule',
        model_regex: ['gpt-.*'],
        path_regex: ['/v1/chat/completions'],
        key_sources: [{ type: 'request_header', key: 'X-Session-ID' }],
        ttl_seconds: 600,
        skip_retry_on_failure: true,
        include_using_group: true,
        include_model_name: false,
        include_rule_name: true,
        param_override_template: { temperature: 0 },
      },
    ]),
  }
  vi.spyOn(api, 'patch').mockResolvedValue({
    data: {
      success: true,
      data: { options: {} },
    },
  })
  vi.spyOn(api, 'get').mockImplementation(async (url) => {
    if (url === '/api/group/') {
      return {
        data: { success: true, data: ['default', 'cc-only', 'cc-pro', 'auto'] },
      }
    }
    if (url === '/api/option/') return { data: optionsResponse() }
    if (url === '/api/option/request_policy') {
      return {
        data: {
          success: true,
          data: {
            options: Object.fromEntries(
              Object.entries(settings).map(([key, value]) => [
                key,
                String(value),
              ])
            ),
          },
        },
      }
    }
    return {
      data: {
        success: true,
        data: {
          enabled: false,
          total: 0,
          unknown: 0,
          by_rule_name: {},
          cache_capacity: 100000,
          cache_algo: 'lru',
        },
      },
    }
  })
  vi.spyOn(api, 'put').mockResolvedValue({ data: { success: true } })
})

afterEach(() => {
  cleanup()
  queryClient.clear()
})

describe('request policy settings', () => {
  it('the Claude Code gate starts disabled and saves selected groups and both modes together', async () => {
    await renderPolicies('/system-settings/request-policies/claude-code')
    const gateMode = await screen.findByRole('combobox', { name: 'Gate mode' })
    const bodyMode = screen.getByRole('combobox', {
      name: 'Body fingerprint checks',
    })
    expect(gateMode).toHaveValue('off')
    expect(bodyMode).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Save Changes' })).toBeDisabled()

    await userEvent.selectOptions(gateMode, 'enforce')
    expect(bodyMode).toBeEnabled()
    await userEvent.selectOptions(bodyMode, 'shadow')
    const groupInput = await screen.findByRole('combobox', {
      name: 'Restricted groups',
    })
    await waitFor(() => expect(groupInput).toBeEnabled())
    await userEvent.click(groupInput)
    expect(
      screen.queryByRole('option', { name: 'auto' })
    ).not.toBeInTheDocument()
    await userEvent.click(
      await screen.findByRole('option', { name: 'cc-only' })
    )
    await userEvent.click(groupInput)
    await userEvent.click(await screen.findByRole('option', { name: 'cc-pro' }))
    await userEvent.keyboard('{Escape}')
    await userEvent.click(screen.getByRole('button', { name: 'Save Changes' }))
    await waitFor(() =>
      expect(api.patch).toHaveBeenCalledWith('/api/option/request_policy', {
        options: {
          'claude_code_gate.mode': 'enforce',
          'claude_code_gate.groups': '["cc-only","cc-pro"]',
          'claude_code_gate.body_mode': 'shadow',
        },
      })
    )
  })

  it('observing the gate preserves the saved body mode and explains that requests remain unblocked', async () => {
    settings['claude_code_gate.mode'] = 'enforce'
    settings['claude_code_gate.body_mode'] = 'enforce'
    settings['claude_code_gate.groups'] = ['cc-only']
    await renderPolicies('/system-settings/request-policies/claude-code')
    const gateMode = await screen.findByRole('combobox', { name: 'Gate mode' })
    await userEvent.selectOptions(gateMode, 'shadow')
    expect(
      screen.getByRole('combobox', { name: 'Body fingerprint checks' })
    ).toHaveValue('enforce')
    expect(
      screen.getByText(
        'Observe only records mismatches without blocking requests or changing group selection.'
      )
    ).toBeVisible()
    await userEvent.click(screen.getByRole('button', { name: 'Save Changes' }))
    await waitFor(() =>
      expect(api.patch).toHaveBeenCalledWith('/api/option/request_policy', {
        options: {
          'claude_code_gate.mode': 'shadow',
          'claude_code_gate.groups': '["cc-only"]',
          'claude_code_gate.body_mode': 'enforce',
        },
      })
    )
  })

  it('a rejected gate save retains the edits for retry', async () => {
    vi.mocked(api.patch).mockResolvedValueOnce({
      data: { success: false, message: 'Save rejected' },
    })
    await renderPolicies('/system-settings/request-policies/claude-code')
    await userEvent.selectOptions(
      await screen.findByRole('combobox', { name: 'Gate mode' }),
      'enforce'
    )
    const save = screen.getByRole('button', { name: 'Save Changes' })
    await userEvent.click(save)
    await waitFor(() => expect(save).toBeEnabled())
    expect(screen.getByRole('combobox', { name: 'Gate mode' })).toHaveValue(
      'enforce'
    )
    await userEvent.click(save)
    await waitFor(() => expect(api.patch).toHaveBeenCalledTimes(2))
  })

  it('a failed group lookup shows a retry action and preserves saved selections', async () => {
    settings['claude_code_gate.groups'] = ['cc-only']
    vi.mocked(api.get).mockImplementation(async (url) => {
      if (url === '/api/option/') return { data: optionsResponse() }
      if (url === '/api/group/') {
        return { data: { success: false, message: 'Groups unavailable' } }
      }
      throw new Error(`Unexpected URL: ${url}`)
    })
    await renderPolicies('/system-settings/request-policies/claude-code')
    expect(await screen.findByText('Failed to load groups')).toBeVisible()
    expect(
      screen.getByRole('combobox', { name: 'Restricted groups' })
    ).toBeDisabled()
    expect(screen.getByText('cc-only')).toBeVisible()
    expect(screen.getByRole('button', { name: 'Retry' })).toBeEnabled()
    expect(api.patch).not.toHaveBeenCalled()
  })

  it.each([
    ['retry', 'Save Changes'],
    ['health', 'Save Changes'],
    ['filtering', 'Save sensitive words'],
    ['affinity', 'Save Changes'],
  ])(
    'opening %s and saving unchanged values does not write options',
    async (section, saveLabel) => {
      await renderPolicies(`/system-settings/request-policies/${section}`)
      const save = await screen.findByRole('button', { name: saveLabel })
      expect(api.put).not.toHaveBeenCalled()
      await userEvent.click(save)
      await waitFor(() => expect(save).toBeEnabled())
      expect(api.put).not.toHaveBeenCalled()
    }
  )

  it.each([
    [
      'health',
      'Channel test concurrency',
      '4',
      'monitor_setting.channel_test_concurrency',
      4,
    ],
  ])(
    'saving a changed %s field writes only that original option key',
    async (section, label, input, key, value) => {
      await renderPolicies(`/system-settings/request-policies/${section}`)
      fireEvent.change(await screen.findByRole('spinbutton', { name: label }), {
        target: { value: input },
      })
      await userEvent.click(
        screen.getByRole('button', { name: 'Save Changes' })
      )
      await waitFor(() =>
        expect(api.patch).toHaveBeenCalledExactlyOnceWith(
          '/api/option/request_policy',
          { options: { [key]: String(value) } }
        )
      )
    }
  )

  it('turning filtering off preserves the prompt switch and keyword list', async () => {
    await renderPolicies('/system-settings/request-policies/filtering')
    await userEvent.click(
      await screen.findByRole('switch', { name: 'Enable filtering' })
    )
    const prompt = screen.getByRole('switch', { name: 'Inspect user prompts' })
    expect(prompt).toHaveAttribute('aria-disabled', 'true')
    expect(prompt).toBeChecked()
    expect(
      screen.getByRole('textbox', { name: 'Blocked keywords' })
    ).toHaveValue('blocked')
    await userEvent.click(
      screen.getByRole('button', { name: 'Save sensitive words' })
    )
    await waitFor(() =>
      expect(api.patch).toHaveBeenCalledExactlyOnceWith(
        '/api/option/request_policy',
        { options: { CheckSensitiveEnabled: 'false' } }
      )
    )
  })

  it('refreshing another policy keeps unsaved filter text', async () => {
    await renderPolicies('/system-settings/request-policies/filtering')
    const keywords = await screen.findByRole('textbox', {
      name: 'Blocked keywords',
    })
    fireEvent.change(keywords, { target: { value: 'unsaved' } })
    settings.RetryTimes = 5
    await act(async () => {
      await queryClient.invalidateQueries({ queryKey: ['system-options'] })
    })
    expect(keywords).toHaveValue('unsaved')
  })

  it('invalid retry status ranges show validation and do not write options', async () => {
    await renderPolicies('/system-settings/request-policies/retry')
    const codes = await screen.findByRole('textbox', {
      name: 'Auto-retry status codes',
    })
    expect(codes).toBeVisible()
    expect(
      screen.queryByRole('button', { name: 'Advanced settings' })
    ).not.toBeInTheDocument()
    fireEvent.change(codes, { target: { value: '599-500' } })
    await userEvent.click(screen.getByRole('button', { name: 'Save Changes' }))
    expect(codes).toHaveAttribute('aria-invalid', 'true')
    expect(api.put).not.toHaveBeenCalled()
  })

  it('a rejected save keeps the edited retry value and allows another save', async () => {
    vi.mocked(api.patch).mockResolvedValue({
      data: { success: false, message: 'Save rejected' },
    })
    await renderPolicies('/system-settings/request-policies/retry')
    const retries = await screen.findByRole('spinbutton', {
      name: 'Maximum retries',
    })
    fireEvent.change(retries, { target: { value: '4' } })
    await userEvent.click(screen.getByRole('button', { name: 'Save Changes' }))
    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Save Changes' })).toBeEnabled()
    )
    expect(retries).toHaveValue(4)
    await userEvent.click(screen.getByRole('button', { name: 'Save Changes' }))
    await waitFor(() => expect(api.patch).toHaveBeenCalledTimes(2))
  })

  it('failed initial loading shows a retry action instead of editable fallback values', async () => {
    vi.mocked(api.get).mockRejectedValueOnce(new Error('Unavailable'))
    await renderPolicies('/system-settings/request-policies/retry')
    expect(await screen.findByText('Failed to load settings')).toBeVisible()
    expect(screen.queryByRole('spinbutton')).not.toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Retry' }))
    expect(
      await screen.findByRole('spinbutton', {
        name: 'Maximum retries',
      })
    ).toHaveValue(2)
    expect(api.put).not.toHaveBeenCalled()
  })

  it('turning scheduled checks off explains recovery without changing the recovery setting', async () => {
    await renderPolicies('/system-settings/request-policies/health')
    await userEvent.click(
      await screen.findByRole('switch', { name: 'Scheduled channel tests' })
    )
    expect(
      screen.getByText(
        'Scheduled recovery is off. Bulk channel tests can still re-enable automatically disabled channels.'
      )
    ).toBeVisible()
    expect(
      screen.getByRole('switch', { name: 'Re-enable on success' })
    ).toBeChecked()
    expect(api.put).not.toHaveBeenCalled()
  })

  it('the affinity cache section opens with the keyboard and keeps the existing values', async () => {
    await renderPolicies('/system-settings/request-policies/affinity')
    const toggle = await screen.findByRole('button', {
      name: 'Affinity cache settings',
    })
    expect(toggle).toHaveAttribute('aria-expanded', 'false')
    toggle.focus()
    await userEvent.keyboard('{Enter}')
    expect(toggle).toHaveAttribute('aria-expanded', 'true')
    expect(
      screen.getByRole('spinbutton', { name: 'Maximum cached sessions' })
    ).toHaveValue(100000)
    expect(
      screen.getByRole('spinbutton', {
        name: 'Default session lifetime (seconds)',
      })
    ).toHaveValue(3600)
    expect(api.put).not.toHaveBeenCalled()
  })

  it.each([
    ['/system-settings/models/channel-affinity', 'routing'],
    ['/system-settings/models/routing-reliability', 'routing'],
    ['/system-settings/security/sensitive-words', 'filtering'],
    ['/system-settings/operations/monitoring', 'health'],
    ['/system-settings/request-policies/', 'routing'],
    ['/system-settings/request-policies/unknown', 'routing'],
  ])('%s opens the corresponding policy page', async (path, section) => {
    const router = await renderPolicies(path)
    await waitFor(() =>
      expect(router.state.location.pathname).toBe(
        `/system-settings/request-policies/${section}`
      )
    )
    expect(api.put).not.toHaveBeenCalled()
  })
})
