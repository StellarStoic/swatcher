// Copyleft 2026 StellarStoic
// SPDX-License-Identifier: AGPL-3.0-or-later

import { T } from '@start9labs/start-sdk'
import { sdk } from './sdk'

export const uiPort = 8080
export const electrumPort = 50001
export const torSocksPort = 9050

// electrs and mempool are not npm dependencies of this package, so their host
// ids are literals. electrs groups its plaintext Electrum port under host
// `electrum`; mempool serves its web UI on host `main`, binding `webui`.
export const electrumBridge = (effects: T.Effects) =>
  sdk.host
    .getBridgeAddress(effects, {
      packageId: 'electrs',
      hostId: 'electrum',
      internalPort: electrumPort,
      ssl: false,
    })
    .const()

export const mempoolHost = (effects: T.Effects) =>
  sdk.host.get(effects, { packageId: 'mempool', hostId: 'main' }).const()

export async function localMempoolUrls(effects: T.Effects): Promise<string[]> {
  const address = (await mempoolHost(effects))?.bindings[8080]?.interfaces.webui
    ?.addressInfo
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

// Resolves null while Tor is absent, which is what the app wants: it routes
// only .onion hosts through the proxy and leaves clearnet traffic direct.
export const torSocksBridge = (effects: T.Effects) =>
  sdk.host
    .getBridgeAddress(effects, {
      packageId: 'tor',
      hostId: 'socks',
      internalPort: torSocksPort,
    })
    .const()

export const usesOnionRelay = (relays: readonly string[]) =>
  relays.some((relay) => {
    try {
      return new URL(relay).hostname.toLowerCase().endsWith('.onion')
    } catch {
      return false
    }
  })
