import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Spinner } from '@/components/ui/spinner'
import { Switch } from '@/components/ui/switch'

import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

export interface LDAPSettingsValues {
  'ldap.enabled': boolean
  'ldap.ldap_url': string
  'ldap.ldap_search_dn': string
  'ldap.ldap_search_password': string
  'ldap.ldap_base_dn': string
  'ldap.ldap_filter': string
  'ldap.ldap_uid': string
  'ldap.ldap_scope': number
  'ldap.ldap_connection_timeout': number
}

export function LDAPSection({
  defaultValues,
}: {
  defaultValues: LDAPSettingsValues
}) {
  const { t } = useTranslation()
  const update = useUpdateOption()
  const [values, setValues] = useState(defaultValues)
  const set = <K extends keyof LDAPSettingsValues>(
    key: K,
    value: LDAPSettingsValues[K]
  ) => setValues((current) => ({ ...current, [key]: value }))
  const save = async () => {
    const entries = Object.entries(values).sort(([left], [right]) => {
      if (left === 'ldap.enabled') return 1
      if (right === 'ldap.enabled') return -1
      return 0
    })
    for (const [key, value] of entries) {
      if (key === 'ldap.ldap_search_password' && value === '') continue
      if (value !== defaultValues[key as keyof LDAPSettingsValues]) {
        await update.mutateAsync({ key, value })
      }
    }
  }
  return (
    <SettingsSection title={t('LDAP Authentication')}>
      <FieldGroup>
        <Field orientation='horizontal'>
          <div className='flex-1'>
            <FieldLabel htmlFor='ldap-enabled'>{t('LDAP login')}</FieldLabel>
          </div>
          <Switch
            id='ldap-enabled'
            checked={values['ldap.enabled']}
            onCheckedChange={(value) => set('ldap.enabled', value)}
          />
        </Field>
        <Field>
          <FieldLabel htmlFor='ldap-url'>{t('LDAP URL')}</FieldLabel>
          <Input
            id='ldap-url'
            value={values['ldap.ldap_url']}
            onChange={(event) => set('ldap.ldap_url', event.target.value)}
            placeholder='ldap://ldap.example.com:389'
          />
        </Field>
        <Field>
          <FieldLabel htmlFor='ldap-base-dn'>{t('Base DN')}</FieldLabel>
          <Input
            id='ldap-base-dn'
            value={values['ldap.ldap_base_dn']}
            onChange={(event) => set('ldap.ldap_base_dn', event.target.value)}
          />
        </Field>
        <Field>
          <FieldLabel htmlFor='ldap-search-dn'>{t('Search DN')}</FieldLabel>
          <Input
            id='ldap-search-dn'
            value={values['ldap.ldap_search_dn']}
            onChange={(event) => set('ldap.ldap_search_dn', event.target.value)}
          />
        </Field>
        <Field>
          <FieldLabel htmlFor='ldap-search-password'>
            {t('Search password')}
          </FieldLabel>
          <Input
            id='ldap-search-password'
            type='password'
            autoComplete='new-password'
            value={values['ldap.ldap_search_password']}
            onChange={(event) =>
              set('ldap.ldap_search_password', event.target.value)
            }
          />
        </Field>
        <Field>
          <FieldLabel htmlFor='ldap-uid'>{t('User attribute')}</FieldLabel>
          <Input
            id='ldap-uid'
            value={values['ldap.ldap_uid']}
            onChange={(event) => set('ldap.ldap_uid', event.target.value)}
          />
        </Field>
        <Field>
          <FieldLabel htmlFor='ldap-filter'>{t('LDAP filter')}</FieldLabel>
          <Input
            id='ldap-filter'
            value={values['ldap.ldap_filter']}
            onChange={(event) => set('ldap.ldap_filter', event.target.value)}
          />
        </Field>
        <Button
          className='w-fit'
          disabled={update.isPending}
          onClick={() => void save()}
        >
          {update.isPending && <Spinner data-icon='inline-start' />}
          {t('Save')}
        </Button>
      </FieldGroup>
    </SettingsSection>
  )
}
