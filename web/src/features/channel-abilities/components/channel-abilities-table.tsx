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
import { getRouteApi } from '@tanstack/react-router'
import type { ColumnDef } from '@tanstack/react-table'
import { Loader2, TestTube } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import {
  DataTablePage,
  useDataTable,
  useDebouncedColumnFilter,
} from '@/components/data-table'
import { GroupBadge } from '@/components/group-badge'
import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { getChannelTypeLabel } from '@/features/channels/lib'
import { useTableUrlState } from '@/hooks/use-table-url-state'

import {
  disableChannelAbilities,
  enableChannelAbilities,
  getChannelAbilities,
  testChannelModel,
  type ChannelAbilityItem,
  type ChannelAbilityStatus,
} from '../lib/api'

const route = getRouteApi('/_authenticated/channel-abilities/')

const STATUS_FILTER_OPTIONS = [
  { label: 'All', value: 'all' },
  { label: 'Enabled', value: 'enabled' },
  { label: 'Manually Disabled', value: 'manual_disabled' },
  { label: 'Automatically Disabled', value: 'auto_disabled' },
]

/** Row key: the ability has a composite key, so key rows by channel+model+group. */
export function getChannelAbilityRowId(item: ChannelAbilityItem): string {
  return `${item.channel_id}-${item.model}-${item.group}`
}

function formatTestDuration(responseTime: number): string {
  if (responseTime >= 1000) {
    return `${(responseTime / 1000).toFixed(2)}s`
  }
  return `${responseTime}ms`
}

function getServerErrorMessage(error: unknown): string | undefined {
  const err = error as { response?: { data?: { message?: string } } }
  return err?.response?.data?.message
}

function useChannelAbilitiesColumns(options: {
  onRequestDisable: (item: ChannelAbilityItem) => void
  onTestModel: (item: ChannelAbilityItem) => void
  testingKey: string | null
  toggleDisabled: boolean
}): ColumnDef<ChannelAbilityItem>[] {
  const { t } = useTranslation()
  const { onRequestDisable, onTestModel, testingKey, toggleDisabled } = options

  return useMemo<ColumnDef<ChannelAbilityItem>[]>(
    () => [
      {
        accessorKey: 'channel_name',
        header: () => t('Channel'),
        cell: ({ row }) => {
          const item = row.original
          return (
            <span className='font-mono'>
              #{item.channel_id}
              {item.channel_name ? `: ${item.channel_name}` : ''}
            </span>
          )
        },
        size: 200,
      },
      {
        accessorKey: 'channel_type',
        header: () => t('Type'),
        cell: ({ row }) => t(getChannelTypeLabel(row.original.channel_type)),
        size: 140,
      },
      {
        accessorKey: 'model',
        header: () => t('Model'),
        cell: ({ row }) => (
          <span className='font-mono text-xs'>{row.original.model}</span>
        ),
        size: 200,
      },
      {
        accessorKey: 'group',
        header: () => t('Group'),
        cell: ({ row }) => (
          <GroupBadge group={row.original.group} copyable={false} />
        ),
        size: 120,
      },
      {
        accessorKey: 'priority',
        header: () => t('Priority'),
        cell: ({ row }) => row.original.priority ?? 0,
        size: 100,
      },
      {
        accessorKey: 'weight',
        header: () => t('Weight'),
        cell: ({ row }) => row.original.weight,
        size: 100,
      },
      {
        id: 'status',
        header: () => t('Status'),
        cell: ({ row }) => {
          const item = row.original
          if (!item.disabled) {
            return (
              <StatusBadge
                label={t('Enabled')}
                variant='success'
                copyable={false}
              />
            )
          }
          const manual = item.disabled_source === 'manual'
          const badge = (
            <StatusBadge
              label={t(manual ? 'Manually Disabled' : 'Automatically Disabled')}
              variant={manual ? 'danger' : 'warning'}
              copyable={false}
            />
          )
          if (!item.disabled_reason) {
            return badge
          }
          return (
            <Tooltip>
              <TooltipTrigger render={<span className='inline-flex' />}>
                {badge}
              </TooltipTrigger>
              <TooltipContent>
                <p>{manual ? t('Manual') : t('Auto')}</p>
                <p className='max-w-[240px] text-xs'>
                  {item.disabled_reason}
                </p>
              </TooltipContent>
            </Tooltip>
          )
        },
        size: 180,
      },
      {
        id: 'actions',
        header: () => t('Actions'),
        cell: ({ row }) => {
          const item = row.original
          const rowKey = getChannelAbilityRowId(item)
          const isTesting = testingKey === rowKey
          return (
            <div className='flex items-center gap-2'>
              <Button
                variant='outline'
                size='sm'
                onClick={() => onTestModel(item)}
                disabled={isTesting}
              >
                {isTesting ? (
                  <Loader2 className='size-4 animate-spin' />
                ) : (
                  <TestTube className='size-4' />
                )}
                <span className='max-sm:hidden'>{t('Test Model')}</span>
              </Button>
              <Tooltip>
                <TooltipTrigger
                  render={
                    <Switch
                      checked={!item.disabled}
                      onCheckedChange={(checked) => {
                        if (checked) {
                          onRequestEnable(item)
                        } else {
                          onRequestDisable(item)
                        }
                      }}
                      disabled={toggleDisabled}
                      aria-label={t('Abilities')}
                    />
                  }
                />
                <TooltipContent>
                  <p>{item.disabled ? t('Enable') : t('Disable')}</p>
                </TooltipContent>
              </Tooltip>
            </div>
          )
        },
        size: 220,
      },
    ],
    [t, onRequestDisable, onTestModel, testingKey, toggleDisabled]
  )
}

