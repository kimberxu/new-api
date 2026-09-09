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
import { resolveLocalizedText } from '@/lib/localized-text'

export type BillingUsageUnit = 'second' | 'count' | 'token' | 'credit'

export type BillingUsageFieldSchema = {
  type?: 'number' | 'boolean'
  unit?: BillingUsageUnit
  enum?: string[]
  enumLabels?: Record<string, string | Record<string, string>>
  description?: string | Record<string, string>
}

export type BillingUsageSchema = Record<string, BillingUsageFieldSchema>

export function taskEnumLabel(
  definition: BillingUsageFieldSchema | undefined,
  value: string,
  language: string
): string {
  const labels = definition?.enumLabels?.[value]
  if (typeof labels === 'string') return labels || value
  const localized =
    typeof labels === 'object' && labels
      ? { ...labels, en: labels.en?.trim() || value }
      : undefined
  return resolveLocalizedText(localized, language) || value
}
