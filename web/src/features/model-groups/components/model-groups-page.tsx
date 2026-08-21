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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  ChevronDown,
  ChevronRight,
  ChevronUp,
  Plus,
  RefreshCw,
  Save,
  Search,
  Trash2,
  X,
} from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { Dialog } from '@/components/dialog'
import { SectionPageLayout } from '@/components/layout'
import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  formatSeconds,
  getChannelTypeLabel,
  parseModelsList,
} from '@/features/channels/lib'
import { getChannels } from '@/features/channels/api'
import type { Channel } from '@/features/channels/types'

import {
  listModelGroups,
  createModelGroup,
  setModelGroupEnabled,
  deleteModelGroup,
  setModelGroupParamOverride,
  addGroupItem,
  updateGroupItem,
  deleteGroupItem,
  addGroupReference,
  deleteGroupReference,
  rebuildModelGroups,
  type ModelGroup,
  type ModelGroupItem,
  type ModelGroupReference,
} from '../lib/api'

interface MemberEditState {
  priorityInput: string
  weightInput: string
}

export function ModelGroupsPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [createOpen, setCreateOpen] = useState(false)
  const [newName, setNewName] = useState('')
  const [deleteTarget, setDeleteTarget] = useState<ModelGroup | null>(null)
  const [deleteItemTarget, setDeleteItemTarget] = useState<{
    group: ModelGroup
    item: ModelGroupItem
  } | null>(null)
  const [paramOverrideTarget, setParamOverrideTarget] = useState<ModelGroup | null>(null)
  const [paramOverrideDraft, setParamOverrideDraft] = useState('')
  const [addMemberGroup, setAddMemberGroup] = useState<ModelGroup | null>(null)
  const [addSelected, setAddSelected] = useState<Set<string>>(new Set())
  const [memberSearch, setMemberSearch] = useState('')
  const [addRefMode, setAddRefMode] = useState<'channel' | 'group'>('channel')
  const [addRefGroupId, setAddRefGroupId] = useState('')
  const [deleteRefTarget, setDeleteRefTarget] = useState<{
    group: ModelGroup
    ref: ModelGroupReference
  } | null>(null)
  const [expanded, setExpanded] = useState<Set<number>>(new Set())
  const [edits, setEdits] = useState<Record<number, MemberEditState>>({})
  const [rebuilding, setRebuilding] = useState(false)
  const [keyword, setKeyword] = useState('')
  const [sortBy, setSortBy] = useState<'default' | 'name' | 'count'>('default')
  const [sortDir, setSortDir] = useState<'asc' | 'desc'>('asc')

  const { data, isLoading } = useQuery({
    queryKey: ['model-groups'],
    queryFn: () => listModelGroups(true),
  })

  const { data: channelsData } = useQuery({
    queryKey: ['model-groups-channels'],
    queryFn: async () => {
      const items: Channel[] = []
      for (let p = 1; p <= 50; p++) {
        const res = await getChannels({ p, page_size: 100 })
        const pageItems = res.data?.items ?? []
        items.push(...pageItems)
        const total = res.data?.total ?? items.length
        if (pageItems.length === 0 || items.length >= total) break
      }
      return items
    },
    enabled: !!addMemberGroup,
  })

  const invalidate = () =>
    queryClient.invalidateQueries({ queryKey: ['model-groups'] })

  const rebuildMutation = useMutation({
    mutationFn: () => rebuildModelGroups(),
    onMutate: () => setRebuilding(true),
    onSettled: () => setRebuilding(false),
    onSuccess: (res) => {
      if (res.success) {
        toast.success(t('Model routing rebuilt'))
      } else {
        toast.error(res.message || t('Failed to rebuild model routing'))
      }
      invalidate()
    },
    onError: () => toast.error(t('Failed to rebuild model routing')),
  })

  const createMutation = useMutation({
    mutationFn: () => createModelGroup(newName.trim()),
    onError: () => toast.error(t('Failed to create model group')),
    onSuccess: (res) => {
      if (res.success) {
        toast.success(t('Model group created'))
        setCreateOpen(false)
        setNewName('')
        invalidate()
      } else {
        toast.error(res.message || t('Failed to create model group'))
      }
    },
  })

  const toggleMutation = useMutation({
    mutationFn: ({ id, enabled }: { id: number; enabled: boolean }) =>
      setModelGroupEnabled(id, enabled),
    onSuccess: () => invalidate(),
  })

  const deleteMutation = useMutation({
    mutationFn: (id: number) => deleteModelGroup(id),
    onSuccess: (res) => {
      if (res.success) {
        toast.success(t('Model group deleted'))
        setDeleteTarget(null)
        invalidate()
      } else {
        toast.error(res.message || t('Failed to delete model group'))
      }
    },
  })

  const itemDeleteMutation = useMutation({
    mutationFn: ({ itemId }: { itemId: number }) => deleteGroupItem(itemId),
    onError: () => toast.error(t('Failed to remove member')),
    onSuccess: (res, { itemId }) => {
      if (res.success) {
        toast.success(t('Member removed'))
        setDeleteItemTarget(null)
        invalidate()
      } else {
        toast.error(res.message || t('Failed to remove member'))
      }
    },
  })

  const itemUpdateMutation = useMutation({
    mutationFn: ({
      itemId,
      priority,
      weight,
      enabled,
    }: {
      itemId: number
      priority?: number | null
      weight?: number | null
      enabled?: boolean
    }) => updateGroupItem(itemId, { priority, weight, enabled }),
    onError: () => toast.error(t('Failed to update member')),
    onSuccess: (res, vars) => {
      if (res.success) {
        toast.success(t('Member updated'))
        setEdits((prev) => {
          const next = { ...prev }
          delete next[vars.itemId]
          return next
        })
        invalidate()
      } else {
        toast.error(res.message || t('Failed to update member'))
      }
    },
  })

  const itemToggleMutation = useMutation({
    mutationFn: ({ itemId, enabled }: { itemId: number; enabled: boolean }) =>
      updateGroupItem(itemId, { enabled }),
    onSuccess: () => invalidate(),
  })

  const addMemberMutation = useMutation({
    mutationFn: async () => {
      const failures: string[] = []
      for (const value of addSelected) {
        const sep = value.indexOf('|')
        const channelId = value.slice(0, sep)
        const model = value.slice(sep + 1)
        const res = await addGroupItem(addMemberGroup!.id, Number(channelId), model)
        if (!res.success) {
          failures.push(`${model} (#${channelId})`)
        }
      }
      return failures
    },
    onError: () => toast.error(t('Failed to add member')),
    onSuccess: (failures) => {
      if (failures.length === 0) {
        toast.success(t('{{count}} members added', { count: addSelected.size }))
        setAddMemberGroup(null)
        invalidate()
      } else {
        toast.error(
          `${t('Failed to add member')}: ${failures.join(', ')}`
        )
        setAddSelected(new Set(failures))
      }
    },
  })

  const addReferenceMutation = useMutation({
    mutationFn: () =>
      addGroupReference(addMemberGroup!.id, Number(addRefGroupId)),
    onError: () => toast.error(t('Failed to add group reference')),
    onSuccess: (res) => {
      if (res.success) {
        toast.success(t('Group reference added'))
        setAddMemberGroup(null)
        setAddRefGroupId('')
        invalidate()
      } else {
        toast.error(res.message || t('Failed to add group reference'))
      }
    },
  })

  const deleteReferenceMutation = useMutation({
    mutationFn: ({
      groupId,
      refGroupId,
    }: {
      groupId: number
      refGroupId: number
    }) => deleteGroupReference(groupId, refGroupId),
    onError: () => toast.error(t('Failed to remove group reference')),
    onSuccess: (res) => {
      if (res.success) {
        toast.success(t('Group reference removed'))
        setDeleteRefTarget(null)
        invalidate()
      } else {
        toast.error(res.message || t('Failed to remove group reference'))
      }
    },
  })

  const paramOverrideMutation = useMutation({
    mutationFn: ({ id, override }: { id: number; override: string }) =>
      setModelGroupParamOverride(id, override),
    onSuccess: (res) => {
      if (res.success) {
        toast.success(t('Parameter override saved'))
        setParamOverrideTarget(null)
        invalidate()
      } else {
        toast.error(res.message || t('Failed to save parameter override'))
      }
    },
  })

  const toggleExpanded = (id: number) => {
    setExpanded((prev) => {
      const next = new Set(prev)
      if (next.has(id)) {
        next.delete(id)
      } else {
        next.add(id)
      }
      return next
    })
  }

  const allGroups = useMemo(() => data?.items ?? [], [data])

  const visibleGroups = useMemo(() => {
    const kw = keyword.trim().toLowerCase()
    const filtered = kw
      ? allGroups.filter(
          (g) =>
            g.name.toLowerCase().includes(kw) ||
            (g.members ?? []).some((m) => m.model.toLowerCase().includes(kw))
        )
      : allGroups

    const dir = sortDir === 'desc' ? -1 : 1
    return [...filtered].sort((a, b) => {
      if (sortBy === 'name') {
        return a.name.localeCompare(b.name) * dir
      }
      if (sortBy === 'count') {
        return ((a.member_count ?? 0) - (b.member_count ?? 0)) * dir
      }
      // default: manual groups first, then auto groups, each by name
      const diff = (a.source === 'auto' ? 1 : 0) - (b.source === 'auto' ? 1 : 0)
      return diff !== 0 ? diff : a.name.localeCompare(b.name) * dir
    })
  }, [allGroups, keyword, sortBy, sortDir])

  // All (channel, model) pairs across every channel, listed at once with
  // client-side search in the add-member dialog.
  const memberOptions = useMemo(() => {
    const options: { value: string; label: string }[] = []
    for (const ch of channelsData ?? []) {
      const name = ch.name || `#${ch.id}`
      for (const m of parseModelsList(ch.models || '')) {
        options.push({
          value: `${ch.id}|${m}`,
          label: `${m} · ${name} (#${ch.id})`,
        })
      }
    }
    return options
  }, [channelsData])

  const visibleMemberOptions = useMemo(() => {
    const kw = memberSearch.trim().toLowerCase()
    if (!kw) return memberOptions
    return memberOptions.filter((o) => o.label.toLowerCase().includes(kw))
  }, [memberOptions, memberSearch])

  const toggleMemberOption = (value: string, checked: boolean) => {
    setAddSelected((prev) => {
      const next = new Set(prev)
      if (checked) {
        next.add(value)
      } else {
        next.delete(value)
      }
      return next
    })
  }

  const getEdit = (item: ModelGroupItem): MemberEditState => {
    const existing = edits[item.id]
    if (existing) return existing
    return {
      priorityInput: item.priority !== null && item.priority !== undefined ? String(item.priority) : '',
      weightInput: item.weight !== null && item.weight !== undefined ? String(item.weight) : '',
    }
  }

  return (
    <>
      <SectionPageLayout>
        <SectionPageLayout.Title>
          <span className='truncate'>{t('Model Groups')}</span>
        </SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <Button
          onClick={() => rebuildMutation.mutate()}
          disabled={rebuilding}
        >
          <RefreshCw
            className={`mr-2 h-4 w-4 ${rebuilding ? 'animate-spin' : ''}`}
          />
          {rebuilding ? t('Rebuilding') : t('Rebuild Model Routing')}
        </Button>
        <Button onClick={() => setCreateOpen(true)}>
          <Plus className='mr-2 h-4 w-4' />
          {t('Create Group')}
        </Button>
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        {isLoading ? (
          <div className='text-muted-foreground p-8 text-center'>
            {t('Loading')}...
          </div>
        ) : allGroups.length === 0 ? (
          <div className='text-muted-foreground p-8 text-center'>
            {t('No Model Groups')}
          </div>
        ) : (
          <div className='space-y-4'>
            {/* Toolbar: keyword filter + sorting */}
            <div className='flex flex-wrap items-center gap-2'>
              <div className='relative min-w-0 flex-1 basis-48'>
                <Search className='text-muted-foreground pointer-events-none absolute top-1/2 left-2.5 h-4 w-4 -translate-y-1/2' />
                <Input
                  className='pl-8'
                  placeholder={t('Search groups...')}
                  value={keyword}
                  onChange={(e) => setKeyword(e.target.value)}
                />
              </div>
              <Select
                value={sortBy}
                onValueChange={(v) =>
                  setSortBy(v as 'default' | 'name' | 'count')
                }
              >
                <SelectTrigger className='w-auto'>
                  <SelectValue placeholder={t('Sort')}>
                    {sortBy === 'default'
                      ? t('Manual first')
                      : sortBy === 'name'
                        ? t('Name')
                        : t('Member count')}
                  </SelectValue>
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value='default'>
                    {t('Manual first')}
                  </SelectItem>
                  <SelectItem value='name'>{t('Name')}</SelectItem>
                  <SelectItem value='count'>
                    {t('Member count')}
                  </SelectItem>
                </SelectContent>
              </Select>
              <Button
                variant='outline'
                size='icon-sm'
                disabled={sortBy === 'default'}
                onClick={() =>
                  setSortDir((d) => (d === 'asc' ? 'desc' : 'asc'))
                }
                title={sortDir === 'asc' ? t('Ascending') : t('Descending')}
              >
                {sortDir === 'asc' ? (
                  <ChevronUp className='h-4 w-4' />
                ) : (
                  <ChevronDown className='h-4 w-4' />
                )}
              </Button>
            </div>

            {visibleGroups.length === 0 ? (
              <div className='text-muted-foreground p-8 text-center'>
                {t('No matching results')}
              </div>
            ) : (
              visibleGroups.map((group) => {
              const isExpanded = expanded.has(group.id)
              const members = group.members ?? []
              const bannedCount = members.filter((m) => m.disabled).length
              return (
                <div
                  key={group.id}
                  className='border-border rounded-lg border'
                >
                  <div className='flex items-center gap-2 p-3'>
                    <Button
                      variant='ghost'
                      size='icon-sm'
                      onClick={() => toggleExpanded(group.id)}
                      aria-label={t('Toggle members')}
                    >
                      {isExpanded ? (
                        <ChevronDown className='h-4 w-4' />
                      ) : (
                        <ChevronRight className='h-4 w-4' />
                      )}
                    </Button>
                    <div className='min-w-0 flex-1'>
                      <div className='flex items-center gap-2'>
                        <span className='font-medium'>{group.name}</span>
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
                      </div>
                      <div className='text-muted-foreground text-xs'>
                        {t('{{count}} members', { count: members.length })}
                      </div>
                    </div>
                    <Switch
                      checked={group.enabled}
                      onCheckedChange={(checked) =>
                        toggleMutation.mutate({ id: group.id, enabled: checked })
                      }
                    />
                    <Button
                      variant='ghost'
                      size='icon-sm'
                      onClick={() => {
                        setParamOverrideTarget(group)
                        setParamOverrideDraft(group.param_override || '')
                      }}
                      title={t('Parameter override')}
                    >
                      <Save className='h-4 w-4' />
                    </Button>
                    <Button
                      variant='ghost'
                      size='icon-sm'
                      onClick={() => setDeleteTarget(group)}
                    >
                      <Trash2 className='h-4 w-4' />
                    </Button>
                  </div>

                  {isExpanded && (
                    <div className='border-border border-t p-3'>
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
                              <span className='font-medium'>
                                {ref.ref_group_name}
                              </span>
                              <Button
                                variant='ghost'
                                size='icon-sm'
                                className='h-4 w-4'
                                onClick={() =>
                                  setDeleteRefTarget({ group, ref })
                                }
                                title={t('Remove reference')}
                              >
                                <X className='h-3 w-3' />
                              </Button>
                            </div>
                          ))}
                          <span className='text-muted-foreground text-xs'>
                            {t('Edits to referenced members also apply to their source group')}
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
                              <TableHead className='w-32'>
                                {t('Priority (empty = inherit)')}
                              </TableHead>
                              <TableHead className='w-32'>
                                {t('Weight (empty = inherit)')}
                              </TableHead>
                              <TableHead>{t('Enabled')}</TableHead>
                              <TableHead className='w-24'>{t('Actions')}</TableHead>
                            </TableRow>
                          </TableHeader>
                          <TableBody>
                            {members.map((item) => {
                              const edit = getEdit(item)
                              const dirty =
                                edit.priorityInput !==
                                  (item.priority != null ? String(item.priority) : '') ||
                                edit.weightInput !==
                                  (item.weight != null ? String(item.weight) : '')
                              return (
                                <TableRow key={item.id}>
                                  <TableCell>
                                    {item.model}
                                    {item.source_group && (
                                      <StatusBadge variant='info' size='sm' className='ml-1 align-middle'>
                                        {t('from {{group}}', { group: item.source_group })}
                                      </StatusBadge>
                                    )}
                                    {item.disabled && (
                                      <StatusBadge
                                        variant={
                                          item.disabled.source === 'manual'
                                            ? 'danger'
                                            : 'warning'
                                        }
                                        size='sm'
                                        showDot
                                        className='ml-1 align-middle'
                                        title={item.disabled.reason || undefined}
                                      >
                                        {item.disabled.source === 'manual'
                                          ? t('Disabled')
                                          : item.disabled.banned_until >
                                              Date.now() / 1000
                                            ? t('Banned ({{time}})', {
                                                time: formatSeconds(
                                                  item.disabled.banned_until -
                                                    Date.now() / 1000
                                                ),
                                              })
                                            : t('Banned')}
                                      </StatusBadge>
                                    )}
                                  </TableCell>
                                  <TableCell className='font-medium'>
                                    {item.channel_name || `#${item.channel_id}`}
                                    <span className='text-muted-foreground ml-1 text-xs'>
                                      {getChannelTypeLabel(item.channel_type)}
                                    </span>
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
                                        itemToggleMutation.mutate({
                                          itemId: item.id,
                                          enabled: checked,
                                        })
                                      }
                                    />
                                  </TableCell>
                                  <TableCell>
                                    <div className='flex items-center gap-1'>
                                      <Button
                                        variant='ghost'
                                        size='icon-sm'
                                        disabled={!dirty || itemUpdateMutation.isPending}
                                        onClick={() =>
                                          itemUpdateMutation.mutate({
                                            itemId: item.id,
                                            priority: edit.priorityInput === '' ? null : Number(edit.priorityInput),
                                            weight: edit.weightInput === '' ? null : Number(edit.weightInput),
                                          })
                                        }
                                      >
                                        <Save className='h-4 w-4' />
                                      </Button>
                                      <Button
                                        variant='ghost'
                                        size='icon-sm'
                                        disabled={group.source === 'auto' || !!item.source_group}
                                        onClick={() =>
                                          setDeleteItemTarget({ group, item })
                                        }
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
                            onClick={() => {
                              setAddMemberGroup(group)
                              setAddSelected(new Set())
                              setMemberSearch('')
                              setAddRefMode('channel')
                              setAddRefGroupId('')
                            }}
                          >
                            <Plus className='mr-1 h-4 w-4' />
                            {t('Add Member')}
                          </Button>
                        </div>
                      )}
                    </div>
                  )}
                </div>
              )
            })
            )}
          </div>
        )}
      </SectionPageLayout.Content>
      </SectionPageLayout>

      {/* Create group dialog */}
      <Dialog
        open={createOpen}
        onOpenChange={setCreateOpen}
        title={t('Create Model Group')}
        description={t('A model group name must match the routable model name.')}
        footer={
          <Button
            disabled={!newName.trim() || createMutation.isPending}
            onClick={() => createMutation.mutate()}
          >
            {t('Create')}
          </Button>
        }
      >
        <Input
          placeholder={t('Enter group name (model name)')}
          value={newName}
          onChange={(e) => setNewName(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter' && newName.trim()) createMutation.mutate()
          }}
        />
      </Dialog>

      {/* Add member dialog */}
      <Dialog
        open={!!addMemberGroup}
        onOpenChange={(open) => !open && setAddMemberGroup(null)}
        title={`${t('Add Member')} — ${addMemberGroup?.name ?? ''}`}
        description={t('Add real upstream models from any channel. Priority/weight empty = inherit the channel values.')}
        footer={
          addRefMode === 'channel' ? (
            <Button
              disabled={
                addSelected.size === 0 ||
                addMemberMutation.isPending
              }
              onClick={() => addMemberMutation.mutate()}
            >
              {t('Add')}
            </Button>
          ) : (
            <Button
              disabled={!addRefGroupId || addReferenceMutation.isPending}
              onClick={() => addReferenceMutation.mutate()}
            >
              {t('Add')}
            </Button>
          )
        }
      >
        <div className='space-y-3'>
          <div className='flex gap-2'>
            <Button
              type='button'
              variant={addRefMode === 'channel' ? 'default' : 'outline'}
              size='sm'
              onClick={() => setAddRefMode('channel')}
            >
              {t('Channel model')}
            </Button>
            <Button
              type='button'
              variant={addRefMode === 'group' ? 'default' : 'outline'}
              size='sm'
              onClick={() => setAddRefMode('group')}
            >
              {t('Reference group')}
            </Button>
          </div>
          {addRefMode === 'group' ? (
            <div>
              <label className='text-muted-foreground mb-1 block text-sm'>
                {t('Referenced group')}
              </label>
              <Select
                value={addRefGroupId}
                onValueChange={setAddRefGroupId}
              >
                <SelectTrigger className='w-full'>
                  <SelectValue placeholder={t('Select a group to include all its members')} />
                </SelectTrigger>
                <SelectContent>
                  {(data?.items ?? [])
                    .filter(
                      (g) =>
                        g.id !== addMemberGroup?.id &&
                        !(g.references ?? []).some(
                          (ref) => ref.ref_group_id === addMemberGroup?.id
                        )
                    )
                    .map((g) => (
                      <SelectItem key={g.id} value={String(g.id)}>
                        {g.name} ({g.member_count ?? 0} members)
                      </SelectItem>
                    ))}
                </SelectContent>
              </Select>
              <p className='text-muted-foreground mt-2 text-xs'>
                {t('Members are aggregated and updated live; duplicates with the direct members are merged.')}
              </p>
            </div>
          ) : (
            <div className='space-y-2'>
              <div className='relative'>
                <Search className='text-muted-foreground pointer-events-none absolute top-1/2 left-2.5 h-4 w-4 -translate-y-1/2' />
                <Input
                  className='pl-8'
                  placeholder={t('Search all channel models...')}
                  value={memberSearch}
                  onChange={(e) => setMemberSearch(e.target.value)}
                />
              </div>
              <div className='border-border max-h-64 overflow-y-auto rounded-md border p-1'>
                {visibleMemberOptions.length === 0 ? (
                  <div className='text-muted-foreground py-6 text-center text-sm'>
                    {t('No matching channel model.')}
                  </div>
                ) : (
                  visibleMemberOptions.map((option) => (
                    <div
                      key={option.value}
                      className='hover:bg-accent flex items-center gap-2 rounded-sm px-2 py-1.5'
                    >
                      <Checkbox
                        id={`member-option-${option.value}`}
                        checked={addSelected.has(option.value)}
                        onCheckedChange={(checked) =>
                          toggleMemberOption(option.value, !!checked)
                        }
                      />
                      <Label
                        htmlFor={`member-option-${option.value}`}
                        className='flex-1 cursor-pointer truncate text-sm font-normal'
                      >
                        {option.label}
                      </Label>
                    </div>
                  ))
                )}
              </div>
              <p className='text-muted-foreground text-xs'>
                {t('{{count}} selected', { count: addSelected.size })}
              </p>
            </div>
          )}
        </div>
      </Dialog>

      {/* Param override dialog */}
      <Dialog
        open={!!paramOverrideTarget}
        onOpenChange={(open) => !open && setParamOverrideTarget(null)}
        title={`${t('Parameter override')} — ${paramOverrideTarget?.name ?? ''}`}
        description={t('Group-level parameter override JSON (same schema as channel param_override). Empty = no override.')}
        footer={
          <Button
            disabled={paramOverrideMutation.isPending}
            onClick={() =>
              paramOverrideTarget &&
              paramOverrideMutation.mutate({
                id: paramOverrideTarget.id,
                override: paramOverrideDraft,
              })
            }
          >
            {t('Save')}
          </Button>
        }
      >
        <Input
          placeholder='{"temperature": 0.7}'
          value={paramOverrideDraft}
          onChange={(e) => setParamOverrideDraft(e.target.value)}
          className='font-mono'
        />
      </Dialog>

      {/* Delete group confirm */}
      <ConfirmDialog
        open={!!deleteTarget}
        onOpenChange={() => setDeleteTarget(null)}
        title={t('Delete Model Group')}
        desc={t('Delete group {{name}}? This will also remove all its members.', { name: deleteTarget?.name ?? '' })}
        confirmText={t('Delete')}
        handleConfirm={() => {
          if (deleteTarget) deleteMutation.mutate(deleteTarget.id)
        }}
      />

      {/* Delete member confirm */}
      <ConfirmDialog
        open={!!deleteItemTarget}
        onOpenChange={() => setDeleteItemTarget(null)}
        title={t('Remove Member')}
        desc={t('Remove {{model}} from {{group}}?', {
          model: deleteItemTarget?.item.model ?? '',
          group: deleteItemTarget?.group.name ?? '',
        })}
        confirmText={t('Remove')}
        handleConfirm={() => {
          if (deleteItemTarget) {
            itemDeleteMutation.mutate({ itemId: deleteItemTarget.item.id })
          }
        }}
      />

      {/* Delete group reference confirm */}
      <ConfirmDialog
        open={!!deleteRefTarget}
        onOpenChange={() => setDeleteRefTarget(null)}
        title={t('Remove Group Reference')}
        desc={t('Remove reference to {{ref}} from {{group}}? Its members will no longer be included.', {
          ref: deleteRefTarget?.ref.ref_group_name ?? '',
          group: deleteRefTarget?.group.name ?? '',
        })}
        confirmText={t('Remove')}
        handleConfirm={() => {
          if (deleteRefTarget) {
            deleteReferenceMutation.mutate({
              groupId: deleteRefTarget.group.id,
              refGroupId: deleteRefTarget.ref.ref_group_id,
            })
          }
        }}
      />
    </>
  )
}