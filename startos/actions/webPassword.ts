// Copyleft 2026 StellarStoic
// SPDX-License-Identifier: AGPL-3.0-or-later

import { sdk } from '../sdk'

const { InputSpec, Value } = sdk
const inputSpec = InputSpec.of({
  password: Value.text({
    name: 'Web password',
    description: 'At least 5 characters; stored only as an Argon2id hash',
    required: true,
    default: null,
    masked: true,
    minLength: 5,
    maxLength: 256,
  }),
  confirmPassword: Value.text({
    name: 'Confirm web password',
    required: true,
    default: null,
    masked: true,
    minLength: 5,
    maxLength: 256,
  }),
})

export const webPassword = sdk.Action.withInput(
  'web-password',
  {
    name: 'Set Web Password',
    description:
      'Set or replace the password required to open the s/watcher Web Interface',
    warning: 'Changing the password signs out every existing browser session.',
    allowedStatuses: 'any',
    group: null,
    visibility: 'enabled',
  },
  inputSpec,
  async () => ({ password: '', confirmPassword: '' }),
  async ({ effects, input }) => {
    if (input.password !== input.confirmPassword) {
      throw new Error('The passwords do not match.')
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
      'set-web-password',
      async (sub) => {
        await sub.execFail(
          [
            'sh',
            '-c',
            's-watcher set-web-password && chown swatcher:swatcher /data/auth.json',
          ],
          {
            input: input.password,
            user: 'root',
            env: { SWATCHER_DATA: '/data' },
          },
        )
      },
    )
    return {
      version: '1',
      title: 'Web password saved',
      message: 'Use the new password to open the s/watcher Web Interface.',
      result: null,
    }
  },
)
