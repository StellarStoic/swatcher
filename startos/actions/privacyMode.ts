// Copyleft 2026 StellarStoic
// SPDX-License-Identifier: AGPL-3.0-or-later

import { stateConfig } from '../fileModels/state.json'
import { i18n } from '../i18n'
import { sdk } from '../sdk'

const { InputSpec, Value } = sdk
const inputSpec = InputSpec.of({
  privacyMode: Value.toggle({
    name: i18n('Enable privacy mode'),
    description: i18n(
      'Masks balances, activity amounts, wallet identifiers, transaction IDs, and the displayed Nostr sender npub in the Web Interface.',
    ),
    default: false,
  }),
  password: Value.text({
    name: i18n('Web password when disabling'),
    description: i18n(
      'Required only when turning privacy mode off. The password is verified without being stored.',
    ),
    required: false,
    default: null,
    masked: true,
    minLength: 5,
    maxLength: 256,
  }),
})
type PrivacyModeInput = typeof inputSpec._TYPE

export const privacyMode = sdk.Action.withInput(
  'privacy-mode',
  {
    name: i18n('Privacy Mode'),
    description: i18n(
      'Control masking of sensitive values in the Web Interface',
    ),
    warning: null,
    allowedStatuses: 'any',
    group: 'General',
    visibility: 'enabled',
  },
  inputSpec,
  async ({ effects }) => {
    const state = await stateConfig.read().const(effects)
    return {
      privacyMode: state?.privacyMode ?? false,
      password: null,
    } satisfies PrivacyModeInput
  },
  async ({ effects, input }) => {
    const state = await stateConfig.read().once()
    const result = {
      version: '1' as const,
      title: i18n('Privacy mode updated'),
      message: i18n('The Web Interface now applies the new masking setting.'),
      result: null,
    }
    if (input.privacyMode === (state?.privacyMode ?? false)) return result
    if (!input.privacyMode && !input.password) {
      throw new Error(i18n('Enter the Web Password to disable Privacy Mode.'))
    }
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
      'set-privacy-mode',
      async (sub) => {
        await sub.execFail(
          [
            'sh',
            '-c',
            `s-watcher set-privacy-mode ${input.privacyMode ? 'enabled' : 'disabled'} && chown swatcher:swatcher /data/state.json`,
          ],
          {
            // The binary reads stdin unconditionally, but only verifies the
            // password when disabling. Send a newline rather than an empty
            // string when it is unused: the SDK skips its stdin write — and
            // the close that goes with it — for a falsy `input`, leaving the
            // read to block until the exec is killed.
            input: input.password || '\n',
            user: 'root',
            env: { SWATCHER_DATA: '/data' },
          },
        )
      },
    )
    await effects.restart()
    return result
  },
)
