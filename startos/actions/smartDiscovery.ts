// Copyleft 2026 StellarStoic
// SPDX-License-Identifier: AGPL-3.0-or-later

import { stateConfig } from '../fileModels/state.json'
import { sdk } from '../sdk'

const { InputSpec, Value } = sdk
const inputSpec = InputSpec.of({
  gap: Value.number({
    name: 'Address discovery gap',
    description:
      'Consecutive unused addresses checked after the highest used address. Larger gaps make more Electrs queries.',
    required: true,
    default: 20,
    integer: true,
    min: 1,
    max: 500,
    step: 1,
    units: 'addresses',
  }),
})
type SmartDiscoveryInput = typeof inputSpec._TYPE

export const smartDiscovery = sdk.Action.withInput(
  'smart-discovery',
  {
    name: 'Smart Wallet Discovery',
    description:
      'Configure how far s/watcher scans beyond the last used wallet address',
    warning: null,
    allowedStatuses: 'any',
    group: 'General',
    visibility: 'enabled',
  },
  inputSpec,
  async ({ effects }) => {
    const state = await stateConfig.read().const(effects)
    return { gap: state?.discoveryGap ?? 20 } satisfies SmartDiscoveryInput
  },
  async ({ effects, input }) => {
    const mounts = sdk.Mounts.of().mountVolume({
      volumeId: 'main',
      subpath: null,
      mountpoint: '/data',
      readonly: false,
    })
    await sdk.SubContainer.withTemp(
      effects,
      { imageId: 's-watcher' },
      mounts,
      'set-discovery-gap',
      async (sub) => {
        await sub.execFail(
          [
            'sh',
            '-c',
            `s-watcher set-discovery-gap ${input.gap} && chown swatcher:swatcher /data/state.json`,
          ],
          { user: 'root', env: { SWATCHER_DATA: '/data' } },
        )
      },
    )
    await effects.restart()
    return {
      version: '1',
      title: 'Discovery gap saved',
      message: `s/watcher will keep ${input.gap} unused addresses beyond the highest used wallet address under observation.`,
      result: null,
    }
  },
)
