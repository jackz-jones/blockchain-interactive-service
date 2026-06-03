import { create } from 'zustand'
import { persist } from 'zustand/middleware'

export interface TenantInfo {
  id: string
  name: string
  role: 'admin' | 'user'
}

interface AuthState {
  apiKey: string | null
  tenant: TenantInfo | null
  setApiKey: (key: string) => void
  setTenant: (tenant: TenantInfo) => void
  logout: () => void
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
      apiKey: null,
      tenant: null,
      setApiKey: (key: string) => set({ apiKey: key }),
      setTenant: (tenant: TenantInfo) => set({ tenant }),
      logout: () => set({ apiKey: null, tenant: null }),
    }),
    {
      name: 'chain-interactive-auth',
      partialize: (state) => ({ apiKey: state.apiKey, tenant: state.tenant }),
    }
  )
)
