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
import type { ModelGroupItem } from './api'

export type PrioritySortDir = 'asc' | 'desc'

/** Effective priority for routing: member override → channel value → 0. */
export const effectivePriority = (m: ModelGroupItem): number =>
  m.priority ?? m.channel_priority ?? 0

/**
 * Return a new array sorted by effective priority.
 * Higher values = higher preference (matches backend routing order).
 *
 * - `'desc'`: highest priority first (default route preference)
 * - `'asc'`: lowest priority first
 */
export function sortByPriority(
  items: ModelGroupItem[],
  dir: PrioritySortDir,
): ModelGroupItem[] {
  return [...items].sort((a, b) => {
    const ea = effectivePriority(a)
    const eb = effectivePriority(b)
    return dir === 'desc' ? eb - ea : ea - eb
  })
}
