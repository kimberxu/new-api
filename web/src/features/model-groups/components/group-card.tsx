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
import {
  ArrowDown,
  ArrowUp,
  ArrowUpDown,
  ChevronDown,
  Layers,
  Link2,
  Pencil,
  Plus,
  Save,
  SlidersHorizontal,
  Trash2,
  X,
  Zap,
} from 'lucide-react'
import { useMemo, useState } from 'react'

import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import { IconBadge } from '@/components/ui/icon-badge'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'

import { formatSeconds } from '@/features/channels/lib'
import type { DemotedChannelInfo } from '@/features/channels/types'
import { formatTimestampToDate } from '@/lib/format'
import { cn } from '@/lib/utils'

import type {
  ModelGroup,
  ModelGroupItem,
  ModelGroupReference,
} from '../lib/api'
import { sortByPriority } from '../lib/priority-sort'

export interface MemberEditState {
  priorityInput: string
  weightInput: string
}

export interface MemberActionResult {
  success: boolean
  message?: string
}

interface GroupCardProps {
  group: ModelGroup
  expanded: boolean
  updatingMember: boolean
  demoted: Map<number, DemotedChannelInfo[]>
  onToggleExpanded: (id: number) => void
  onToggleEnabled: (id: number, enabled: boolean) => void
  onEditParams: (group: ModelGroup) => void
  onRename: (group: ModelGroup) => void
  onDelete: (group: ModelGroup) => void
  onAddMember: (group: ModelGroup) => void
  onDeleteReference: (group: ModelGroup, ref: ModelGroupReference) => void
  onDeleteMember: (group: ModelGroup, item: ModelGroupItem) => void
  onUpdateMember: (
    item: ModelGroupItem,
    priority: number | null,
    weight: number | null
  ) => Promise<MemberActionResult>
  onToggleMember: (item: ModelGroupItem, enabled: boolean) => void
  onTestMember: (item: ModelGroupItem) => Promise<MemberActionResult>
}

