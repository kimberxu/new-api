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

import {
  parseExcludeChannelIds,
  serializeExcludeChannelIds,
} from '../utils'

describe('parseExcludeChannelIds', () => {
  test('parses JSON array strings into comma-separated text', () => {
    expect(parseExcludeChannelIds('[1, 2, 3]')).toBe('1, 2, 3')
  })

  test('treats null, empty array and empty string as no exclusions', () => {
    expect(parseExcludeChannelIds('null')).toBe('')
    expect(parseExcludeChannelIds('[]')).toBe('')
    expect(parseExcludeChannelIds('')).toBe('')
    expect(parseExcludeChannelIds(undefined)).toBe('')
  })

  test('falls back to raw text when not valid JSON', () => {
    expect(parseExcludeChannelIds('1, 2')).toBe('1, 2')
  })
})

describe('serializeExcludeChannelIds', () => {
  test('serializes comma-separated text into a JSON array string', () => {
    expect(serializeExcludeChannelIds('1, 2, 3')).toBe('[1,2,3]')
  })

  test('drops empty parts and invalid ids', () => {
    expect(serializeExcludeChannelIds('1, abc, -2, 3.5, 4')).toBe('[1,4]')
    expect(serializeExcludeChannelIds('7,,8')).toBe('[7,8]')
  })

  test('empty input serializes to an empty array', () => {
    expect(serializeExcludeChannelIds('')).toBe('[]')
    expect(serializeExcludeChannelIds('  ,  ')).toBe('[]')
  })
})
