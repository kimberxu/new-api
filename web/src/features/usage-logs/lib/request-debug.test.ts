import { describe, expect, test } from 'vitest'

import { formatRequestDebugBody } from './request-debug'

describe('request debug body formatting', () => {
  test('pretty prints json request bodies', () => {
    const body =
      '{"model":"gpt-test","messages":[{"role":"user","content":"hello"}]}'

    expect(formatRequestDebugBody(body)).toBe(
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

    expect(formatRequestDebugBody(body)).toBe(body)
  })
})