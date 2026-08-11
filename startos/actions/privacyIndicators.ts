// Copyleft 2026 StellarStoic
// SPDX-License-Identifier: AGPL-3.0-or-later

import { stateConfig } from '../fileModels/state.json'
import { sdk } from '../sdk'

const { InputSpec, Value } = sdk
const inputSpec = InputSpec.of({
  addressReuse: Value.toggle({
    name: 'Show address-reuse indicators',
    description: 'Counts observed receipts to the same watched address',
    default: true,
  }),
  smallDeposit: Value.toggle({
    name: 'Show small-deposit indicators',
    default: true,
  }),
  smallDepositThreshold: Value.number({
    name: 'Small-deposit threshold',
    description:
      'Incoming amounts below this value receive an informational badge',
    required: true,
    default: 1000,
    integer: true,
    min: 1,
    max: 2100000000000000,
    units: 'sat',
  }),
  combinedWallets: Value.toggle({
    name: 'Show combined-wallet indicators',
    description: 'Marks transactions involving more than one watched wallet',
    default: true,
  }),
})
type PrivacyIndicatorsInput = typeof inputSpec._TYPE

export const privacyIndicators = sdk.Action.withInput(
  'privacy-indicators',
  {
    name: 'Privacy Indicators',
    description: 'Configure informational transaction privacy badges',
    warning: null,
    allowedStatuses: 'any',
    group: 'General',
    visibility: 'enabled',
  },
  inputSpec,
  async ({ effects }) => {
    const state = await stateConfig.read().const(effects)
    const configured = state?.privacyIndicatorsConfigured ?? false
    return {
      addressReuse: configured ? (state?.addressReuseIndicators ?? true) : true,
      smallDeposit: configured ? (state?.smallDepositIndicators ?? true) : true,
      smallDepositThreshold: state?.smallDepositThreshold ?? 1000,
      combinedWallets: configured
        ? (state?.combinedWalletIndicators ?? true)
        : true,
    } satisfies PrivacyIndicatorsInput
  },
  async ({ effects, input }) => {
    const mounts = sdk.Mounts.of().mountVolume({
      volumeId: 'main',
      subpath: null,
      mountpoint: '/data',
      readonly: false,
    })
    const flag = (enabled: boolean) => (enabled ? 'enabled' : 'disabled')
    await sdk.SubContainer.withTemp(
      effects,
      { imageId: 's-watcher' },
      mounts,
      'set-privacy-indicators',
      async (sub) => {
        await sub.execFail(
          [
            'sh',
            '-c',
            `s-watcher set-privacy-indicators ${input.smallDepositThreshold} ${flag(input.addressReuse)} ${flag(input.smallDeposit)} ${flag(input.combinedWallets)} && chown swatcher:swatcher /data/state.json`,
          ],
          { user: 'root', env: { SWATCHER_DATA: '/data' } },
        )
      },
    )
    await effects.restart()
  },
)
