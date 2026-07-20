export function formatRequestDebugBody(body: string): string {
  const trimmed = body.trim()
  if (!trimmed) return body
  if (trimmed[0] !== '{' && trimmed[0] !== '[') return body

  try {
    const parsed = JSON.parse(body)
    return JSON.stringify(parsed, null, 2)
  } catch {
    return body
  }
}
