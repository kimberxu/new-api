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
import { describe, expect, test } from 'vitest'

import type { InflightResponse } from '../inflight-api'

describe('inflight API response parsing', () => {
  test('parses a well-formed inflight response with items', () => {
    const raw: InflightResponse = {
      page: 1,
      page_size: 20,
      total: 2,
      items: [
        {
          request_id: 'req-001',
          channel_id: 42,
          model_name: 'gpt-4',
          start_time: 1700000000,
          request_path: '/v1/chat/completions',
        },
        {
          request_id: 'req-002',
          channel_id: 7,
          model_name: 'claude-3',
          start_time: 1700000010,
          request_path: '/v1/messages',
        },
      ],
    }

    expect(raw.items).toHaveLength(2)
    expect(raw.items[0].request_id).toBe('req-001')
    expect(raw.items[0].channel_id).toBe(42)
    expect(raw.items[0].model_name).toBe('gpt-4')
    expect(raw.items[0].start_time).toBe(1700000000)
    expect(raw.items[0].request_path).toBe('/v1/chat/completions')
    expect(raw.total).toBe(2)
  })

  test('handles an empty inflight response', () => {
    const raw: InflightResponse = {
      page: 1,
      page_size: 20,
      total: 0,
      items: [],
    }

    expect(raw.items).toHaveLength(0)
    expect(raw.total).toBe(0)
  })

  test('handles pagination metadata correctly', () => {
    const raw: InflightResponse = {
      page: 3,
      page_size: 50,
      total: 125,
      items: [],
    }

    expect(raw.page).toBe(3)
    expect(raw.page_size).toBe(50)
    expect(raw.total).toBe(125)
  })
})
