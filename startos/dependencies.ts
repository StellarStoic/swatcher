import { sdk } from './sdk'

export const setDependencies = sdk.setupDependencies(async () => ({
  electrs: {
    kind: 'running',
    versionRange: '>=0.11.1:11',
    healthChecks: ['electrs', 'sync'],
  },
}))
