import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import type { Session } from '../contracts/generated/models'
import { authApi } from '../api/auth'
import { STORAGE_KEYS } from '../constants/keys'
import { isPlainObject, safeJsonStorage } from '../utils/safeJsonStorage.js'

export type SessionStatus = 'loading' | 'anonymous' | 'authenticated' | 'error'

export interface Capabilities {
  canUseRoom: boolean
  canCreateRoom: boolean
  canManageCurrentRoom: boolean
  canManageMembers: boolean
  canManageInvites: boolean
  canManageSite: boolean
  canUploadLocalMedia: boolean
}

export const useUserStore = defineStore('user', () => {
  const status = ref<SessionStatus>('loading')
  const account = ref<Session | null>(null)
  const bindings = ref<Record<string, string>>(
    safeJsonStorage.read(STORAGE_KEYS.BINDINGS, {}, { validate: isPlainObject }) as Record<string, string>
  )
  const showNameModal = ref(false)
  const onNameSetCallback = ref<(() => void) | null>(null)

  const publicId = computed(() => account.value?.publicId || '')
  const role = computed(() => account.value?.role || 'GUEST')
  const isGuest = computed(() => account.value?.guest ?? true)
  const isAdmin = computed(() => role.value === 'PLATFORM_ADMIN')
  const currentUser = computed(() => ({
    name: account.value?.displayName || account.value?.username || '游客',
    publicId: publicId.value
  }))
  const accountLastLoginAt = computed(() => account.value?.lastLoginAt ?? null)
  const hasDisplayName = computed(() => Boolean(currentUser.value.name.trim() && currentUser.value.name !== '游客'))
  const accountType = computed<'guest' | 'admin'>(() => isAdmin.value ? 'admin' : 'guest')
  const capabilities = computed<Capabilities>(() => ({
    canUseRoom: status.value === 'authenticated',
    canCreateRoom: isAdmin.value,
    canManageCurrentRoom: isAdmin.value,
    canManageMembers: isAdmin.value,
    canManageInvites: false,
    canManageSite: isAdmin.value,
    canUploadLocalMedia: isAdmin.value
  }))

  const isAuthPassed = computed(() => status.value === 'authenticated')
  const isSignedIn = computed(() => status.value === 'authenticated' && Boolean(publicId.value))
  const canManageRooms = computed(() => capabilities.value.canManageCurrentRoom)
  const canCreatePlaylists = computed(() => isSignedIn.value)

  function capabilitiesForRoom(_creatorPublicId?: string): Capabilities {
    const managesRoom = isAdmin.value
    return {
      ...capabilities.value,
      canManageCurrentRoom: managesRoom,
      canManageMembers: managesRoom,
      canManageInvites: false
    }
  }

  function finishPendingNameAction(): void {
    if (!showNameModal.value || !hasDisplayName.value) return
    showNameModal.value = false
    const callback = onNameSetCallback.value
    onNameSetCallback.value = null
    callback?.()
  }

  function initAccount(session: Session): void {
    account.value = {
      publicId: session.publicId,
      username: session.username || '',
      displayName: session.displayName || session.username || '',
      role: session.role || 'GUEST',
      guest: session.guest,
      enabled: session.enabled ?? true,
      lastLoginAt: session.lastLoginAt ?? null
    }
    status.value = 'authenticated'
    finishPendingNameAction()
  }

  function initUser(serverPublicId: string, serverName: string, serverIsGuest: boolean, serverRole = 'GUEST', serverIsAdmin = false): false {
    initAccount({
      publicId: serverPublicId,
      username: account.value?.username || '',
      displayName: serverName,
      role: serverIsAdmin ? 'PLATFORM_ADMIN' : serverRole,
      guest: serverIsGuest,
      enabled: true,
      lastLoginAt: account.value?.lastLoginAt ?? null
    })
    return false
  }

  async function refreshAccount(): Promise<Session> {
    try {
      const session = await authApi.getAccountMe()
      initAccount(session)
      return session
    } catch (error) {
      status.value = 'anonymous'
      account.value = null
      throw error
    }
  }

  async function updateProfile(displayName: string): Promise<Session> {
    const session = await authApi.updateAccountProfile(displayName)
    initAccount(session)
    return session
  }

  async function logout(): Promise<void> {
    try {
      await authApi.logoutAccount()
    } finally {
      clearAccountIdentity()
    }
  }

  function updateBinding(platform: string, accountId: string): void {
    bindings.value = { ...bindings.value, [platform]: accountId }
    localStorage.setItem(STORAGE_KEYS.BINDINGS, JSON.stringify(bindings.value))
  }

  function resolveName(id: string, fallbackName?: string): string {
    if (!id) return 'Unknown'
    if (id === publicId.value) return currentUser.value.name
    return fallbackName || 'Unknown Agent'
  }

  function setPostNameAction(callback: () => void): void { onNameSetCallback.value = callback }
  function saveName(): void { finishPendingNameAction() }

  function resetAuthentication(): void {
    status.value = 'anonymous'
    account.value = null
  }

  function clearAccountIdentity(): void {
    status.value = 'anonymous'
    account.value = null
    showNameModal.value = false
    onNameSetCallback.value = null
    bindings.value = {}
    localStorage.removeItem(STORAGE_KEYS.BINDINGS)
  }

  return {
    status, account, bindings, showNameModal,
    publicId, role, isGuest, isAdmin, currentUser, accountLastLoginAt,
    hasDisplayName, accountType, capabilities, isAuthPassed, isSignedIn,
    canManageRooms, canCreatePlaylists, capabilitiesForRoom,
    initAccount, initUser, refreshAccount, updateProfile, logout,
    updateBinding, resolveName, setPostNameAction,
    saveName, resetAuthentication, clearAccountIdentity
  }
})
