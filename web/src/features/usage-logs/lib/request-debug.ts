/**
 * Format the request debug body for display in the admin log details dialog.
 * Parsed JSON is pretty-printed; raw strings are returned as-is.
 */
export function formatRequestDebugBody(body: string): string {
  if (!body) return body
  try {
    const parsed = JSON.parse(body)
    return JSON.stringify(parsed, null, 2)
  } catch {
    return body
  }
}