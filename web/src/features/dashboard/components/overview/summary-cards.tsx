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
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import { StaggerContainer, StaggerItem } from '@/components/page-transition'
import { getUserQuotaDates } from '@/features/dashboard/api'
import { useSummaryCardsConfig } from '@/features/dashboard/hooks/use-dashboard-config'
import type { QuotaDataItem } from '@/features/dashboard/types'
import { useStatus } from '@/hooks/use-status'
import { formatNumber } from '@/lib/format'
import { computeTimeRange } from '@/lib/time'
import { useAuthStore } from '@/stores/auth-store'

import { StatCard } from '../ui/stat-card'

const SUMMARY_SPARKLINE_BUCKETS = 12

function getBucketIndex(
  timestamp: number,
  start: number,
  end: number,
  bucketCount: number
): number {
  if (end <= start) return 0
  const ratio = (timestamp - start) / (end - start)
  return Math.min(bucketCount - 1, Math.max(0, Math.floor(ratio * bucketCount)))
}

function buildRequestsSparkline(
  data: QuotaDataItem[],
  start: number,
  end: number
): number[] {
  const requests = Array.from({ length: SUMMARY_SPARKLINE_BUCKETS }, () => 0)
  for (const item of data) {
    const timestamp = Number(item.created_at) || start
    const index = getBucketIndex(
      timestamp,
      start,
      end,
      SUMMARY_SPARKLINE_BUCKETS
    )
    requests[index] += Number(item.count) || 0
  }
  return requests
}

export function SummaryCards() {
  const { t } = useTranslation()
  const user = useAuthStore((state) => state.auth.user)
  const { loading } = useStatus()

  const summaryTimeRange = useMemo(() => computeTimeRange(1), [])
  const requestCount = Number(user?.request_count ?? 0)

  const usageTrendQuery = useQuery({
    queryKey: [
      'dashboard',
      'overview',
      'summary-sparklines',
      summaryTimeRange.start_timestamp,
      summaryTimeRange.end_timestamp,
    ],
    queryFn: async () =>
      getUserQuotaDates({
        start_timestamp: summaryTimeRange.start_timestamp,
        end_timestamp: summaryTimeRange.end_timestamp,
        default_time: 'hour',
      }),
    staleTime: 60 * 1000,
  })

  const requestCountDisplay = useMemo(
    () => formatNumber(requestCount),
    [requestCount]
  )

  const sparklineData = useMemo(
    () =>
      buildRequestsSparkline(
        usageTrendQuery.data?.data ?? [],
        summaryTimeRange.start_timestamp,
        summaryTimeRange.end_timestamp
      ),
    [
      summaryTimeRange.end_timestamp,
      summaryTimeRange.start_timestamp,
      usageTrendQuery.data?.data,
    ]
  )

  const items = useSummaryCardsConfig({
    requestCountDisplay,
  }).map((config) => {
    return {
      key: config.key,
      title: config.title,
      value: config.value,
      desc: config.description,
      icon: config.icon,
      tone: 'accent-1' as const,
      sparkline: sparklineData,
      sparklineVariant: 'line' as const,
    }
  })

  return (
    <div className='bg-card overflow-hidden rounded-2xl border shadow-xs'>
      <div className='p-3 sm:p-5'>
        <div className='flex flex-wrap items-start justify-between gap-3'>
          <div className='flex flex-col gap-1'>
            <h3 className='text-sm font-semibold sm:text-base'>
              {t('Usage at a glance')}
            </h3>
            <p className='text-muted-foreground text-xs sm:text-sm'>
              {t('Monitor request volume')}
            </p>
          </div>
        </div>
        <StaggerContainer className='grid grid-cols-3 gap-1.5 sm:gap-3'>
          {items.map((it) => (
            <StaggerItem
              key={it.key}
              className='bg-background/60 col-span-3 rounded-lg border px-2 py-1.5 sm:col-span-1 sm:rounded-xl sm:p-3'
            >
              <StatCard
                title={it.title}
                value={it.value}
                description={it.desc}
                icon={it.icon}
                tone={it.tone}
                sparkline={it.sparkline}
                sparklineVariant={it.sparklineVariant}
                loading={loading}
                compactMobile
              />
            </StaggerItem>
          ))}
        </StaggerContainer>
      </div>
    </div>
  )
}