export function ChannelAbilitiesTable() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [pendingDisable, setPendingDisable] =
    useState<ChannelAbilityItem | null>(null)
  const [testingKey, setTestingKey] = useState<string | null>(null)

  const {
    columnFilters,
    onColumnFiltersChange,
    pagination,
    onPaginationChange,
    ensurePageInRange,
  } = useTableUrlState({
    search: route.useSearch(),
    navigate: route.useNavigate(),
    pagination: { defaultPage: 1, defaultPageSize: 20 },
    globalFilter: { enabled: false },
    columnFilters: [
      { columnId: 'channelId', searchKey: 'channelId', type: 'string' },
      { columnId: 'model', searchKey: 'model', type: 'string' },
      { columnId: 'group', searchKey: 'group', type: 'string' },
      { columnId: 'status', searchKey: 'status', type: 'array' },
    ],
  })

  const channelIdFilter =
    (columnFilters.find((f) => f.id === 'channelId')?.value as string) ?? ''
  const statusFilter =
    (columnFilters.find((f) => f.id === 'status')?.value as string[]) ?? []
  const {
    value: modelFilter,
    inputValue: modelFilterInput,
    onChange: onModelFilterInputChange,
    onCompositionStart: onModelFilterCompositionStart,
    onCompositionEnd: onModelFilterCompositionEnd,
    resetInput: resetModelFilterInput,
  } = useDebouncedColumnFilter({
    columnFilters,
    columnId: 'model',
    onColumnFiltersChange,
  })
  const {
    value: groupFilter,
    inputValue: groupFilterInput,
    onChange: onGroupFilterInputChange,
    onCompositionStart: onGroupFilterCompositionStart,
    onCompositionEnd: onGroupFilterCompositionEnd,
    resetInput: resetGroupFilterInput,
  } = useDebouncedColumnFilter({
    columnFilters,
    columnId: 'group',
    onColumnFiltersChange,
  })

  const { data, isLoading, isFetching } = useQuery({
    queryKey: [
      'channel-abilities',
      pagination.pageIndex + 1,
      pagination.pageSize,
      channelIdFilter,
      modelFilter,
      groupFilter,
      statusFilter,
    ],
    queryFn: () =>
      getChannelAbilities({
        p: pagination.pageIndex + 1,
        page_size: pagination.pageSize,
        channel_id: channelIdFilter ? Number(channelIdFilter) : undefined,
        model: modelFilter || undefined,
        group: groupFilter || undefined,
        status: (statusFilter[0] ?? 'all') as ChannelAbilityStatus,
      }),
    placeholderData: (prev) => prev,
  })

  const items = data?.items ?? []
  const total = data?.total ?? 0

  const invalidateAbilities = () => {
    void queryClient.invalidateQueries({ queryKey: ['channel-abilities'] })
  }

  const disableMutation = useMutation({
    mutationFn: ({
      item,
      reason,
    }: {
      item: ChannelAbilityItem
      reason: string
    }) => disableChannelAbilities(item.channel_id, [item.model], reason),
    onSuccess: (res) => {
      if (res.success) {
        toast.success(t('Model disabled successfully'))
        invalidateAbilities()
      } else {
        toast.error(res.message || t('Failed to disable model'))
      }
    },
    onError: (error) => {
      const message = (error as { response?: { data?: { message?: string } } })
        ?.response?.data?.message
      toast.error(message || t('Failed to disable model'))
    },
  })

  const enableMutation = useMutation({
    mutationFn: (item: ChannelAbilityItem) =>
      enableChannelAbilities(item.channel_id, [item.model]),
    onSuccess: (res) => {
      if (res.success) {
        toast.success(t('Model enabled successfully'))
        invalidateAbilities()
      } else {
        toast.error(res.message || t('Failed to enable model'))
      }
    },
    onError: (error) => {
      const message = (error as { response?: { data?: { message?: string } } })
        ?.response?.data?.message
      toast.error(message || t('Failed to enable model'))
    },
  })

  const toggleDisabled = disableMutation.isPending || enableMutation.isPending

  const handleTestModel = async (item: ChannelAbilityItem) => {
    const rowKey = getChannelAbilityRowId(item)
    setTestingKey(rowKey)
    const target = t('Channel {{name}} model {{model}}', {
      name: item.channel_name || `#${item.channel_id}`,
      model: item.model,
    })
    try {
      const res = await testChannelModel(item.channel_id, item.model)
      if (res.success) {
        const responseTime = res.time ?? res.data?.response_time
        toast.success(
          t('{{target}} test succeeded', { target }),
          typeof responseTime === 'number'
            ? {
                description: t('Response time: {{duration}}', {
                  duration: formatTestDuration(responseTime),
                }),
              }
            : undefined
        )
        invalidateAbilities()
      } else {
        toast.error(t('{{target}} test failed', { target }), {
          description: res.message || res.data?.error,
        })
      }
    } catch (error) {
      toast.error(t('{{target}} test failed', { target }), {
        description: getServerErrorMessage(error),
      })
    } finally {
      setTestingKey(null)
    }
  }

  const handleToggle = (item: ChannelAbilityItem, checked: boolean) => {
    if (checked) {
      enableMutation.mutate(item)
    } else {
      setPendingDisable(item)
    }
  }

  const columns = useChannelAbilitiesColumns({
    onRequestDisable: setPendingDisable,
    onTestModel: handleTestModel,
    testingKey,
    toggleDisabled,
  })

  const { table } = useDataTable({
    data: items,
    columns,
    pagination,
    onPaginationChange,
    manualPagination: true,
    manualFiltering: true,
    enableRowSelection: false,
    totalCount: total,
    getRowId: getChannelAbilityRowId,
    ensurePageInRange,
  })

  return (
    <>
      <DataTablePage
        table={table}
        columns={columns}
        isLoading={isLoading}
        isFetching={isFetching}
        emptyTitle={t('No Channel Abilities Found')}
        emptyDescription={t('No channel abilities match your filters.')}
        skeletonKeyPrefix='channel-abilities-skeleton'
        toolbarProps={{
          searchPlaceholder: t('Filter by channel ID'),
          searchKey: 'channelId',
          searchDebounceMs: 500,
          onReset: () => {
            resetModelFilterInput()
            resetGroupFilterInput()
          },
          additionalSearch: (
            <div className='flex items-center gap-2'>
              <Input
                placeholder={t('Filter by model name...')}
                value={modelFilterInput}
                onChange={onModelFilterInputChange}
                onCompositionStart={onModelFilterCompositionStart}
                onCompositionEnd={onModelFilterCompositionEnd}
                className='w-full sm:w-[150px] lg:w-[180px]'
              />
              <Input
                placeholder={t('Filter by group')}
                value={groupFilterInput}
                onChange={onGroupFilterInputChange}
                onCompositionStart={onGroupFilterCompositionStart}
                onCompositionEnd={onGroupFilterCompositionEnd}
                className='w-full sm:w-[150px] lg:w-[180px]'
              />
            </div>
          ),
          filters: [
            {
              columnId: 'status',
              title: t('Status'),
              options: STATUS_FILTER_OPTIONS,
              singleSelect: true,
            },
          ],
        }}
      />

      <ConfirmDialog
        open={pendingDisable !== null}
        onOpenChange={(open) => {
          if (!open) {
            setPendingDisable(null)
          }
        }}
        title={t('Disable')}
        desc={t('Disable model {{model}} on channel {{name}}?', {
          model: pendingDisable?.model ?? '',
          name: pendingDisable?.channel_name
            ? pendingDisable.channel_name
            : pendingDisable
              ? `#${pendingDisable.channel_id}`
              : '',
        })}
        destructive
        confirmText={t('Disable')}
        handleConfirm={() => {
          if (pendingDisable) {
            disableMutation.mutate({
              item: pendingDisable,
              reason: t('Manually Disabled'),
            })
          }
          setPendingDisable(null)
        }}
      />
    </>
  )
}
