import { sdk } from '../sdk'
import { notifications } from './notifications'

export const actions = sdk.Actions.of().addAction(notifications)
