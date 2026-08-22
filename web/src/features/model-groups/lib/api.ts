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
import { api } from '@/lib/api'

export interface ModelGroupItem {
  id: number
  group_id: number
  channel_id: number
  channel_name: string
  channel_type: number
  model: string
  priority: number | null
  weight: number | null
  enabled: boolean
  channel_priority: number
  channel_weight: number
  // live channel status: 1 enabled, 2 manually disabled, 3 auto disabled.
  // Members on a non-enabled channel are excluded from routing even though
  // item.enabled is true.
  channel_status?: number
  // source_group is set when this member came from a referenced group
  // (empty = direct member of this group).
  source_group?: string
  disabled?: {
    source: string
    reason: string
    banned_until: number
  } | null
}

export interface ModelGroupReference {
  id: number
  group_id: number
  ref_group_id: number
  ref_group_name: string
  created_at: number
}

export interface ModelGroup {
  id: number
  name: string
  source: string
  enabled: boolean
  param_override: string
  created_at: number
  members?: ModelGroupItem[]
  member_count?: number
  references?: ModelGroupReference[]
}

export interface ListModelGroupsResponse {
  items: ModelGroup[]
  total: number
}

export async function listModelGroups(withItems = false): Promise<ListModelGroupsResponse> {
  const res = await api.get('/api/model-groups/', { params: { with_items: withItems ? '1' : '0' } })
  return res.data?.data ?? { items: [], total: 0 }
}

export async function getModelGroup(id: number): Promise<ModelGroup> {
  const res = await api.get(`/api/model-groups/${id}`)
  return res.data?.data
}

export async function createModelGroup(name: string): Promise<{ success: boolean; message?: string; data?: ModelGroup }> {
  const res = await api.post('/api/model-groups/', { name }, { skipBusinessError: true, skipErrorHandler: true })
  return res.data
}

export async function setModelGroupEnabled(id: number, enabled: boolean): Promise<{ success: boolean; message?: string }> {
  const res = await api.patch(`/api/model-groups/${id}`, { enabled })
  return res.data
}

export async function deleteModelGroup(id: number): Promise<{ success: boolean; message?: string }> {
  const res = await api.delete(`/api/model-groups/${id}`, { skipBusinessError: true, skipErrorHandler: true })
  return res.data
}

export async function addGroupItem(groupId: number, channelId: number, model: string, priority?: number | null, weight?: number | null): Promise<{ success: boolean; message?: string }> {
  const res = await api.post(`/api/model-groups/${groupId}/items`, { channel_id: channelId, model, priority, weight }, { skipBusinessError: true, skipErrorHandler: true })
  return res.data
}

export async function updateGroupItem(itemId: number, data: { enabled?: boolean; priority?: number | null; weight?: number | null }): Promise<{ success: boolean; message?: string }> {
  const res = await api.patch(`/api/model-groups/items/${itemId}`, data, { skipBusinessError: true, skipErrorHandler: true })
  return res.data
}

export async function deleteGroupItem(itemId: number): Promise<{ success: boolean; message?: string }> {
  const res = await api.delete(`/api/model-groups/items/${itemId}`, { skipBusinessError: true, skipErrorHandler: true })
  return res.data
}

export async function setModelGroupParamOverride(id: number, paramOverride: string): Promise<{ success: boolean; message?: string }> {
  const res = await api.put(`/api/model-groups/${id}/param-override`, { param_override: paramOverride })
  return res.data
}

export async function addGroupReference(groupId: number, refGroupId: number): Promise<{ success: boolean; message?: string }> {
  const res = await api.post(`/api/model-groups/${groupId}/references`, { ref_group_id: refGroupId }, { skipBusinessError: true, skipErrorHandler: true })
  return res.data
}

export async function deleteGroupReference(groupId: number, refGroupId: number): Promise<{ success: boolean; message?: string }> {
  const res = await api.delete(`/api/model-groups/${groupId}/references/${refGroupId}`, { skipBusinessError: true, skipErrorHandler: true })
  return res.data
}

export async function rebuildModelGroups(): Promise<{ success: boolean; message?: string }> {
  const res = await api.post('/api/model-groups/rebuild', {}, { skipBusinessError: true, skipErrorHandler: true })
  return res.data
}