export function GroupCard(props: GroupCardProps) {
  const { t } = useTranslation()
  const { group, demoted } = props
  const members = group.members ?? []
  const bannedCount = members.filter((m) => m.disabled).length
  // Demotion records are keyed by the routable model (the group name), not
  // the member's real upstream model: for a manual group "ox" containing the
  // upstream model "gpt-4o", requests arrive as model="ox" and RecordSlowStream
  // keys on "ox". Auto groups match because their name equals the model name.
  const demotedCount = members.filter((m) =>
    demoted.get(m.channel_id)?.some((d) => d.model === group.name)
  ).length
  const [prioritySort, setPrioritySort] = useState<'asc' | 'desc' | null>(
    null
  )
  const sortedMembers = useMemo(() => {
    if (!prioritySort) return members
    return sortByPriority(members, prioritySort)
  }, [members, prioritySort])

  const [edits, setEdits] = useState<Record<number, MemberEditState>>({})
  const [testingId, setTestingId] = useState<number | null>(null)

  const getEdit = (item: ModelGroupItem): MemberEditState => {
    const existing = edits[item.id]
    if (existing) return existing
    return {
      priorityInput:
        item.priority !== null && item.priority !== undefined
          ? String(item.priority)
          : '',
      weightInput:
        item.weight !== null && item.weight !== undefined
          ? String(item.weight)
          : '',
    }
  }

  const saveMember = async (item: ModelGroupItem) => {
    const edit = getEdit(item)
    try {
      const res = await props.onUpdateMember(
        item,
        edit.priorityInput === '' ? null : Number(edit.priorityInput),
        edit.weightInput === '' ? null : Number(edit.weightInput)
      )
      if (res.success) {
        setEdits((prev) => {
          const n = { ...prev }
          delete n[item.id]
          return n
        })
      }
    } catch {
      /* 页面 mutation onError 已 toast */
    }
  }

  const testMember = async (item: ModelGroupItem) => {
    setTestingId(item.id)
    try {
      const res = await props.onTestMember(item)
      if (res.success) {
        if (item.disabled) {
          toast.success(t('Test passed, member re-enabled'))
        } else {
          toast.success(t('Test passed'))
        }
      } else {
        toast.error(res.message || t('Test failed'))
      }
    } catch {
      /* 网络层错误已由全局拦截器 toast */
    } finally {
      setTestingId(null)
    }
  }

  return (
    <Collapsible
      open={props.expanded}
      onOpenChange={(o) => {
        if (o !== props.expanded) props.onToggleExpanded(group.id)
      }}
      className='border-border rounded-lg border'
    >
      <div className='flex flex-wrap items-center gap-2 p-3'>
        <CollapsibleTrigger
          render={
            <button
              type='button'
              className='flex min-w-0 flex-1 cursor-pointer items-center gap-2 rounded-md text-left'
            />
          }
        >
          <ChevronDown
            className={cn(
              'h-4 w-4 shrink-0 transition-transform',
              !props.expanded && '-rotate-90'
            )}
          />
          <IconBadge
            tone={group.source === 'auto' ? 'info' : 'primary'}
            size='sm'
          >
            <Layers className='h-3.5 w-3.5' />
          </IconBadge>
          {group.source === 'manual' ? (
            <span
              className='hover:text-foreground hover:underline cursor-pointer truncate font-medium underline-offset-4'
              onClick={(e) => {
                e.stopPropagation()
                props.onRename(group)
              }}
              title={t('Click to rename model group')}
            >
              {group.name}
              <Pencil className='text-muted-foreground ml-1 inline h-3 w-3' />
            </span>
          ) : (
            <span className='truncate font-medium'>{group.name}</span>
          )}
          <StatusBadge
            variant={group.source === 'auto' ? 'info' : 'success'}
            size='sm'
          >
            {group.source === 'auto' ? t('Auto') : t('Manual')}
          </StatusBadge>
          {bannedCount > 0 && (
            <StatusBadge variant='warning' size='sm' showDot>
              {t('{{count}} banned', { count: bannedCount })}
            </StatusBadge>
          )}
          {demotedCount > 0 && (
            <StatusBadge variant='warning' size='sm' showDot>
              {t('{{count}} demoted', { count: demotedCount })}
            </StatusBadge>
          )}
          <span className='text-muted-foreground ml-auto shrink-0 text-xs'>
            {t('{{count}} members', { count: members.length })}
          </span>
        </CollapsibleTrigger>
        <Switch
          checked={group.enabled}
          onCheckedChange={(checked) =>
            props.onToggleEnabled(group.id, checked)
          }
        />
        <Button
          variant={group.param_override ? 'secondary' : 'ghost'}
          size='icon-sm'
          onClick={() => props.onEditParams(group)}
          title={t('Parameter override')}
        >
          <SlidersHorizontal className='h-4 w-4' />
        </Button>
        <Button
          variant='ghost'
          size='icon-sm'
          onClick={() => props.onDelete(group)}
        >
          <Trash2 className='h-4 w-4' />
        </Button>
      </div>

      <CollapsibleContent className='CollapsibleContent border-border border-t'>
        <div className='p-3'>
          {(group.references ?? []).length > 0 && (
            <div className='mb-3 flex flex-wrap items-center gap-2'>
              <span className='text-muted-foreground text-xs'>
                {t('References')}:
              </span>
              {(group.references ?? []).map((ref) => (
                <div
                  key={ref.id}
                  className='bg-muted flex items-center gap-1 rounded-full px-2 py-0.5 text-xs'
                >
                  <Link2 className='h-3 w-3' />
                  <span className='text-foreground font-medium'>
                    {ref.ref_group_name}
                  </span>
                  <Button
                    variant='ghost'
                    size='icon-sm'
                    className='h-4 w-4'
                    onClick={() => props.onDeleteReference(group, ref)}
                    title={t('Remove reference')}
                  >
                    <X className='h-3 w-3' />
                  </Button>
                </div>
              ))}
              <span className='text-muted-foreground text-xs'>
                {t(
                  'Edits to referenced members also apply to their source group'
                )}
              </span>
            </div>
          )}
          {members.length === 0 ? (
            <div className='text-muted-foreground py-4 text-center text-sm'>
              {t('No members in this group yet')}
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('Model')}</TableHead>
                  <TableHead>{t('Channel')}</TableHead>
                  <TableHead className='w-36'>
                    <button
                      type='button'
                      className='hover:text-foreground flex cursor-pointer items-center gap-1 font-medium'
                      onClick={() =>
                        setPrioritySort((cur) =>
                          cur === null ? 'desc' : cur === 'desc' ? 'asc' : null
                        )
                      }
                      title={t('Sort by priority')}
                    >
                      {t('Priority (empty = inherit)')}
                      {prioritySort === 'desc' ? (
                        <ArrowDown className='h-3.5 w-3.5' />
                      ) : prioritySort === 'asc' ? (
                        <ArrowUp className='h-3.5 w-3.5' />
                      ) : (
                        <ArrowUpDown className='text-muted-foreground h-3.5 w-3.5' />
                      )}
                    </button>
                  </TableHead>
                  <TableHead className='w-32'>
                    {t('Weight (empty = inherit)')}
                  </TableHead>
                  <TableHead>{t('Enabled')}</TableHead>
                  <TableHead className='w-24'>{t('Actions')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {sortedMembers.map((item) => {
                  const edit = getEdit(item)
                  const dirty =
                    edit.priorityInput !==
                      (item.priority != null ? String(item.priority) : '') ||
                    edit.weightInput !==
                      (item.weight != null ? String(item.weight) : '')
                  let disabledLabel: string | undefined
                  let disabledVariant: 'danger' | 'warning' = 'warning'
                  if (item.disabled) {
                    if (item.disabled.source === 'manual') {
                      disabledLabel = t('Disabled')
                      disabledVariant = 'danger'
                    } else if (item.disabled.banned_until > Date.now() / 1000) {
                      disabledLabel = t('Banned ({{time}})', {
                        time: formatSeconds(
                          item.disabled.banned_until - Date.now() / 1000
                        ),
                      })
                    } else if (item.disabled.banned_until === 0) {
                      // Permanent auto ban: never expires, recovery probe
                      // no longer re-tests it.
                      disabledLabel = t('Banned permanently')
                    } else {
                      disabledLabel = t('Banned')
                    }
                  }
                  // Match on the routable model (group.name), the demotion key
                  // recorded by RecordSlowStream — see demotedCount above.
                  const demotion = demoted
                    .get(item.channel_id)
                    ?.find((d) => d.model === group.name)
                  return (
                    <TableRow key={item.id}>
                      <TableCell>
                        <span className='font-mono text-xs break-all'>
                          {item.model}
                        </span>
                        {item.source_group && (
                          <StatusBadge
                            variant='info'
                            size='sm'
                            className='ml-1 align-middle'
                          >
                            {t('from {{group}}', {
                              group: item.source_group,
                            })}
                          </StatusBadge>
                        )}
                        {item.disabled && (
                          <TooltipProvider delay={100}>
                            <Tooltip>
                              <TooltipTrigger render={<span />}>
                                <StatusBadge
                                  variant={disabledVariant}
                                  size='sm'
                                  showDot
                                  className='ml-1 align-middle'
                                >
                                  {disabledLabel}
                                </StatusBadge>
                              </TooltipTrigger>
                              <TooltipContent side='top' className='max-w-xs'>
                                <div className='space-y-1 text-xs'>
                                  <div className='font-medium'>
                                    {t('Model-level disabled')}
                                  </div>
                                  <div>
                                    {item.disabled.source === 'auto'
                                      ? t('Auto')
                                      : t('Manual')}
                                  </div>
                                  {item.disabled.reason && (
                                    <div>
                                      {t('Reason:')} {item.disabled.reason}
                                    </div>
                                  )}
                                  {item.disabled.last_error &&
                                    item.disabled.last_error !==
                                      item.disabled.reason && (
                                      <div className='break-words'>
                                        {t('Last probe error:')}{' '}
                                        {item.disabled.last_error}
                                      </div>
                                    )}
                                  {!!item.disabled.created_at && (
                                    <div>
                                      {t('Time:')}{' '}
                                      {formatTimestampToDate(
                                        item.disabled.created_at
                                      )}
                                    </div>
                                  )}
                                  {item.disabled.banned_until >
                                    Date.now() / 1000 && (
                                    <div>
                                      {t('recovers in')}{' '}
                                      {formatSeconds(
                                        item.disabled.banned_until -
                                          Date.now() / 1000
                                      )}
                                    </div>
                                  )}
                                  {item.disabled.banned_until === 0 && (
                                    <div>{t('Permanent')}</div>
                                  )}
                                </div>
                              </TooltipContent>
                            </Tooltip>
                          </TooltipProvider>
                        )}
                        {demotion && (
                          <TooltipProvider delay={100}>
                            <Tooltip>
                              <TooltipTrigger render={<span />}>
                                <StatusBadge
                                  variant='warning'
                                  size='sm'
                                  showDot
                                  className='ml-1 align-middle'
                                >
                                  {t('Demoted ({{time}})', {
                                    time: formatSeconds(
                                      demotion.remaining_seconds
                                    ),
                                  })}
                                </StatusBadge>
                              </TooltipTrigger>
                              <TooltipContent side='top' className='max-w-xs'>
                                <div className='space-y-1 text-xs'>
                                  <div className='font-medium'>
                                    {t('Temporarily demoted (slow latency)')}
                                  </div>
                                  <div>
                                    {(demotion.sources ?? []).length > 0
                                      ? (demotion.sources ?? [])
                                          .map((s) =>
                                            s === 'tps'
                                              ? t('Slow generation rate')
                                              : t('Slow first-token latency')
                                          )
                                          .join(' + ')
                                      : t('Slow generation rate')}
                                  </div>
                                  <div>
                                    {t('recovers in')}{' '}
                                    {formatSeconds(
                                      demotion.remaining_seconds
                                    )}
                                  </div>
                                </div>
                              </TooltipContent>
                            </Tooltip>
                          </TooltipProvider>
                        )}
                      </TableCell>
                      <TableCell>
                        {item.channel_name}
                        <span className='text-muted-foreground ml-1 text-xs'>
                          #{item.channel_id}
                        </span>
                        {item.channel_status != null &&
                          item.channel_status !== 1 && (
                            <TooltipProvider delay={100}>
                              <Tooltip>
                                <TooltipTrigger render={<span />}>
                                  <StatusBadge
                                    variant={
                                      item.channel_status === 3
                                        ? 'warning'
                                        : 'danger'
                                    }
                                    size='sm'
                                    showDot
                                    className='ml-1 align-middle'
                                  >
                                    {item.channel_status === 3
                                      ? t('Auto Disabled')
                                      : t('Manually Disabled')}
                                  </StatusBadge>
                                </TooltipTrigger>
                                <TooltipContent side='top' className='max-w-xs'>
                                  <div className='space-y-1 text-xs'>
                                    <div className='font-medium'>
                                      {t('Channel-level disabled')}
                                    </div>
                                    {item.channel_status_reason && (
                                      <div>
                                        {t('Reason:')}{' '}
                                        {item.channel_status_reason}
                                      </div>
                                    )}
                                    {!!item.channel_status_time && (
                                      <div>
                                        {t('Time:')}{' '}
                                        {formatTimestampToDate(
                                          item.channel_status_time
                                        )}
                                      </div>
                                    )}
                                  </div>
                                </TooltipContent>
                              </Tooltip>
                            </TooltipProvider>
                          )}
                      </TableCell>
                      <TableCell>
                        <Input
                          type='number'
                          className='h-8'
                          value={edit.priorityInput}
                          placeholder={
                            item.channel_priority
                              ? `${t('Inherited')}: ${item.channel_priority}`
                              : t('Inherited')
                          }
                          onChange={(e) =>
                            setEdits((prev) => ({
                              ...prev,
                              [item.id]: {
                                ...getEdit(item),
                                priorityInput: e.target.value,
                              },
                            }))
                          }
                        />
                      </TableCell>
                      <TableCell>
                        <Input
                          type='number'
                          className='h-8'
                          value={edit.weightInput}
                          placeholder={
                            item.channel_weight
                              ? `${t('Inherited')}: ${item.channel_weight}`
                              : t('Inherited')
                          }
                          onChange={(e) =>
                            setEdits((prev) => ({
                              ...prev,
                              [item.id]: {
                                ...getEdit(item),
                                weightInput: e.target.value,
                              },
                            }))
                          }
                        />
                      </TableCell>
                      <TableCell>
                        <Switch
                          checked={item.enabled}
                          onCheckedChange={(checked) =>
                            props.onToggleMember(item, checked)
                          }
                        />
                      </TableCell>
                      <TableCell>
                        <div className='flex items-center gap-1'>
                          <Button
                            variant='ghost'
                            size='icon-sm'
                            disabled={testingId !== null || props.updatingMember}
                            onClick={() => testMember(item)}
                            title={t('Test')}
                          >
                            <Zap
                              className={cn(
                                'h-4 w-4',
                                testingId === item.id && 'animate-pulse'
                              )}
                            />
                          </Button>
                          <Button
                            variant='ghost'
                            size='icon-sm'
                            disabled={!dirty || props.updatingMember}
                            onClick={() => saveMember(item)}
                          >
                            <Save className='h-4 w-4' />
                          </Button>
                          <Button
                            variant='ghost'
                            size='icon-sm'
                            disabled={
                              group.source === 'auto' || !!item.source_group
                            }
                            onClick={() => props.onDeleteMember(group, item)}
                          >
                            <Trash2 className='h-4 w-4' />
                          </Button>
                        </div>
                      </TableCell>
                    </TableRow>
                  )
                })}
              </TableBody>
            </Table>
          )}
          {group.source !== 'auto' && (
            <div className='mt-3'>
              <Button
                variant='outline'
                size='sm'
                onClick={() => props.onAddMember(group)}
              >
                <Plus className='mr-1 h-4 w-4' />
                {t('Add Member')}
              </Button>
            </div>
          )}
        </div>
      </CollapsibleContent>
    </Collapsible>
  )
}
