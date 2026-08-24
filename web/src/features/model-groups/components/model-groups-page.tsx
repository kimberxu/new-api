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
import { ChevronDown, ChevronUp, Layers, Plus, RefreshCw, Search } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { EmptyState } from '@/components/empty-state'
import { Dialog } from '@/components/dialog'
import { LoadingState } from '@/components/loading-state'
import { SectionPageLayout } from '@/components/layout'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { ParamOverrideEditorDialog } from '@/features/channels/components/dialogs/param-override-editor-dialog'

import { AddMemberDialog } from './add-member-dialog'
import { GroupCard } from './group-card'

import {
  listModelGroups,
  createModelGroup,
  setModelGroupEnabled,
  deleteModelGroup,
  setModelGroupParamOverride,
  addGroupItem,
  updateGroupItem,
  deleteGroupItem,
  testGroupItem,
  addGroupReference,
  deleteGroupReference,
  rebuildModelGroups,
  type ModelGroup,
  type ModelGroupItem,
  type ModelGroupReference,
} from '../lib/api'

const displayValue = (v: string) =>
  `${v.slice(v.indexOf('|') + 1)} (#${v.slice(0, v.indexOf('|'))})`

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
  const [paramOverrideTarget, setParamOverrideTarget] =
    useState<ModelGroup | null>(null)
  const [addMemberGroup, setAddMemberGroup] = useState<ModelGroup | null>(null)
  const [deleteRefTarget, setDeleteRefTarget] = useState<{
    group: ModelGroup
    ref: ModelGroupReference
  } | null>(null)
  const [expanded, setExpanded] = useState<Set<number>>(new Set())
  const [rebuilding, setRebuilding] = useState(false)
  const [keyword, setKeyword] = useState('')
  const [sortBy, setSortBy] = useState<'default' | 'name' | 'count'>('default')
  const [sortDir, setSortDir] = useState<'asc' | 'desc'>('asc')

  const { data, isLoading } = useQuery({
    queryKey: ['model-groups'],
    queryFn: () => listModelGroups(true),
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
    onSuccess: (res) => {
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
    onSuccess: (res) => {
      if (res.success) {
        toast.success(t('Member updated'))
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
    mutationFn: async (values: string[]) => {
      const failed: string[] = []
      for (const value of values) {
        const sep = value.indexOf('|')
        const channelId = value.slice(0, sep)
        const model = value.slice(sep + 1)
        const res = await addGroupItem(
          addMemberGroup!.id,
          Number(channelId),
          model
        )
        if (!res.success) {
          failed.push(value)
        }
      }
      return failed
    },
    onError: () => toast.error(t('Failed to add member')),
    onSuccess: (failed, values) => {
      if (failed.length === 0) {
        toast.success(t('{{count}} members added', { count: values.length }))
        setAddMemberGroup(null)
        invalidate()
      } else {
        toast.error(
          `${t('Failed to add member')}: ${failed.map(displayValue).join(', ')}`
        )
      }
    },
  })

  const addReferenceMutation = useMutation({
    mutationFn: (refGroupId: number) =>
      addGroupReference(addMemberGroup!.id, refGroupId),
    onError: () => toast.error(t('Failed to add group reference')),
    onSuccess: (res) => {
      if (res.success) {
        toast.success(t('Group reference added'))
        setAddMemberGroup(null)
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
          <LoadingState className='min-h-[300px]' />
        ) : allGroups.length === 0 ? (
          <EmptyState
            icon={Layers}
            title={t('No Model Groups')}
            description={t(
              'A model group name must match the routable model name.'
            )}
            action={
              <Button onClick={() => setCreateOpen(true)}>
                <Plus className='mr-2 h-4 w-4' />
                {t('Create Group')}
              </Button>
            }
          />
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
              <div className='text-muted-foreground rounded-lg border border-dashed p-8 text-center text-sm'>
                {t('No matching results')}
              </div>
            ) : (
              visibleGroups.map((group) => (
                <GroupCard
                  key={group.id}
                  group={group}
                  expanded={expanded.has(group.id)}
                  updatingMember={itemUpdateMutation.isPending}
                  onToggleExpanded={toggleExpanded}
                  onToggleEnabled={(id, enabled) =>
                    toggleMutation.mutate({ id, enabled })
                  }
                  onEditParams={setParamOverrideTarget}
                  onDelete={setDeleteTarget}
                  onAddMember={(g) => setAddMemberGroup(g)}
                  onDeleteReference={(g, ref) =>
                    setDeleteRefTarget({ group: g, ref })
                  }
                  onDeleteMember={(g, item) =>
                    setDeleteItemTarget({ group: g, item })
                  }
                  onUpdateMember={(item, priority, weight) =>
                    itemUpdateMutation.mutateAsync({
                      itemId: item.id,
                      priority,
                      weight,
                    })
                  }
                  onToggleMember={(item, enabled) =>
                    itemToggleMutation.mutate({ itemId: item.id, enabled })
                  }
                  onTestMember={async (item) => {
                    const res = await testGroupItem(item.id)
                    invalidate()
                    return res
                  }}
                />
              ))
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
      {addMemberGroup && (
        <AddMemberDialog
          group={addMemberGroup}
          groups={allGroups}
          submittingMembers={addMemberMutation.isPending}
          submittingReference={addReferenceMutation.isPending}
          onClose={() => setAddMemberGroup(null)}
          onSubmitMembers={(values) => addMemberMutation.mutateAsync(values)}
          onSubmitReference={(refGroupId) =>
            addReferenceMutation.mutate(refGroupId)
          }
        />
      )}

      {/* Param override dialog */}
      {paramOverrideTarget && (
        <ParamOverrideEditorDialog
          open
          value={paramOverrideTarget.param_override || ''}
          onOpenChange={(open) => {
            if (!open) setParamOverrideTarget(null)
          }}
          onSave={(nextValue) =>
            paramOverrideMutation.mutate({
              id: paramOverrideTarget.id,
              override: nextValue,
            })
          }
        />
      )}

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
