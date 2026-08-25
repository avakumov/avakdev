import { create } from 'zustand'

const THEME_KEY = "avakumov-theme";

// Сохранённая тема из localStorage (по умолчанию — светлая).
function getStoredTheme() {
  try {
    return localStorage.getItem(THEME_KEY) === "dark" ? "dark" : "light";
  } catch {
    return "light";
  }
}

// Применяет тему: класс .dark на <html> (CSS-переменные в index.css) +
// сохранение выбора в localStorage.
export function applyTheme(theme) {
  document.documentElement.classList.toggle("dark", theme === "dark");
  try {
    localStorage.setItem(THEME_KEY, theme);
  } catch {
    /* localStorage недоступен — тема живёт до перезагрузки */
  }
}

// Применяет сохранённую тему (вызывается до первого рендера, чтобы не было
// «вспышки» светлой темы при включённой тёмной).
export function initTheme() {
  applyTheme(getStoredTheme());
}

// Глобальное состояние приложения (Zustand).
export const useAppStore = create((set) => ({
  // Пример глобального счётчика запросов / последнего обновления.
  lastUpdatedAt: null,
  // Счётчик количества загруженных карточек (опциональный пример).
  loadedCards: 0,

  // Тема оформления: "light" | "dark".
  theme: getStoredTheme(),
  setTheme: (theme) => {
    applyTheme(theme);
    set({ theme });
  },
  toggleTheme: () =>
    set((state) => {
      const theme = state.theme === "dark" ? "light" : "dark";
      applyTheme(theme);
      return { theme };
    }),

  setLastUpdatedAt: (date) => set({ lastUpdatedAt: date }),
  incrementLoaded: () => set((state) => ({ loadedCards: state.loadedCards + 1 })),
  reset: () => set({ lastUpdatedAt: null, loadedCards: 0 }),
}))
