import { computed, readonly, ref, shallowRef } from 'vue'

export const spotifyCallbackPath = '/spotify/callback'
const pendingKey = 'musicparty.spotify.pending'
const accountsOrigin = 'https://accounts.spotify.com'

type AuthStatus = 'disconnected' | 'connecting' | 'connected' | 'error'
type AuthError = 'spotify-auth-required' | 'spotify-premium-required'
type Pending = { state: string; verifier: string; expiresAt: number; clientId: string; returnPath: string }
type Grant = { accessToken: string; refreshToken: string; expiresAt: number; clientId: string }

const status = ref<AuthStatus>('disconnected')
const error = ref<AuthError | null>(null)
const grant = shallowRef<Grant | null>(null)
const grantedClientId = computed(() => grant.value?.clientId)
let revision = 0
let refreshing: Promise<string> | null = null

function clearPending() {
  try { sessionStorage.removeItem(pendingKey) } catch { /* Storage may be disabled. */ }
}

function disconnect() {
  revision++
  refreshing = null
  grant.value = null
  error.value = null
  status.value = 'disconnected'
  clearPending()
}

function fail(reason: AuthError) {
  disconnect()
  error.value = reason
  status.value = 'error'
}

function base64url(bytes: Uint8Array) {
  return btoa(String.fromCharCode(...bytes)).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '')
}

function safeReturnPath(path: string) {
  try {
    const url = new URL(path, location.origin)
    if (url.origin === location.origin && !url.pathname.startsWith('//') && url.pathname !== spotifyCallbackPath) {
      for (const key of ['code', 'state', 'error', 'error_description']) url.searchParams.delete(key)
      return `${url.pathname}${url.search}${url.hash}`
    }
  } catch { /* Use the main entry point. */ }
  return '/'
}

function callbackURL() {
  return new URL(spotifyCallbackPath, location.origin).href
}

async function connect(clientId: string, redirectUri: string) {
  disconnect()
  const attempt = revision
  status.value = 'connecting'
  try {
    if (!clientId || redirectUri !== callbackURL()) throw new Error('Spotify callback is unavailable')
    const state = base64url(crypto.getRandomValues(new Uint8Array(32)))
    const verifier = base64url(crypto.getRandomValues(new Uint8Array(32)))
    const digest = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(verifier))
    if (attempt !== revision) return
    const returnPath = safeReturnPath(location.href)
    const pending: Pending = { state, verifier, expiresAt: Date.now() + 10 * 60_000, clientId, returnPath }
    sessionStorage.setItem(pendingKey, JSON.stringify(pending))
    const url = new URL('/authorize', accountsOrigin)
    url.search = new URLSearchParams({
      client_id: clientId, response_type: 'code', redirect_uri: redirectUri, scope: 'streaming user-modify-playback-state',
      state, code_challenge_method: 'S256', code_challenge: base64url(new Uint8Array(digest)),
    }).toString()
    location.assign(url)
  } catch {
    if (attempt === revision) fail('spotify-auth-required')
  }
}

function readPending(): Pending | null {
  try {
    const value: unknown = JSON.parse(sessionStorage.getItem(pendingKey) ?? 'null')
    if (value && typeof value === 'object' && 'state' in value && typeof value.state === 'string' &&
        'verifier' in value && typeof value.verifier === 'string' && 'expiresAt' in value && typeof value.expiresAt === 'number' &&
        'clientId' in value && typeof value.clientId === 'string' && 'returnPath' in value && typeof value.returnPath === 'string') {
      return { state: value.state, verifier: value.verifier, expiresAt: value.expiresAt, clientId: value.clientId, returnPath: value.returnPath }
    }
  } catch { /* An unreadable record cannot authorize a callback. */ }
  return null
}

class InvalidGrant extends Error {}

async function exchange(clientId: string, params: Record<string, string>, previousRefreshToken = ''): Promise<Grant> {
  const response = await fetch(`${accountsOrigin}/api/token`, {
    method: 'POST', redirect: 'error', credentials: 'omit', referrerPolicy: 'no-referrer',
    signal: AbortSignal.timeout(15_000),
    headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
    body: new URLSearchParams({ client_id: clientId, ...params }),
  })
  const token: unknown = await response.json()
  if (!response.ok) {
    if (token && typeof token === 'object' && 'error' in token && token.error === 'invalid_grant') throw new InvalidGrant()
    throw new Error('Spotify authorization request failed')
  }
  if (!token || typeof token !== 'object' || !('access_token' in token) || typeof token.access_token !== 'string' || !token.access_token ||
      !('expires_in' in token) || typeof token.expires_in !== 'number' || !Number.isFinite(token.expires_in) || token.expires_in <= 0) {
    throw new Error('Invalid Spotify authorization response')
  }
  const refreshToken = 'refresh_token' in token && typeof token.refresh_token === 'string' && token.refresh_token ? token.refresh_token : previousRefreshToken
  if (!refreshToken) throw new Error('Spotify refresh token missing')
  return { clientId, accessToken: token.access_token, refreshToken, expiresAt: Date.now() + token.expires_in * 1000 }
}

async function handleCallback() {
  if (location.pathname !== spotifyCallbackPath) return
  const query = new URLSearchParams(location.search)
  const pending = readPending()
  clearPending()
  const code = query.get('code')
  const valid = pending && pending.expiresAt > Date.now() && pending.state === query.get('state')
  history.replaceState(history.state, '', valid ? safeReturnPath(pending.returnPath) : '/')
  if (!valid || !code || query.has('error')) {
    fail('spotify-auth-required')
    return
  }
  const attempt = ++revision
  status.value = 'connecting'
  try {
    const next = await exchange(pending.clientId, {
      grant_type: 'authorization_code', code, redirect_uri: callbackURL(), code_verifier: pending.verifier,
    })
    if (attempt !== revision) return
    grant.value = next
    error.value = null
    status.value = 'connected'
  } catch {
    if (attempt === revision) fail('spotify-auth-required')
  }
}

// A 401 can reject a token before expiry. Refresh only if it is still the active one.
async function getToken(rejectedToken?: string): Promise<string> {
  const current = grant.value
  if (!current) throw new Error('Spotify authorization required')
  if (refreshing) return refreshing
  if (current.accessToken !== rejectedToken && current.expiresAt - Date.now() > 60_000) return current.accessToken
  const attempt = revision
  refreshing = exchange(current.clientId, { grant_type: 'refresh_token', refresh_token: current.refreshToken }, current.refreshToken)
    .then(next => {
      if (attempt !== revision) throw new Error('Spotify authorization changed')
      grant.value = next
      return next.accessToken
    })
    .catch((cause: unknown) => {
      if (attempt === revision && cause instanceof InvalidGrant) fail('spotify-auth-required')
      throw cause
    })
    .finally(() => { if (attempt === revision) refreshing = null })
  return refreshing
}

export function useSpotifyAuth() {
  return { status: readonly(status), error: readonly(error), grantedClientId, connect, handleCallback, getToken, disconnect, fail }
}
