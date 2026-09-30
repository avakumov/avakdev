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

// Тема оформления кода по умолчанию (совпадает с DEFAULT в миграции).
export const CODE_THEME_DEFAULT = "night-owl";

// Применяет тему кода: атрибут data-code-theme на <html> (палитры — в
// highlight.css). Светлый/тёмный вариант выбирается там же, по классу .dark,
// поэтому при переключении темы сайта код перекрашивается автоматически.
// Значение хранится в профиле пользователя (/api/me).
export function applyCodeTheme(theme) {
  document.documentElement.dataset.codeTheme = theme || CODE_THEME_DEFAULT;
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

  // Запрос на открытие книги в разделе «Чтение» из другого раздела
  // (например, кнопкой «Читать» в карточке «Чтение» на странице «День»).
  // bookId: null значит «любая книга»; anchor — позиция закладки.
  readingRequest: null,
  openReading: (req) =>
    set({
      readingRequest: { bookId: null, anchor: null, excerpt: "", ...req },
    }),
  clearReadingRequest: () => set({ readingRequest: null }),

  // Быстрая заметка (раздел «Заметки»): плавающая кнопка «з» есть на всех
  // страницах. draftEditor = { id, content } — открытый редактор на весь экран;
  // id === null значит «новая заметка».
  draftEditor: null,
  openDraftEditor: (draft) =>
    set({ draftEditor: { id: null, content: "", ...draft } }),
  closeDraftEditor: () => set({ draftEditor: null }),

  reset: () => set({ lastUpdatedAt: null, loadedCards: 0 }),
}))
