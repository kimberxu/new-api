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

import { sortByPriority } from '../priority-sort'
import type { ModelGroupItem } from '../api'

const item = (overrides: Partial<ModelGroupItem>): ModelGroupItem => ({
  id: 1,
  group_id: 1,
  channel_id: 1,
  channel_name: 'ch',
  channel_type: 1,
  model: 'm',
  priority: null,
  weight: null,
  enabled: true,
  channel_priority: 0,
  channel_weight: 1,
  ...overrides,
})

describe('sortByPriority', () => {
  const low = item({ id: 1, model: 'low', priority: 1 })
  const mid = item({ id: 2, model: 'mid', priority: 5 })
  const high = item({ id: 3, model: 'high', priority: 9 })

  test('desc sorts highest priority first', () => {
    expect(sortByPriority([low, high, mid], 'desc').map((i) => i.id)).toEqual(
      [3, 2, 1]
    )
  })

  test('asc sorts lowest priority first', () => {
    expect(sortByPriority([mid, high, low], 'asc').map((i) => i.id)).toEqual(
      [1, 2, 3]
    )
  })

  test('member override wins over channel priority', () => {
    const override = item({ id: 1, priority: 8, channel_priority: 2 })
    const channelOnly = item({ id: 2, priority: null, channel_priority: 5 })
    expect(
      sortByPriority([channelOnly, override], 'desc').map((i) => i.id)
    ).toEqual([1, 2])
  })

  test('null override falls back to channel priority', () => {
    const chHigh = item({ id: 1, priority: null, channel_priority: 12 })
    expect(sortByPriority([high, chHigh], 'desc').map((i) => i.id)).toEqual([
      chHigh.id,
      high.id,
    ])
  })

  test('null override and channel fallback sort as 0 (lowest)', () => {
    const none = item({ id: 1, priority: null, channel_priority: 0 })
    const lowPrio = item({ id: 2, priority: 1, channel_priority: 0 })
    expect(sortByPriority([none, lowPrio], 'desc').map((i) => i.id)).toEqual([
      lowPrio.id,
      none.id,
    ])
  })

  test('does not mutate the input array', () => {
    const input = [low, high, mid]
    const result = sortByPriority(input, 'desc')
    expect(result).not.toBe(input)
    expect(input.map((i) => i.id)).toEqual([1, 3, 2])
  })
})