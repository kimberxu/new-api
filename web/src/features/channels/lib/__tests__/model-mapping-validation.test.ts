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
import { describe, test } from 'node:test'

import { findExposedTargetModels } from '../model-mapping-validation'

describe('findExposedTargetModels', () => {
  test('reports a redirect target that is listed in Models', () => {
    const mapping = JSON.stringify({ 'ds-v4': 'deepseek-v4-flash-0731' })

    const exposed = findExposedTargetModels(mapping, [
      'ds-v4',
      'deepseek-v4-flash-0731',
    ])

    assert.deepEqual(exposed, ['deepseek-v4-flash-0731'])
  })

  test('does not report a model that is both source key and target', () => {
    const mapping = JSON.stringify({ 'deepseek-v4-flash': 'deepseek-v4-flash' })

    const exposed = findExposedTargetModels(mapping, ['deepseek-v4-flash'])

    assert.deepEqual(exposed, [])
  })

  test('weighted array: keeps source key, reports other listed targets', () => {
    const mapping = JSON.stringify({
      'deepseek-v4-flash': [
        { model: 'deepseek-v4-flash', weight: 1 },
        { model: 'deepseek-v4-flash-0731', weight: 1 },
      ],
    })

    const exposed = findExposedTargetModels(mapping, [
      'deepseek-v4-flash',
      'deepseek-v4-flash-0731',
    ])

    assert.deepEqual(exposed, ['deepseek-v4-flash-0731'])
  })

  test('reports multiple exposed targets in mapping order', () => {
    const mapping = JSON.stringify({
      'ds-v4': 'deepseek-v4-flash-0731',
      'ds-x': 'deepseek-reasoner',
    })

    const exposed = findExposedTargetModels(mapping, [
      'ds-v4',
      'ds-x',
      'deepseek-v4-flash-0731',
      'deepseek-reasoner',
    ])

    assert.deepEqual(exposed, ['deepseek-v4-flash-0731', 'deepseek-reasoner'])
  })

  test('ignores targets that are not in Models', () => {
    const mapping = JSON.stringify({ 'ds-v4': 'deepseek-v4-flash-0731' })

    const exposed = findExposedTargetModels(mapping, ['ds-v4'])

    assert.deepEqual(exposed, [])
  })

  test('trims whitespace in source keys and target names', () => {
    const mapping = JSON.stringify({ ' ds-v4 ': ' deepseek-v4-flash-0731 ' })

    const exposed = findExposedTargetModels(mapping, [
      'ds-v4',
      'deepseek-v4-flash-0731',
    ])

    assert.deepEqual(exposed, ['deepseek-v4-flash-0731'])
  })

  test('trims whitespace so a key-equal target is excluded', () => {
    const mapping = JSON.stringify({
      ' deepseek-v4-flash ': 'deepseek-v4-flash',
    })

    const exposed = findExposedTargetModels(mapping, ['deepseek-v4-flash'])

    assert.deepEqual(exposed, [])
  })

  test('returns empty for empty mapping', () => {
    assert.deepEqual(findExposedTargetModels('', ['deepseek-v4-flash']), [])
    assert.deepEqual(findExposedTargetModels('{}', ['deepseek-v4-flash']), [])
  })

  test('returns empty for invalid JSON', () => {
    assert.deepEqual(
      findExposedTargetModels('not-json', ['deepseek-v4-flash']),
      []
    )
  })

  test('returns empty when mapping is not an object', () => {
    assert.deepEqual(
      findExposedTargetModels('[1, 2]', ['deepseek-v4-flash']),
      []
    )
  })
})
