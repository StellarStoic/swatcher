import { sdk } from '../sdk'
import { notifications } from './notifications'
import { webPassword } from './webPassword'

export const actions = sdk.Actions.of()
  .addAction(notifications)
  .addAction(webPassword)
