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
import { createContext } from 'react'

import type { ChannelHealthItem } from '../types'

export interface ChannelHealthState {
  /** Keyed by channel id; absent while the deferred request is still running. */
  byChannelId: Map<number, ChannelHealthItem>
  blockCount: number
  blockSeconds: number
  /** Unix seconds at which the first block of the window starts. */
  startTs: number
  /** False until the first response arrives, so cells can stay blank. */
  isLoaded: boolean
}

export const CHANNEL_HEALTH_EMPTY_STATE: ChannelHealthState = {
  byChannelId: new Map(),
  blockCount: 20,
  blockSeconds: 600,
  startTs: 0,
  isLoaded: false,
}

/**
 * Health data is fetched once per table page and read by every status cell, so
 * it travels through context instead of being threaded into each column def.
 */
export const ChannelHealthContext = createContext<ChannelHealthState>(
  CHANNEL_HEALTH_EMPTY_STATE
)
