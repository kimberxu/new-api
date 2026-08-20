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
import { Plus, Trash2 } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { Dialog } from '@/components/dialog'
import { SectionPageLayout } from '@/components/layout'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { DataTablePage, useDataTable } from '@/components/data-table'
import type { ColumnDef } from '@tanstack/react-table'

import {
  listModelGroups,
  createModelGroup,
  setModelGroupEnabled,
  deleteModelGroup,
  type ModelGroup,
} from '../lib/api'

export function ModelGroupsPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [createOpen, setCreateOpen] = useState(false)
  const [newName, setNewName] = useState('')
  const [deleteTarget, setDeleteTarget] = useState<ModelGroup | null>(null)

  const { data, isLoading } = useQuery({
    queryKey: ['model-groups'],
    queryFn: () => listModelGroups(true),
  })

  const createMutation = useMutation({
    mutationFn: () => createModelGroup(newName.trim()),
    onSuccess: (res) => {
      if (res.success) {
        toast.success(t('Model group created'))
        setCreateOpen(false)
        setNewName('')
        queryClient.invalidateQueries({ queryKey: ['model-groups'] })
      } else {
        toast.error(res.message || t('Failed to create model group'))
      }
    },
  })

  const toggleMutation = useMutation({
    mutationFn: ({ id, enabled }: { id: number; enabled: boolean }) =>
      setModelGroupEnabled(id, enabled),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['model-groups'] })
    },
  })

  const deleteMutation = useMutation({
    mutationFn: (id: number) => deleteModelGroup(id),
    onSuccess: (res) => {
      if (res.success) {
        toast.success(t('Model group deleted'))
        setDeleteTarget(null)
        queryClient.invalidateQueries({ queryKey: ['model-groups'] })
      } else {
        toast.error(res.message || t('Failed to delete model group'))
      }
    },
  })

  const columns = useMemo<ColumnDef<ModelGroup>[]>(
    () => [
      {
        accessorKey: 'name',
        header: t('Group Name'),
      },
      {
        accessorKey: 'source',
        header: t('Source'),
        cell: ({ row }) => (
          <span className='text-muted-foreground text-xs'>
            {row.original.source === 'auto' ? t('Auto') : t('Manual')}
          </span>
        ),
      },
      {
        accessorKey: 'member_count',
        header: t('Members'),
        cell: ({ row }) => <span>{row.original.member_count ?? 0}</span>,
      },
      {
        id: 'enabled',
        header: t('Enabled'),
        cell: ({ row }) => (
          <Switch
            checked={row.original.enabled}
            onCheckedChange={(checked) =>
              toggleMutation.mutate({ id: row.original.id, enabled: checked })
            }
          />
        ),
      },
      {
        id: 'actions',
        header: t('Actions'),
        cell: ({ row }) => (
          <div className='flex items-center gap-1'>
            <Button
              variant='ghost'
              size='icon-sm'
              onClick={() => setDeleteTarget(row.original)}
            >
              <Trash2 className='h-4 w-4' />
            </Button>
          </div>
        ),
      },
    ],
    [t, toggleMutation]
  )

  const { table } = useDataTable({
    data: data?.items ?? [],
    columns,
    rowCount: data?.total ?? 0,
  })

  return (
    <SectionPageLayout fixedContent>
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
        <DataTablePage
          table={table}
          columns={columns}
          isLoading={isLoading}
          emptyTitle={t('No Model Groups')}
        />
      </SectionPageLayout.Content>

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
    </SectionPageLayout>
  )
}