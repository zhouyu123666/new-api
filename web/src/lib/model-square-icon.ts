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
*/

const COLOR_ICON_BASES = new Set([
  'Ai360',
  'Anthropic',
  'AzureAI',
  'Baidu',
  'Claude',
  'Cloudflare',
  'Cohere',
  'DeepSeek',
  'Doubao',
  'Gemini',
  'Hunyuan',
  'Jimeng',
  'Kling',
  'Minimax',
  'Mistral',
  'Moonshot',
  'Qwen',
  'Spark',
  'Vidu',
  'Wenxin',
  'Yi',
  'Zhipu',
])

export function getModelSquareIconName(
  iconName: string | undefined | null,
  modelName?: string
): string | undefined {
  if (!iconName?.trim()) return undefined

  const trimmedName = iconName.trim()
  const segments = trimmedName.split('.')
  const baseKey = segments[0]

  if (segments.length > 1 || baseKey === 'Sub2API') return trimmedName

  if (
    baseKey === 'OpenAI' &&
    /(?:^|[-_/])gpt-?5(?:$|[._-])/i.test(modelName ?? '')
  ) {
    return 'OpenAI.Avatar.type={gpt5}'
  }

  return COLOR_ICON_BASES.has(baseKey) ? `${baseKey}.Color` : trimmedName
}
