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
import { zodResolver } from '@hookform/resolvers/zod'
import { useEffect } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import * as z from 'zod'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'

import {
  MAX_RATE_LIMIT_VALUE,
  hasUserModelRateLimitRule,
  type UserModelRateLimitRule,
} from './user-model-rate-limit'

const USER_MODEL_RATE_LIMIT_FORM_ID = 'user-model-rate-limit-form'

const createUserModelRateLimitSchema = (t: (key: string) => string) =>
  z
    .object({
      user_id: z
        .number()
        .int(t('User ID must be a positive integer'))
        .min(1, t('User ID must be a positive integer'))
        .max(MAX_RATE_LIMIT_VALUE, t('Must be ≤ 2,147,483,647')),
      model: z.string().trim().min(1, t('Model is required')),
      max_requests: z
        .number()
        .int(t('Must be a non-negative integer'))
        .min(0, t('Must be a non-negative integer'))
        .max(MAX_RATE_LIMIT_VALUE, t('Must be ≤ 2,147,483,647')),
      max_success: z
        .number()
        .int(t('Must be a non-negative integer'))
        .min(0, t('Must be a non-negative integer'))
        .max(MAX_RATE_LIMIT_VALUE, t('Must be ≤ 2,147,483,647')),
    })
    .refine((rule) => rule.max_requests > 0 || rule.max_success > 0, {
      message: t('At least one limit must be greater than 0'),
      path: ['max_success'],
    })

type UserModelRateLimitDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  onSave: (rule: UserModelRateLimitRule) => void
  rules: UserModelRateLimitRule[]
  editData?: UserModelRateLimitRule | null
}

export function UserModelRateLimitDialog(props: UserModelRateLimitDialogProps) {
  const { t } = useTranslation()
  const schema = createUserModelRateLimitSchema(t)
  const isEditMode = !!props.editData

  const form = useForm<UserModelRateLimitRule>({
    resolver: zodResolver(schema),
    defaultValues: {
      user_id: 0,
      model: '',
      max_requests: 0,
      max_success: 1,
    },
  })

  useEffect(() => {
    form.reset(
      props.editData ?? {
        user_id: 0,
        model: '',
        max_requests: 0,
        max_success: 1,
      }
    )
  }, [form, props.editData, props.open])

  const handleSubmit = (rule: UserModelRateLimitRule) => {
    if (
      hasUserModelRateLimitRule(
        props.rules,
        rule.user_id,
        rule.model,
        props.editData
      )
    ) {
      form.setError('model', {
        message: t('A rule for this user and model already exists'),
      })
      return
    }

    props.onSave(rule)
    form.reset()
    props.onOpenChange(false)
  }

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={
        isEditMode
          ? t('Edit user-model rate limit')
          : t('Add user-model rate limit')
      }
      description={t(
        'Configure rate limiting for one user and one exact requested model.'
      )}
      contentClassName='sm:max-w-[540px]'
      contentHeight='auto'
      bodyClassName='space-y-4'
      footer={
        <>
          <Button
            type='button'
            variant='outline'
            onClick={() => props.onOpenChange(false)}
          >
            {t('Cancel')}
          </Button>
          <Button type='submit' form={USER_MODEL_RATE_LIMIT_FORM_ID}>
            {isEditMode ? t('Update') : t('Add')}
          </Button>
        </>
      }
    >
      <Form {...form}>
        <form
          id={USER_MODEL_RATE_LIMIT_FORM_ID}
          onSubmit={form.handleSubmit(handleSubmit)}
          className='space-y-4'
        >
          <div className='grid gap-4 sm:grid-cols-2'>
            <FormField
              control={form.control}
              name='user_id'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('User ID')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min={1}
                      max={MAX_RATE_LIMIT_VALUE}
                      step={1}
                      {...field}
                      value={field.value || ''}
                      onChange={(event) =>
                        field.onChange(event.target.valueAsNumber || 0)
                      }
                    />
                  </FormControl>
                  <FormDescription>
                    {t('The limit applies to all API keys owned by this user.')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='model'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Model')}</FormLabel>
                  <FormControl>
                    <Input placeholder='gpt-5.4' {...field} />
                  </FormControl>
                  <FormDescription>
                    {t('Exact requested model name before channel mapping.')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>

          <div className='grid gap-4 sm:grid-cols-2'>
            <FormField
              control={form.control}
              name='max_requests'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>
                    {t('Max Requests (including failures)')}
                  </FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min={0}
                      max={MAX_RATE_LIMIT_VALUE}
                      step={1}
                      {...field}
                      onChange={(event) =>
                        field.onChange(event.target.valueAsNumber || 0)
                      }
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Total requests allowed per period. 0 = unlimited.')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='max_success'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Max Successful Requests')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min={0}
                      max={MAX_RATE_LIMIT_VALUE}
                      step={1}
                      {...field}
                      onChange={(event) =>
                        field.onChange(event.target.valueAsNumber || 0)
                      }
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Only successful requests count toward this limit. 0 = unlimited.'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>
        </form>
      </Form>
    </Dialog>
  )
}
