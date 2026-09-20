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
import { AlertTriangle, Route } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { StatusBadge } from '@/components/status-badge'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'
import { getLobeIcon } from '@/lib/lobe-icon'
import { cn } from '@/lib/utils'

import { isResponseModelMismatch } from '../lib/response-model'
import type { LogOtherData } from '../types'

interface ModelBadgeProps {
  modelName: string
  actualModel?: string
  responseModel?: LogOtherData['response_model']
  className?: string
}

interface ModelProvider {
  icon: string
  label: string
}

function resolveModelProvider(modelName: string): ModelProvider | null {
  const model = modelName.toLowerCase()
  const hasAny = (keywords: string[]) =>
    keywords.some((keyword) => model.includes(keyword))

  if (
    hasAny([
      'gpt-',
      'chatgpt-',
      'text-embedding-',
      'omni-moderation',
      'dall-e',
      'whisper',
      'tts-',
    ]) ||
    /\bo[134](?:-|$)/.test(model)
  ) {
    return { icon: 'OpenAI.Color', label: 'OpenAI' }
  }
  if (hasAny(['claude-', 'anthropic'])) {
    return { icon: 'Claude.Color', label: 'Claude' }
  }
  if (hasAny(['gemini-', 'learnlm-'])) {
    return { icon: 'Gemini.Color', label: 'Gemini' }
  }
  if (hasAny(['grok-', 'xai-'])) {
    return { icon: 'Grok.Color', label: 'Grok' }
  }
  if (hasAny(['deepseek-'])) {
    return { icon: 'DeepSeek.Color', label: 'DeepSeek' }
  }
  if (hasAny(['qwen', 'qwq-'])) {
    return { icon: 'Qwen.Color', label: 'Qwen' }
  }
  if (hasAny(['doubao-', 'volcengine'])) {
    return { icon: 'Doubao.Color', label: 'Doubao' }
  }
  if (hasAny(['moonshot-', 'kimi-'])) {
    return { icon: 'Moonshot.Color', label: 'Moonshot' }
  }
  if (hasAny(['minimax', 'abab'])) {
    return { icon: 'Minimax.Color', label: 'MiniMax' }
  }
  if (hasAny(['glm-', 'chatglm', 'cogview', 'cogvideo'])) {
    return { icon: 'Zhipu.Color', label: 'Zhipu' }
  }
  if (hasAny(['mimo-'])) {
    return { icon: 'XiaomiMiMo', label: 'MiMo' }
  }
  if (hasAny(['ernie'])) {
    return { icon: 'Wenxin.Color', label: 'Baidu' }
  }
  if (hasAny(['spark'])) {
    return { icon: 'Spark.Color', label: 'iFlyTek' }
  }
  if (hasAny(['hunyuan'])) {
    return { icon: 'Hunyuan.Color', label: 'Tencent' }
  }
  if (hasAny(['baichuan'])) {
    return { icon: 'Baichuan.Color', label: 'Baichuan' }
  }
  if (hasAny(['internlm'])) {
    return { icon: 'InternLM.Color', label: 'InternLM' }
  }
  if (hasAny(['step-'])) {
    return { icon: 'Stepfun.Color', label: 'StepFun' }
  }
  if (hasAny(['yi-'])) {
    return { icon: 'Yi.Color', label: 'Yi' }
  }
  if (hasAny(['mistral-', 'mixtral-'])) {
    return { icon: 'Mistral.Color', label: 'Mistral' }
  }
  if (hasAny(['llama-', 'meta-'])) {
    return { icon: 'Meta.Color', label: 'Meta' }
  }
  if (hasAny(['command-', 'cohere-'])) {
    return { icon: 'Cohere.Color', label: 'Cohere' }
  }

  return null
}

function ModelBadgeContent(props: ModelBadgeProps) {
  const provider = resolveModelProvider(props.modelName)

  return (
    <StatusBadge
      copyText={props.modelName}
      size='sm'
      showDot={!provider}
      autoColor={provider ? undefined : props.modelName}
      className={cn(
        'border-border/60 bg-muted/30 h-6 max-w-none gap-1.5 rounded-md border px-2 [font-family:var(--font-body)]',
        provider && 'text-foreground',
        props.className
      )}
    >
      <span className='flex max-w-none items-center gap-1.5'>
        {provider && (
          <span
            className='flex h-[18px] w-[18px] shrink-0 items-center justify-center'
            title={provider.label}
            aria-label={provider.label}
          >
            {getLobeIcon(provider.icon, 18)}
          </span>
        )}
        <span className='whitespace-nowrap'>{props.modelName}</span>
      </span>
    </StatusBadge>
  )
}

export function ModelBadge(props: ModelBadgeProps) {
  const { t } = useTranslation()
  const mismatch = isResponseModelMismatch(props.responseModel)
  const hasResponseDetails = !!(
    props.responseModel?.returned_model &&
    (mismatch ||
      props.responseModel.returned_model !== props.responseModel.requested_model ||
      props.responseModel.upstream_model !== props.responseModel.requested_model)
  )

  if (!props.actualModel && !hasResponseDetails) {
    return <ModelBadgeContent {...props} />
  }

  const content = (
    <>
      <ModelBadgeContent {...props} />
      {mismatch && (
        <StatusBadge
          icon={AlertTriangle}
          label={t('Response model mismatch')}
          variant='warning'
          copyable={false}
        />
      )}
      {!mismatch && props.actualModel && (
        <Route
          className='text-muted-foreground size-3 shrink-0'
          aria-hidden='true'
        />
      )}
    </>
  )

  return (
    <Popover>
      <PopoverTrigger
        render={
          <button type='button' className='inline-flex items-center gap-1' />
        }
      >
        {content}
      </PopoverTrigger>
      <PopoverContent className='w-72'>
        {props.responseModel?.returned_model ? (
          <ResponseModelDetails observation={props.responseModel} />
        ) : (
          <ModelMappingDetails
            requestModel={props.modelName}
            actualModel={props.actualModel}
          />
        )}
      </PopoverContent>
    </Popover>
  )
}

function ModelMappingDetails(props: {
  requestModel: string
  actualModel?: string
}) {
  const { t } = useTranslation()

  return (
    <div className='space-y-2'>
      <ModelDetailRow label={t('Request Model:')} value={props.requestModel} />
      <ModelDetailRow label={t('Actual Model:')} value={props.actualModel} />
    </div>
  )
}

export function ResponseModelDetails(props: {
  observation: NonNullable<LogOtherData['response_model']>
}) {
  const { t } = useTranslation()
  const mismatch = isResponseModelMismatch(props.observation)

  return (
    <div className='space-y-2'>
      {mismatch && (
        <StatusBadge
          icon={AlertTriangle}
          label={t('Response model: {{model}}', {
            model: props.observation.returned_model,
          })}
          variant='warning'
          copyable={false}
          className='h-auto whitespace-normal'
        />
      )}
      <ModelDetailRow
        label={t('Request Model:')}
        value={props.observation.requested_model}
      />
      <ModelDetailRow
        label={t('Upstream Model:')}
        value={props.observation.upstream_model}
      />
      <ModelDetailRow
        label={t('Response Model:')}
        value={props.observation.returned_model}
      />
    </div>
  )
}

function ModelDetailRow(props: { label: string; value?: string }) {
  return (
    <div className='flex items-start justify-between gap-3'>
      <span className='text-muted-foreground text-xs'>{props.label}</span>
      <span className='truncate font-mono text-xs font-medium'>
        {props.value || '-'}
      </span>
    </div>
  )
}
