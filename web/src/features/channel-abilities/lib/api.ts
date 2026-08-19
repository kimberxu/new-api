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
import { api, type ApiRequestConfig } from '@/lib/api'

// ============================================================================
// Types
// ============================================================================

export type ChannelAbilityStatus =
  | 'all'
  | 'enabled'
  | 'manual_disabled'
  | 'auto_disabled'

/**
 * One row of the channel×model route table. `id` is the row position within
 * the filtered result (the ability has a composite key); rows are keyed by
 * `channel_id-model-group`.
 */
export interface ChannelAbilityItem {
  id: number
  channel_id: number
  channel_name: string
  channel_type: number
  group: string
  model: string
  priority: number | null
  weight: number
  ability_enabled: boolean
  disabled: boolean
  disabled_source: string
  disabled_reason: string
}

export interface ChannelAbilitiesResponse {
  items: ChannelAbilityItem[]
  total: number
  page: number
  page_size: number
}

export interface GetChannelAbilitiesParams {
  p?: number
  page_size?: number
  channel_id?: number
  model?: string
  group?: string
  status?: ChannelAbilityStatus
}

export interface ChannelAbilityMutationResponse {
  success: boolean
  message?: string
  data?: { disabled?: number; enabled?: number }
}

export interface ChannelAbilityTestResponse {
  success: boolean
  message?: string
  error_code?: string
  time?: number
  data?: { response_time?: number; error?: string }
}

// ============================================================================
// API
// ============================================================================

// Row actions inspect `success` themselves and render their own toasts, so
// bypass the global business-error/error handlers like the channels feature.
const channelAbilityActionConfig = (
  config: ApiRequestConfig = {}
): ApiRequestConfig => ({
  ...config,
  skipBusinessError: true,
  skipErrorHandler: true,
})

/**
 * Get the paginated channel×model route table.
 */
export async function getChannelAbilities(
  params: GetChannelAbilitiesParams = {}
): Promise<ChannelAbilitiesResponse> {
  const res = await api.get('/api/channel/abilities', { params })
  const data = res.data?.data
  if (!data) {
    return {
      items: [],
      total: 0,
      page: params.p ?? 1,
      page_size: params.page_size ?? 20,
    }
  }
  return data as ChannelAbilitiesResponse
}

/**
 * Manually disable specific models on a channel.
 */
export async function disableChannelAbilities(
  channelId: number,
  models: string[],
  reason?: string
): Promise<ChannelAbilityMutationResponse> {
  const res = await api.post(
    '/api/channel/abilities/disable',
    { channel_id: channelId, models, reason },
    channelAbilityActionConfig()
  )
  return res.data
}

/**
 * Re-enable specific models on a channel (clears manual and auto disables).
 */
export async function enableChannelAbilities(
  channelId: number,
  models: string[]
): Promise<ChannelAbilityMutationResponse> {
  const res = await api.post(
    '/api/channel/abilities/enable',
    { channel_id: channelId, models },
    channelAbilityActionConfig()
  )
  return res.data
}

/**
 * Test a specific model on a channel. A successful test also clears the
 * model-level disable for that channel automatically.
 */
export async function testChannelModel(
  channelId: number,
  model: string
): Promise<ChannelAbilityTestResponse> {
  const res = await api.get(
    `/api/channel/test/${channelId}`,
    channelAbilityActionConfig({ params: { model } })
  )
  return res.data
}
