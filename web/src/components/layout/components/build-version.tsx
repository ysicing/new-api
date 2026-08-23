/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { useTranslation } from 'react-i18next'

import { SidebarFooter } from '@/components/ui/sidebar'
import { cn } from '@/lib/utils'

export function BuildVersion(props: {
  className?: string
  version?: string | null
}) {
  const { t } = useTranslation()
  const version =
    props.version === undefined
      ? import.meta.env.VITE_REACT_APP_VERSION?.trim()
      : props.version?.trim()
  if (!version) return null

  return (
    <span className={cn('text-muted-foreground/45 text-xs', props.className)}>
      {t('Build {{version}}', { version })}
    </span>
  )
}

export function SidebarBuildVersion() {
  const version = import.meta.env.VITE_REACT_APP_VERSION?.trim()
  if (!version) return null

  return (
    <SidebarFooter className='border-sidebar-border/70 border-t px-3 py-2 group-data-[collapsible=icon]:hidden'>
      <BuildVersion
        version={version}
        className='text-sidebar-foreground/40 truncate text-[10px]'
      />
    </SidebarFooter>
  )
}
