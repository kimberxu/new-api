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
  createRootRoute,
  createRoute,
  createRouter,
  createMemoryHistory,
  RouterProvider,
} from '@tanstack/react-router'
import {
  flexRender,
  getCoreRowModel,
  useReactTable,
} from '@tanstack/react-table'
import {
  render,
  screen,
  within,
  cleanup,
  waitFor,
  act,
} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import i18n from 'i18next'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'

import fr from '@/i18n/locales/fr.json'
import zhCN from '@/i18n/locales/zh.json'
import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'
import {
  DEFAULT_CURRENCY_CONFIG,
  useSystemConfigStore,
} from '@/stores/system-config-store'

import { ModelsDialogs } from '../components/models-dialogs'
import { ModelsProvider } from '../components/models-provider'
import { ModelsTable } from '../components/models-table'
import type { Model } from '../types'

const metadata: Model = {
  id: 7,
  model_name: 'catalog-only',
  square_state: 'unavailable',
  has_metadata: true,
  configured_channel_count: 0,
  name_rule: 0,
  status: 1,
  sync_official: 1,
  created_time: 1,
  updated_time: 1,
}
const channel: Model = {
  ...metadata,
  id: 0,
  model_name: 'channel-only',
  has_metadata: false,
  configured_channel_count: 1,
  status: 0,
  sync_official: 0,
}
const clients: QueryClient[] = []

function Page() {
  return (
    <ModelsProvider>
      <ModelsTable />
      <ModelsDialogs />
    </ModelsProvider>
  )
}

async function renderList(
  items: Model[] = [
    metadata,
    channel,
    { ...channel, model_name: 'other-channel' },
  ],
  options: {
    initialUrl?: string
    total?: number
  } = {}
) {
  useAuthStore.getState().auth.setUser({ id: 1, username: 'admin', role: 100 })
  const get = vi.spyOn(api, 'get').mockImplementation(async (url) => {
    if (url === '/api/models/' || url === '/api/models/search') {
      return {
        data: {
          success: true,
          data: { items, total: options.total ?? items.length },
        },
      }
    }
    if (url === '/api/models/7') {
      return { data: { success: true, data: metadata } }
    }
    return { data: { success: true, data: { items: [] } } }
  })
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  clients.push(client)
  const root = createRootRoute()
  const authenticated = createRoute({
    getParentRoute: () => root,
    id: '_authenticated',
  })
  const models = createRoute({
    getParentRoute: () => authenticated,
    path: 'models/$section',
    component: Page,
  })
  const router = createRouter({
    routeTree: root.addChildren([authenticated.addChildren([models])]),
    history: createMemoryHistory({
      initialEntries: [options.initialUrl ?? '/models/metadata'],
    }),
  })
  await router.load()
  const result = render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  )
  if (items.length) {
    await screen.findByRole('button', { name: items[0].model_name })
  } else await screen.findByText('No Models Found')
  if (false) {
    await waitFor(() => expect(client.isFetching()).toBe(0))
  }
  return { ...result, get, router }
}

beforeEach(() => {
  useSystemConfigStore.getState().setConfig({
    currency: { ...DEFAULT_CURRENCY_CONFIG, quotaDisplayType: 'USD' },
  })
  vi.spyOn(window, 'scrollTo').mockImplementation(() => {})
  i18n.addResourceBundle('fr', 'translation', fr.translation, true, true)
  i18n.addResourceBundle('zhCN', 'translation', zhCN.translation, true, true)
})

afterEach(async () => {
  cleanup()
  clients.splice(0).forEach((client) => client.clear())
  useAuthStore.getState().auth.reset()
  useSystemConfigStore
    .getState()
    .setConfig({ currency: { ...DEFAULT_CURRENCY_CONFIG } })
  await i18n.changeLanguage('en')
})
