/* eslint-disable react-refresh/only-export-components */
import { createContext, useContext, useEffect, useMemo, useRef, useState } from 'react'
import { Navigate, Outlet, useLocation } from 'react-router-dom'

import { authApi } from './api.ts'
import { STORAGE_REFRESH_TOKEN_KEY, STORAGE_TOKEN_KEY, STORAGE_USER_KEY } from './config.ts'
import type { AuthResult, LoginRequest, RegisterRequest, UserProfile } from './types.ts'

interface AuthContextValue {
  user: UserProfile | null
  token: string | null
  isAuthenticated: boolean
  isAdmin: boolean
  login: (payload: LoginRequest) => Promise<AuthResult>
  register: (payload: RegisterRequest) => Promise<AuthResult>
  refreshMe: () => Promise<void>
  logout: () => Promise<void>
}

const AuthContext = createContext<AuthContextValue | null>(null)
let inflightProfileToken: string | null = null
let inflightProfileRequest: Promise<UserProfile> | null = null

function fetchProfileOnce(token: string) {
  if (inflightProfileToken === token && inflightProfileRequest) {
    return inflightProfileRequest
  }
  inflightProfileToken = token
  inflightProfileRequest = authApi.me().finally(() => {
    inflightProfileToken = null
    inflightProfileRequest = null
  })
  return inflightProfileRequest
}

function persistAuth(result: AuthResult) {
  localStorage.setItem(STORAGE_TOKEN_KEY, result.accessToken)
  localStorage.setItem(STORAGE_REFRESH_TOKEN_KEY, result.refreshToken)
  localStorage.setItem(STORAGE_USER_KEY, JSON.stringify(result.user))
}

function clearAuthStorage() {
  localStorage.removeItem(STORAGE_TOKEN_KEY)
  localStorage.removeItem(STORAGE_REFRESH_TOKEN_KEY)
  localStorage.removeItem(STORAGE_USER_KEY)
}

function readStoredUser() {
  const raw = localStorage.getItem(STORAGE_USER_KEY)
  if (!raw || raw === 'undefined' || raw === 'null') {
    return null
  }

  try {
    return JSON.parse(raw) as UserProfile
  } catch {
    localStorage.removeItem(STORAGE_USER_KEY)
    return null
  }
}

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [token, setToken] = useState<string | null>(() => localStorage.getItem(STORAGE_TOKEN_KEY))
  const [user, setUser] = useState<UserProfile | null>(() => readStoredUser())
  const verifiedTokenRef = useRef<string | null>(null)

  useEffect(() => {
    if (!token) {
      verifiedTokenRef.current = null
      return
    }
    if (verifiedTokenRef.current === token) {
      return
    }
    verifiedTokenRef.current = token

    void fetchProfileOnce(token)
      .then((profile) => {
        setUser(profile)
        localStorage.setItem(STORAGE_USER_KEY, JSON.stringify(profile))
      })
      .catch(() => undefined)
  }, [token])

  const value = useMemo<AuthContextValue>(
    () => ({
      user,
      token,
      isAuthenticated: Boolean(token && user),
      isAdmin: user?.role === 'ADMIN',
      async login(payload) {
        const result = await authApi.login(payload)
        persistAuth(result)
        verifiedTokenRef.current = result.accessToken
        setToken(result.accessToken)
        setUser(result.user)
        return result
      },
      async register(payload) {
        const result = await authApi.register(payload)
        persistAuth(result)
        verifiedTokenRef.current = result.accessToken
        setToken(result.accessToken)
        setUser(result.user)
        return result
      },
      async refreshMe() {
        const profile = await authApi.me()
        setUser(profile)
        localStorage.setItem(STORAGE_USER_KEY, JSON.stringify(profile))
      },
      async logout() {
        try {
          await authApi.logout()
        } finally {
          clearAuthStorage()
          verifiedTokenRef.current = null
          setToken(null)
          setUser(null)
        }
      },
    }),
    [token, user],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth() {
  const context = useContext(AuthContext)
  if (!context) {
    throw new Error('useAuth must be used within AuthProvider')
  }
  return context
}

export function RequireAuth() {
  const { isAuthenticated } = useAuth()
  const location = useLocation()

  if (!isAuthenticated) {
    return <Navigate to="/login" replace state={{ from: location.pathname }} />
  }

  return <Outlet />
}

export function GuestOnlyRoute() {
  const { isAuthenticated } = useAuth()
  if (isAuthenticated) {
    return <Navigate to="/files" replace />
  }
  return <Outlet />
}

export function RequireAdmin() {
  const { isAdmin } = useAuth()
  if (!isAdmin) {
    return <Navigate to="/files" replace />
  }
  return <Outlet />
}
