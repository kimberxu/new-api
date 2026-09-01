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
import { render, screen, within } from '@testing-library/react'
import { describe, expect, test } from 'vitest'

import { TooltipProvider } from '@/components/ui/tooltip'
import type { DemotedChannelInfo } from '@/features/channels/types'
import { GroupCard } from '../group-card'

// A manual group whose routable name differs from the member's real upstream
// model. Demotions are recorded against the routable name (what the client
// requested), so the badge must match on group.name, not item.model.
const ROUTABLE_MODEL = 'ox'
const UPSTREAM_MODEL = 'gpt-4o'

function CardHarness(props: {
  demoted: Map<number, DemotedChannelInfo[]>
}) {
  return (
    <TooltipProvider>
      <GroupCard
        group={{
          id: 1,
          name: ROUTABLE_MODEL,
          source: 'manual',
          enabled: true,
          param_override: '',
          created_at: 0,
          members: [
            {
              id: 1,
              group_id: 1,
              channel_id: 7,
              channel_name: 'Test Upstream',
              channel_type: 1,
              model: UPSTREAM_MODEL,
              priority: null,
              weight: null,
              enabled: true,
              channel_priority: 5,
              channel_weight: 1,
            },
          ],
        }}
        expanded
        updatingMember={false}
        demoted={props.demoted}
        onToggleExpanded={() => undefined}
        onToggleEnabled={() => undefined}
        onEditParams={() => undefined}
        onRename={() => undefined}
        onDelete={() => undefined}
        onAddMember={() => undefined}
        onDeleteReference={() => undefined}
        onDeleteMember={() => undefined}
        onUpdateMember={async () => ({ success: true })}
        onToggleMember={() => undefined}
        onTestMember={async () => ({ success: true })}
      />
    </TooltipProvider>
  )
}

function modelCellOf(container: HTMLElement): HTMLElement {
  const cell = within(container).getByText(UPSTREAM_MODEL).closest('td')
  if (!cell) throw new Error('member model cell not found')
  return cell as HTMLElement
}

describe('GroupCard demoted member badge', () => {
  test('shows the demoted count badge on the group header when a member is demoted', () => {
    const demoted = new Map<number, DemotedChannelInfo[]>([
      [7, [{ model: ROUTABLE_MODEL, remaining_seconds: 300, sources: ['tps'] }]],
    ])
    render(<CardHarness demoted={demoted} />)

    expect(screen.getByText('1 demoted')).toBeInTheDocument()
  })

  test('hides the demoted count badge when no member is demoted', () => {
    render(<CardHarness demoted={new Map()} />)

    expect(screen.queryByText(/demoted/i)).toBeNull()
  })

  test('marks the member row when the routable model is demoted on its channel', () => {
    const demoted = new Map<number, DemotedChannelInfo[]>([
      [7, [{ model: ROUTABLE_MODEL, remaining_seconds: 300, sources: ['ttft'] }]],
    ])
    const { container } = render(<CardHarness demoted={demoted} />)

    expect(
      within(modelCellOf(container)).getByText(/Demoted/)
    ).toBeInTheDocument()
  })

  test('does not mark a member whose routable channel+model pair is not demoted', () => {
    const demoted = new Map<number, DemotedChannelInfo[]>([
      [
        7,
        [
          {
            model: 'claude-3-5-sonnet',
            remaining_seconds: 60,
            sources: ['tps'],
          },
        ],
      ],
    ])
    const { container } = render(<CardHarness demoted={demoted} />)

    expect(within(modelCellOf(container)).queryByText(/Demoted/)).toBeNull()
  })
})
