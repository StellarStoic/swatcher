import { randomBytes } from 'node:crypto'
import { schnorr } from '@noble/curves/secp256k1'
import { bech32 } from '@scure/base'

const nameAlphabet = 'abcdefghjkmnpqrstuvwxyz23456789'

function decodeNsec(nsec: string): Uint8Array {
  const decoded = bech32.decodeToBytes(nsec)
  if (decoded.prefix !== 'nsec' || decoded.bytes.length !== 32) {
    throw new Error('Nostr sender key must be a valid nsec')
  }
  schnorr.getPublicKey(decoded.bytes)
  return decoded.bytes
}

export function validateRecipientNpub(value: string): string {
  const npub = value.trim()
  if (npub.toLowerCase().startsWith('nsec1')) {
    throw new Error(
      'I will not accept this key. You pasted an nsec, which is your secret private key. Please do not paste your nsec into websites. You got away with it here because this is your own server, but a malicious website could easily compromise your Nostr identity. Remember: nsec is secret—protect it. Now provide your npub, your public key.',
    )
  }
  let decoded
  try {
    decoded = bech32.decodeToBytes(npub)
  } catch {
    throw new Error('The Nostr recipient must be a valid npub public key.')
  }
  if (decoded.prefix !== 'npub' || decoded.bytes.length !== 32) {
    throw new Error('The Nostr recipient must be a valid npub public key.')
  }
  return npub.toLowerCase()
}

export function uniqueSenderName(): string {
  const entropy = randomBytes(6)
  let suffix = ''
  for (const byte of entropy) suffix += nameAlphabet[byte % nameAlphabet.length]
  return `swatcher-${suffix}`
}

export function ensureNostrIdentity(existingNsec: string): {
  nsec: string
  npub: string
  avatar: string
} {
  const secret = existingNsec
    ? decodeNsec(existingNsec)
    : schnorr.utils.randomSecretKey()
  const publicKey = schnorr.getPublicKey(secret)
  const nsec = bech32.encodeFromBytes('nsec', secret)
  const npub = bech32.encodeFromBytes('npub', publicKey)
  const avatar = `https://api.dicebear.com/10.x/pixelbot/svg?seed=${encodeURIComponent(npub)}`
  return { nsec, npub, avatar }
}
