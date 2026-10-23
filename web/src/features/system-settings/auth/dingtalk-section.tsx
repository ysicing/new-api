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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { CopyButton } from '@/components/copy-button'
import { Button } from '@/components/ui/button'
import {
  Field,
  FieldDescription,
  FieldGroup,
  FieldLabel,
} from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Spinner } from '@/components/ui/spinner'
import { Switch } from '@/components/ui/switch'

import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'
import {
  sendDingTalkAnnouncementGroupTestMessage,
  sendDingTalkTestMessage,
  type DingTalkTestUser,
} from './dingtalk-api'
import { DingTalkTestRecipientPicker } from './dingtalk-test-recipient-picker'
import { buildOAuthCallbackUrl } from './oauth-callback-url'

// DingTalkSettingsValues 对应后端 dingtalk.* 配置键。
export interface DingTalkSettingsValues {
  'dingtalk.enabled': boolean
  'dingtalk.corp_id': string
  'dingtalk.client_id': string
  'dingtalk.client_secret': string
  'dingtalk.announcement_group_open_conversation_id': string
}

type DingTalkSectionProps = {
  defaultValues: DingTalkSettingsValues
  serverAddress: string
}

// DingTalkSection 配置企业内部应用扫码登录及其固定回调地址。
export function DingTalkSection(props: DingTalkSectionProps) {
  const { t } = useTranslation()
  const update = useUpdateOption()
  const [values, setValues] = useState(props.defaultValues)
  const [testRecipient, setTestRecipient] = useState<DingTalkTestUser | null>(
    null
  )
  const [isTesting, setIsTesting] = useState(false)
  const [isGroupTesting, setIsGroupTesting] = useState(false)
  const callbackUrl = buildOAuthCallbackUrl(
    props.serverAddress,
    'dingtalk',
    t('Site URL')
  )
  const set = <K extends keyof DingTalkSettingsValues>(
    key: K,
    value: DingTalkSettingsValues[K]
  ) => setValues((current) => ({ ...current, [key]: value }))

  const save = async () => {
    const entries = Object.entries(values).sort(([left], [right]) => {
      if (left === 'dingtalk.enabled') return 1
      if (right === 'dingtalk.enabled') return -1
      return 0
    })
    for (const [key, value] of entries) {
      if (key === 'dingtalk.client_secret' && value === '') continue
      if (value !== props.defaultValues[key as keyof DingTalkSettingsValues]) {
        const response = await update.mutateAsync({ key, value })
        if (!response.success) return false
      }
    }
    return true
  }

  const saveAndTest = async () => {
    if (!testRecipient) {
      toast.error(t('Select a DingTalk test recipient first'))
      return
    }
    setIsTesting(true)
    try {
      if (!(await save())) return
      const response = await sendDingTalkTestMessage(testRecipient.id)
      if (!response.success) {
        throw new Error(
          response.message || t('Failed to send DingTalk test message')
        )
      }
      toast.success(t('DingTalk test message sent successfully'))
      if (response.data?.bound_now) {
        toast.success(t('DingTalk account automatically bound by email'))
        setTestRecipient({ ...testRecipient, dingtalk_bound: true })
      }
    } catch (error) {
      const message =
        error instanceof Error && error.message
          ? error.message
          : t('Failed to send DingTalk test message')
      toast.error(message)
    } finally {
      setIsTesting(false)
    }
  }

  const saveAndTestGroup = async () => {
    setIsGroupTesting(true)
    try {
      if (!(await save())) return
      const response = await sendDingTalkAnnouncementGroupTestMessage()
      if (!response.success) {
        throw new Error(
          response.message || t('Failed to send DingTalk group test message')
        )
      }
      toast.success(t('DingTalk group test message sent successfully'))
    } catch (error) {
      toast.error(
        error instanceof Error && error.message
          ? error.message
          : t('Failed to send DingTalk group test message')
      )
    } finally {
      setIsGroupTesting(false)
    }
  }

  return (
    <SettingsSection title={t('DingTalk Authentication')}>
      <FieldGroup>
        <Field orientation='horizontal'>
          <div className='flex-1'>
            <FieldLabel htmlFor='dingtalk-enabled'>
              {t('DingTalk login')}
            </FieldLabel>
            <FieldDescription>
              {t('Allow employees to sign in by scanning with DingTalk.')}
            </FieldDescription>
          </div>
          <Switch
            id='dingtalk-enabled'
            checked={values['dingtalk.enabled']}
            onCheckedChange={(value) => set('dingtalk.enabled', value)}
          />
        </Field>
        <Field>
          <FieldLabel htmlFor='dingtalk-corp-id'>{t('Corp ID')}</FieldLabel>
          <Input
            id='dingtalk-corp-id'
            value={values['dingtalk.corp_id']}
            onChange={(event) => set('dingtalk.corp_id', event.target.value)}
          />
        </Field>
        <Field>
          <FieldLabel htmlFor='dingtalk-client-id'>
            {t('Client ID / AppKey')}
          </FieldLabel>
          <Input
            id='dingtalk-client-id'
            value={values['dingtalk.client_id']}
            onChange={(event) => set('dingtalk.client_id', event.target.value)}
          />
        </Field>
        <Field>
          <FieldLabel htmlFor='dingtalk-client-secret'>
            {t('Client Secret / AppSecret')}
          </FieldLabel>
          <Input
            id='dingtalk-client-secret'
            type='password'
            autoComplete='new-password'
            value={values['dingtalk.client_secret']}
            onChange={(event) =>
              set('dingtalk.client_secret', event.target.value)
            }
          />
          <FieldDescription>
            {t('Leave blank unless rotating the secret')}
          </FieldDescription>
        </Field>
        <Field>
          <FieldLabel htmlFor='dingtalk-announcement-group'>
            {t('Announcement group openConversationId')}
          </FieldLabel>
          <Input
            id='dingtalk-announcement-group'
            value={values['dingtalk.announcement_group_open_conversation_id']}
            onChange={(event) =>
              set(
                'dingtalk.announcement_group_open_conversation_id',
                event.target.value
              )
            }
          />
          <FieldDescription>
            {t(
              'The internal DingTalk group that receives system announcements.'
            )}
          </FieldDescription>
        </Field>
        <Field>
          <FieldLabel>{t('OAuth callback URL')}</FieldLabel>
          <div className='flex items-center gap-2'>
            <code className='bg-muted min-w-0 flex-1 rounded px-2 py-1 text-xs break-all'>
              {callbackUrl}
            </code>
            <CopyButton
              value={callbackUrl}
              size='icon'
              aria-label={t('Copy callback URL')}
              tooltip={t('Copy callback URL')}
            />
          </div>
        </Field>
        <Field>
          <FieldLabel htmlFor='dingtalk-test-recipient'>
            {t('DingTalk test recipient')}
          </FieldLabel>
          <DingTalkTestRecipientPicker
            value={testRecipient}
            onValueChange={setTestRecipient}
            disabled={isTesting}
          />
          <FieldDescription>
            {t(
              'Select a platform user to verify personal Bot delivery and bind DingTalk by email when needed.'
            )}
          </FieldDescription>
        </Field>
        <div className='flex flex-wrap gap-2'>
          <Button
            className='w-fit'
            disabled={update.isPending || isTesting}
            onClick={() => void save()}
          >
            {update.isPending && <Spinner data-icon='inline-start' />}
            {t('Save')}
          </Button>
          <Button
            className='w-fit'
            variant='outline'
            disabled={
              values[
                'dingtalk.announcement_group_open_conversation_id'
              ].trim() === '' ||
              update.isPending ||
              isGroupTesting
            }
            onClick={() => void saveAndTestGroup()}
          >
            {isGroupTesting && <Spinner data-icon='inline-start' />}
            {t('Save and send group test message')}
          </Button>
          <Button
            className='w-fit'
            variant='outline'
            disabled={!testRecipient || update.isPending || isTesting}
            onClick={() => void saveAndTest()}
          >
            {isTesting && <Spinner data-icon='inline-start' />}
            {t('Save and send test message')}
          </Button>
        </div>
      </FieldGroup>
    </SettingsSection>
  )
}
