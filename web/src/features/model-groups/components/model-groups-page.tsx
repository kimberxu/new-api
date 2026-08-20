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
  Plus,
  Save,
  Trash2,
} from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { Dialog } from '@/components/dialog'
import { SectionPageLayout } from '@/components/layout'
import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
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
import { getChannelTypeLabel } from '@/features/channels/lib'
import { getChannels } from '@/features/channels/api'

import {
  listModelGroups,
  createModelGroup,
  setModelGroupEnabled,
  deleteModelGroup,
  setModelGroupParamOverride,
  addGroupItem,
  updateGroupItem,
  deleteGroupItem,
  getChannelModelOptions,
  type ModelGroup,
  type ModelGroupItem,
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
  const [addChannelId, setAddChannelId] = useState('')
  const [addModel, setAddModel] = useState('')
  const [expanded, setExpanded] = useState<Set<number>>(new Set())
  const [edits, setEdits] = useState<Record<number, MemberEditState>>({})

  const { data, isLoading } = useQuery({
    queryKey: ['model-groups'],
    queryFn: () => listModelGroups(true),
  })

  const { data: channelsData } = useQuery({
    queryKey: ['model-groups-channels'],
    queryFn: () => getChannels({ p: 1, page_size: 100 }),
    enabled: !!addMemberGroup,
  })

  const { data: channelModels } = useQuery({
    queryKey: ['model-group-channel-models', addChannelId],
    queryFn: () => getChannelModelOptions(Number(addChannelId)),
    enabled: !!addChannelId && Number(addChannelId) > 0,
  })

  const invalidate = () =>
    queryClient.invalidateQueries({ queryKey: ['model-groups'] })

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
    mutationFn: () =>
      addGroupItem(addMemberGroup!.id, Number(addChannelId), addModel),
    onError: () => toast.error(t('Failed to add member')),
    onSuccess: (res) => {
      if (res.success) {
        toast.success(t('Member added'))
        setAddMemberGroup(null)
        setAddChannelId('')
        setAddModel('')
        invalidate()
      } else {
        toast.error(res.message || t('Failed to add member'))
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

  const groups = useMemo(() => data?.items ?? [], [data])

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
        ) : groups.length === 0 ? (
          <div className='text-muted-foreground p-8 text-center'>
            {t('No Model Groups')}
          </div>
        ) : (
          <div className='space-y-4'>
            {groups.map((group) => {
              const isExpanded = expanded.has(group.id)
              const members = group.members ?? []
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
                      {members.length === 0 ? (
                        <div className='text-muted-foreground py-4 text-center text-sm'>
                          {t('No members in this group yet')}
                        </div>
                      ) : (
                        <Table>
                          <TableHeader>
                            <TableRow>
                              <TableHead>{t('Channel')}</TableHead>
                              <TableHead>{t('Model')}</TableHead>
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
                                  <TableCell className='font-medium'>
                                    {item.channel_name || `#${item.channel_id}`}
                                    <span className='text-muted-foreground ml-1 text-xs'>
                                      {getChannelTypeLabel(item.channel_type)}
                                    </span>
                                  </TableCell>
                                  <TableCell>{item.model}</TableCell>
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
                      <div className='mt-3'>
                        <Button
                          variant='outline'
                          size='sm'
                          onClick={() => {
                            setAddMemberGroup(group)
                            setAddChannelId('')
                            setAddModel('')
                          }}
                        >
                          <Plus className='mr-1 h-4 w-4' />
                          {t('Add Member')}
                        </Button>
                      </div>
                    </div>
                  )}
                </div>
              )
            })}
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
        description={t('Add a real upstream model of an existing channel. Priority/weight empty = inherit the channel values.')}
        footer={
          <Button
            disabled={
              !addChannelId ||
              !addModel ||
              addMemberMutation.isPending
            }
            onClick={() => addMemberMutation.mutate()}
          >
            {t('Add')}
          </Button>
        }
      >
        <div className='space-y-3'>
          <div>
            <label className='text-muted-foreground mb-1 block text-sm'>
              {t('Channel')}
            </label>
            <Select value={addChannelId} onValueChange={(v) => { setAddChannelId(v); setAddModel('') }}>
              <SelectTrigger className='w-full'>
                <SelectValue placeholder={t('Select a channel')} />
              </SelectTrigger>
              <SelectContent>
                {(channelsData?.data?.items ?? []).map((ch: { id: number; name: string }) => (
                  <SelectItem key={ch.id} value={String(ch.id)}>
                    {ch.name} (#{ch.id})
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          {addChannelId && (
            <div>
              <label className='text-muted-foreground mb-1 block text-sm'>
                {t('Model')}
              </label>
              <Select value={addModel} onValueChange={setAddModel}>
                <SelectTrigger className='w-full'>
                  <SelectValue placeholder={t('Select a model on the channel')} />
                </SelectTrigger>
                <SelectContent>
                  {(channelModels ?? []).map((m) => (
                    <SelectItem key={m} value={m}>
                      {m}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
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
    </>
  )
}