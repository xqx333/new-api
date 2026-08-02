/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { Plus, Search } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { StaticDataTable } from '@/components/data-table/static/static-data-table'
import { StaticRowActions } from '@/components/data-table/static/static-row-actions'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

import {
  parseUserModelRateLimitRules,
  removeUserModelRateLimitRule,
  serializeUserModelRateLimitRules,
  upsertUserModelRateLimitRule,
  type UserModelRateLimitRule,
} from './user-model-rate-limit'
import { UserModelRateLimitDialog } from './user-model-rate-limit-dialog'

type UserModelRateLimitVisualEditorProps = {
  value: string
  onChange: (value: string) => void
}

export function UserModelRateLimitVisualEditor(
  props: UserModelRateLimitVisualEditorProps
) {
  const { t } = useTranslation()
  const [searchText, setSearchText] = useState('')
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editData, setEditData] = useState<UserModelRateLimitRule | null>(null)

  const rules = useMemo(
    () => parseUserModelRateLimitRules(props.value),
    [props.value]
  )
  const filteredRules = useMemo(() => {
    const normalizedSearch = searchText.trim().toLowerCase()
    if (!normalizedSearch) return rules

    return rules.filter(
      (rule) =>
        String(rule.user_id).includes(normalizedSearch) ||
        rule.model.toLowerCase().includes(normalizedSearch)
    )
  }, [rules, searchText])

  const handleSave = (rule: UserModelRateLimitRule) => {
    props.onChange(
      serializeUserModelRateLimitRules(
        upsertUserModelRateLimitRule(rules, rule, editData)
      )
    )
  }

  const handleDelete = (rule: UserModelRateLimitRule) => {
    props.onChange(
      serializeUserModelRateLimitRules(
        removeUserModelRateLimitRule(rules, rule.user_id, rule.model)
      )
    )
  }

  const handleEdit = (rule: UserModelRateLimitRule) => {
    setEditData(rule)
    setDialogOpen(true)
  }

  const handleAdd = () => {
    setEditData(null)
    setDialogOpen(true)
  }

  return (
    <div className='space-y-4'>
      <div className='flex items-center gap-4'>
        <div className='relative flex-1'>
          <Search className='text-muted-foreground absolute top-2.5 left-2.5 h-4 w-4' />
          <Input
            placeholder={t('Search user IDs or models...')}
            value={searchText}
            onChange={(event) => setSearchText(event.target.value)}
            className='pl-9'
          />
        </div>
        <Button type='button' onClick={handleAdd}>
          <Plus className='mr-2 h-4 w-4' />
          {t('Add rule')}
        </Button>
      </div>

      <StaticDataTable
        data={filteredRules}
        getRowKey={(rule) => `${rule.user_id}:${rule.model}`}
        emptyContent={
          searchText
            ? t('No user-model rules match your search')
            : t(
                'No user-model rate limits configured. Click "Add rule" to get started.'
              )
        }
        columns={[
          {
            id: 'user-id',
            header: t('User ID'),
            cellClassName: 'font-mono font-medium',
            cell: (rule) => rule.user_id,
          },
          {
            id: 'model',
            header: t('Model'),
            cellClassName: 'font-mono',
            cell: (rule) => rule.model,
          },
          {
            id: 'max-requests',
            header: t('Max Requests (incl. failures)'),
            className: 'text-right',
            cellClassName: 'text-right',
            cell: (rule) => (
              <span className='font-mono'>
                {rule.max_requests === 0
                  ? t('Unlimited')
                  : rule.max_requests.toLocaleString()}
              </span>
            ),
          },
          {
            id: 'max-success',
            header: t('Max Success'),
            className: 'text-right',
            cellClassName: 'text-right',
            cell: (rule) => (
              <span className='font-mono'>
                {rule.max_success === 0
                  ? t('Unlimited')
                  : rule.max_success.toLocaleString()}
              </span>
            ),
          },
          {
            id: 'actions',
            header: t('Actions'),
            className: 'text-right',
            cellClassName: 'text-right',
            cell: (rule) => (
              <StaticRowActions
                editLabel={t('Edit')}
                deleteLabel={t('Delete')}
                menuLabel={t('Open menu')}
                onEdit={() => handleEdit(rule)}
                onDelete={() => handleDelete(rule)}
              />
            ),
          },
        ]}
      />

      <UserModelRateLimitDialog
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        onSave={handleSave}
        rules={rules}
        editData={editData}
      />
    </div>
  )
}
