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
import { describe, expect, test } from 'vitest'

import { getModelSquareIconName } from '../model-square-icon'

describe('model square icon names', () => {
  test('upgrades legacy monochrome provider icons to color variants', () => {
    expect(getModelSquareIconName('Claude')).toBe('Claude.Color')
    expect(getModelSquareIconName('DeepSeek')).toBe('DeepSeek.Color')
  })

  test('uses the matching colored GPT-5 avatar for OpenAI GPT-5 models', () => {
    expect(getModelSquareIconName('OpenAI', 'gpt-5.6-sol')).toBe(
      'OpenAI.Avatar.type={gpt5}'
    )
  })

  test('preserves explicit icon variants configured by the catalog', () => {
    expect(getModelSquareIconName('Claude.Avatar.type={platform}')).toBe(
      'Claude.Avatar.type={platform}'
    )
  })
})
