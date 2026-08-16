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
import { useQuery } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'
import type { ColumnDef } from '@tanstack/react-table'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { DataTablePage, useDataTable } from '@/components/data-table'
import { useTableUrlState } from '@/hooks/use-table-url-state'

import { getInflightRequests, type InflightRequest } from '../inflight-api'

const route = getRouteApi('/_authenticated/usage-logs/$section')

function formatDuration(seconds: number): string {
  if (seconds < 60) return `${seconds}s`
  const m = Math.floor(seconds / 60)
  const s = seconds % 60
  return `${m}m ${s}s`
}

function useElapsedCounter(): number {
  const [now, setNow] = useState(() => Math.floor(Date.now() / 1000))
  useEffect(() => {
    const id = setInterval(() => setNow(Math.floor(Date.now() / 1000)), 1000)
    return () => clearInterval(id)
  }, [])
  return now
}

export function InflightTable() {
  const { t } = useTranslation()
  const searchParams = route.useSearch()
  const navigate = route.useNavigate()
  const now = useElapsedCounter()

  const { pagination, onPaginationChange, ensurePageInRange } =
    useTableUrlState({
      search: searchParams,
      navigate,
      pagination: { defaultPage: 1, defaultPageSize: 50 },
      globalFilter: { enabled: false },
      columnFilters: [],
    })

  const { data, isLoading, isFetching } = useQuery({
    queryKey: [
      'inflight-requests',
      pagination.pageIndex + 1,
      pagination.pageSize,
    ],
    queryFn: () =>
      getInflightRequests(pagination.pageIndex + 1, pagination.pageSize),
    refetchInterval: 3000,
    placeholderData: (prev) => prev,
  })

  const items = data?.items ?? []
  const total = data?.total ?? 0

  const columns: ColumnDef<InflightRequest>[] = [
  {
  accessorKey: 'request_id',
  header: () => t('Request ID'),
  cell: ({ row }) => (
  <span className='font-mono text-xs' title={row.original.request_id}>
    {row.original.request_id?.slice(0, 8)}
  </span>
  ),
  size: 180,
  },
  {
  accessorKey: 'channel_name',
  header: () => t('Channel Name'),
  cell: ({ row }) => (
  <span className='font-mono'>
    {row.original.channel_name || row.original.channel_id}
  </span>
  ),
  size: 140,
  },
  {
  accessorKey: 'channel_id',
  header: () => t('Channel ID'),
  cell: ({ row }) => (
  <span className='font-mono'>#{row.original.channel_id}</span>
  ),
  size: 100,
  },
    {
      accessorKey: 'model_name',
      header: () => t('Model'),
      cell: ({ row }) => row.original.model_name || '-',
      size: 200,
    },
  {
  accessorKey: 'request_path',
  header: () => t('Request Path'),
  cell: ({ row }) => (
  <span className='font-mono text-xs'>
  {row.original.request_path}
  </span>
  ),
  size: 250,
  },
  {
  accessorKey: 'client_ip',
  header: () => t('Client IP'),
  cell: ({ row }) => row.original.client_ip || '-',
  size: 140,
  },
  {
  accessorKey: 'key_name',
  header: () => t('Key Name'),
  cell: ({ row }) => row.original.key_name || '-',
  size: 180,
  },
  {
  id: 'elapsed',
  header: () => t('Elapsed'),
  cell: ({ row }) => {
  const finished = row.original.finished;
  const endTime = row.original.end_time;
  const elapsed = finished && endTime ? endTime - row.original.start_time : now - row.original.start_time;
  return (
  <span className='font-mono tabular-nums'>
    {formatDuration(elapsed)}
    {finished && <span className='ml-1 text-green-500'>(✓)</span>}
  </span>
  )
  },
  size: 140,
  },
    {
      accessorKey: 'start_time',
      header: () => t('Started At'),
      cell: ({ row }) => {
        const date = new Date(row.original.start_time * 1000)
        return (
          <span className='font-mono text-xs'>
            {date.toLocaleTimeString()}
          </span>
        )
      },
      size: 120,
    },
  ]

  const { table } = useDataTable({
    data: items as unknown as Record<string, unknown>[],
    columns: columns as ColumnDef<Record<string, unknown>>[],
    pagination,
    enableRowSelection: false,
    onPaginationChange,
    manualPagination: true,
    manualFiltering: true,
    totalCount: total,
    ensurePageInRange,
  })

  return (
    <DataTablePage
      table={table}
      columns={columns as ColumnDef<Record<string, unknown>>[]}
      isLoading={isLoading}
      isFetching={isFetching}
      emptyTitle={t('No Active Connections')}
      emptyDescription={t(
        'No in-flight requests at the moment. Active relay requests will appear here in real time.'
      )}
      skeletonKeyPrefix='inflight-skeleton'
      showPagination
    />
  )
}
