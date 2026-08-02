import { T } from '@start9labs/start-sdk'
import { sdk } from './sdk'

export const uiPort = 8080
export const electrumPort = 50001

export const electrumBridge = (effects: T.Effects) =>
  sdk.host
    .getBridgeAddress(effects, {
      packageId: 'electrs',
      hostId: 'electrum',
      internalPort: electrumPort,
      ssl: false,
    })
    .const()
