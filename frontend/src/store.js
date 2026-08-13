import { create } from 'zustand'

// Глобальное состояние приложения (Zustand).
// Здесь храним UI-состояние, которое нужно за пределами одного компонента.
export const useAppStore = create((set, get) => ({
  // Пример глобального счётчика запросов / последнего обновления.
  lastUpdatedAt: null,
  // Счётчик количества загруженных карточек (опциональный пример).
  loadedCards: 0,

  setLastUpdatedAt: (date) => set({ lastUpdatedAt: date }),
  incrementLoaded: () => set((state) => ({ loadedCards: state.loadedCards + 1 })),
  reset: () => set({ lastUpdatedAt: null, loadedCards: 0 }),
}))

// Отдельный store можно завести под auth-состояние, если понадобится.
// export const useAuthStore = create((set) => ({
//   user: null,
//   token: null,
//   setUser: (user) => set({ user }),
//   setToken: (token) => set({ token }),
//   logout: () => set({ user: null, token: null }),
// }))
