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
import { describe, expect, it } from 'vitest'

import { DEFAULT_CONFIG, DEFAULT_PARAMETER_ENABLED } from '../../constants'
import type { Message, PlaygroundConfig } from '../../types'
import { buildChatCompletionPayload } from './payload-builder'

const messages: Message[] = [
  {
    key: 'message-1',
    from: 'user',
    versions: [{ id: 'version-1', content: 'hello' }],
  },
]

describe('buildChatCompletionPayload provider routing', () => {
  it('omits provider routing when no provider is selected', () => {
    const payload = buildChatCompletionPayload(
      messages,
      DEFAULT_CONFIG,
      DEFAULT_PARAMETER_ENABLED
    )

    expect(payload.provider).toBeUndefined()
  })

  it('serializes provider order and fallback policy', () => {
    const config: PlaygroundConfig = {
      ...DEFAULT_CONFIG,
      providerOrder: ['siliconflow', 'deepseek'],
      allowProviderFallbacks: false,
    }
    const payload = buildChatCompletionPayload(
      messages,
      config,
      DEFAULT_PARAMETER_ENABLED
    )

    expect(payload.provider).toEqual({
      order: ['siliconflow', 'deepseek'],
      allow_fallbacks: false,
    })
  })
})
