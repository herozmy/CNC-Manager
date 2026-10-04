import { readonly, ref } from 'vue'
import { getAuthStatus, getCurrentUser, login, logout, setupAdmin } from '../api'
import type { AuthUser } from '../api/types'

const user = ref<AuthUser | null>(null)
const ready = ref(false)
const setupRequired = ref(false)

async function initialize(): Promise<void> {
  try {
    const status = await getAuthStatus()
    setupRequired.value = status.setupRequired
    if (!status.setupRequired) {
      try {
        user.value = await getCurrentUser()
      } catch {
        user.value = null
      }
    }
  } finally {
    ready.value = true
  }
}

async function signIn(username: string, password: string): Promise<void> {
  user.value = await login(username, password)
  setupRequired.value = false
}

async function createAdmin(username: string, displayName: string, password: string): Promise<void> {
  user.value = await setupAdmin(username, displayName, password)
  setupRequired.value = false
}

async function signOut(): Promise<void> {
  try {
    await logout()
  } finally {
    user.value = null
  }
}

function clearUser(): void {
  user.value = null
}

export function useAuth() {
  return {
    user: readonly(user),
    ready: readonly(ready),
    setupRequired: readonly(setupRequired),
    initialize,
    signIn,
    createAdmin,
    signOut,
    clearUser
  }
}
