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
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, it, vi } from 'vitest'

import { api } from '@/lib/api'

import { TwoFACard } from '../components/two-fa-card'

it('shows a retry when the 2FA status query fails instead of offering enrollment', async () => {
  const get = vi
    .spyOn(api, 'get')
    .mockRejectedValue(new Error('Status unavailable'))
  const user = userEvent.setup()
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  render(
    <QueryClientProvider client={client}>
      <TwoFACard loading={false} />
    </QueryClientProvider>
  )
  expect(await screen.findByRole('alert')).toHaveTextContent(
    'Status unavailable'
  )
  expect(
    screen.queryByRole('button', { name: 'Enable' })
  ).not.toBeInTheDocument()
  get.mockResolvedValue({
    data: {
      success: true,
      data: { enabled: false, locked: false, backup_codes_remaining: 0 },
    },
  })
  await user.click(screen.getByRole('button', { name: 'Retry' }))
  expect(await screen.findByRole('button', { name: 'Enable' })).toBeEnabled()
})
