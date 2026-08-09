// Copyright (C) 2026 StellarStoic
// SPDX-License-Identifier: AGPL-3.0-or-later

import { T } from '@start9labs/start-sdk'
import { sdk } from './sdk'

export const uiPort = 8080
export const electrumPort = 50001
export const torSocksPort = 9050

export const electrumBridge = (effects: T.Effects) =>
  sdk.host
    .getBridgeAddress(effects, {
      packageId: 'electrs',
      hostId: 'electrum',
      internalPort: electrumPort,
      ssl: false,
    })
    .const()

export async function localMempoolUrls(effects: T.Effects): Promise<string[]> {
  const host = await sdk.host
    .get(effects, { packageId: 'mempool', hostId: 'main' })
    .const()
  const address = host?.bindings[8080]?.interfaces.webui?.addressInfo
  if (!address) return []

  return address.nonLocal
    .filter({
      predicate: (hostname) =>
        (hostname.metadata.kind === 'plugin' &&
          hostname.metadata.packageId === 'tor') ||
        (!hostname.public && hostname.metadata.kind !== 'plugin'),
    })
    .format('urlstring')
}

export async function torSocksBridge(
  effects: T.Effects,
): Promise<string | undefined> {
  const host = await sdk.host
    .get(effects, { packageId: 'tor', hostId: 'socks' })
    .const()
  if (!host?.bindings[torSocksPort]) return undefined

  return `${await sdk.getOsIp(effects)}:${torSocksPort}`
}
