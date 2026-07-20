import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { formatRequestDebugBody } from './request-debug'

describe('request debug body formatting', () => {
  test('pretty prints json request bodies', () => {
    const body =
      '{"model":"gpt-test","messages":[{"role":"user","content":"hello"}]}'

    assert.equal(
      formatRequestDebugBody(body),
      [
        '{',
        '  "model": "gpt-test",',
        '  "messages": [',
        '    {',
        '      "role": "user",',
        '      "content": "hello"',
        '    }',
        '  ]',
        '}',
      ].join('\n')
    )
  })

  test('keeps non-json request bodies unchanged', () => {
    const body = 'plain text body'

    assert.equal(formatRequestDebugBody(body), body)
  })
})
