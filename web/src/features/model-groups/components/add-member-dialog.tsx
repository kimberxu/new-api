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
import { Search, X } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  ToggleGroup,
  ToggleGroupItem,
} from '@/components/ui/toggle-group'
import { getChannels } from '@/features/channels/api'
import { parseModelsList } from '@/features/channels/lib'
import type { Channel } from '@/features/channels/types'

import type { ModelGroup } from '../lib/api'

interface AddMemberDialogProps {
  group: ModelGroup
  groups: ModelGroup[]
  submittingMembers: boolean
  submittingReference: boolean
  onClose: () => void
  onSubmitMembers: (values: string[]) => Promise<string[]>
  onSubmitReference: (refGroupId: number) => void
}

export function AddMemberDialog(props: AddMemberDialogProps) {
  const { t } = useTranslation()
  const [mode, setMode] = useState<'channel' | 'group'>('channel')
  const [memberSearch, setMemberSearch] = useState('')
  const [addSelected, setAddSelected] = useState<Set<string>>(new Set())
  const [addRefGroupId, setAddRefGroupId] = useState('')

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
  })

  const memberOptions = useMemo(() => {
    const options: { value: string; model: string; channelLabel: string }[] =
      []
    for (const ch of channelsData ?? []) {
      const name = ch.name || `#${ch.id}`
      for (const m of parseModelsList(ch.models || '')) {
        options.push({
          value: `${ch.id}|${m}`,
          model: m,
          channelLabel: `${name} (#${ch.id})`,
        })
      }
    }
    return options
  }, [channelsData])

  const optionMap = useMemo(
    () => new Map(memberOptions.map((o) => [o.value, o])),
    [memberOptions]
  )

  const visibleMemberOptions = useMemo(() => {
    const kw = memberSearch.trim().toLowerCase()
    if (!kw) return memberOptions
    return memberOptions.filter((o) =>
      `${o.model} ${o.channelLabel}`.toLowerCase().includes(kw)
    )
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

  const submitMembers = async () => {
    try {
      const failed = await props.onSubmitMembers([...addSelected])
      if (failed.length === 0) {
        setAddSelected(new Set())
        props.onClose()
      } else {
        setAddSelected(new Set(failed))
      }
    } catch {
      /* 页面 mutation onError 已 toast */
    }
  }

  return (
    <Dialog
      open
      onOpenChange={(open) => !open && props.onClose()}
      title={`${t('Add Member')} — ${props.group.name}`}
      description={t(
        'Add real upstream models from any channel. Priority/weight empty = inherit the channel values.'
      )}
      footer={
        mode === 'channel' ? (
          <Button
            disabled={addSelected.size === 0 || props.submittingMembers}
            onClick={submitMembers}
          >
            {t('Add')}
          </Button>
        ) : (
          <Button
            disabled={!addRefGroupId || props.submittingReference}
            onClick={() => props.onSubmitReference(Number(addRefGroupId))}
          >
            {t('Add')}
          </Button>
        )
      }
    >
      <div className='space-y-3'>
        <ToggleGroup
          value={[mode]}
          variant='outline'
          size='sm'
          onValueChange={(vals) => {
            const next = vals.find((v) => v !== mode)
            if (next) setMode(next as 'channel' | 'group')
          }}
        >
          <ToggleGroupItem value='channel'>
            {t('Channel model')}
          </ToggleGroupItem>
          <ToggleGroupItem value='group'>
            {t('Reference group')}
          </ToggleGroupItem>
        </ToggleGroup>
        {mode === 'group' ? (
          <div>
            <label className='text-muted-foreground mb-1 block text-sm'>
              {t('Referenced group')}
            </label>
            <Select
              value={addRefGroupId}
              onValueChange={(value) => {
                if (value != null) setAddRefGroupId(value)
              }}
            >
              <SelectTrigger className='w-full'>
                <SelectValue
                  placeholder={t(
                    'Select a group to include all its members'
                  )}
                />
              </SelectTrigger>
              <SelectContent>
                {props.groups
                  .filter(
                    (g) =>
                      g.id !== props.group.id &&
                      !(g.references ?? []).some(
                        (ref) => ref.ref_group_id === props.group.id
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
              {t(
                'Members are aggregated and updated live; duplicates with the direct members are merged.'
              )}
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
            {addSelected.size > 0 && (
              <div className='flex flex-wrap gap-1'>
                {[...addSelected].map((v) => {
                  const opt = optionMap.get(v)
                  return (
                    <span
                      key={v}
                      className='bg-secondary text-secondary-foreground flex items-center gap-1 rounded-full px-2 py-0.5 text-xs'
                    >
                      <span className='font-mono'>{opt?.model ?? v}</span>
                      <button
                        type='button'
                        aria-label={t('Remove')}
                        onClick={() => toggleMemberOption(v, false)}
                      >
                        <X className='h-3 w-3' />
                      </button>
                    </span>
                  )
                })}
              </div>
            )}
            <div className='border-border max-h-72 overflow-y-auto rounded-md border p-1'>
              {visibleMemberOptions.length === 0 ? (
                <div className='text-muted-foreground py-6 text-center text-sm'>
                  {t('No matching channel model.')}
                </div>
              ) : (
                visibleMemberOptions.map((option) => (
                  <div
                    key={option.value}
                    className='hover:bg-accent/60 flex items-center gap-2 rounded-md px-2 py-1.5'
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
                      className='min-w-0 flex-1 cursor-pointer font-normal'
                    >
                      <span className='block truncate font-mono text-xs font-medium'>
                        {option.model}
                      </span>
                    </Label>
                    <span className='ml-auto shrink-0 pl-2 text-muted-foreground text-xs'>
                      {option.channelLabel}
                    </span>
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
  )
}
