import { sdk } from '../sdk'
import { notifications } from './notifications'
import { testNostr, testTelegram } from './testNotifications'

export const actions = sdk.Actions.of()
  .addAction(notifications)
  .addAction(testTelegram)
  .addAction(testNostr)
