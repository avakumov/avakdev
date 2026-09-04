// Готовые варианты аватаров (без картинок — эмодзи на цветном фоне).
export const AVATAR_PRESETS = [
  { id: "fox", emoji: "🦊", bg: "bg-orange-500" },
  { id: "panda", emoji: "🐼", bg: "bg-slate-600" },
  { id: "tiger", emoji: "🐯", bg: "bg-amber-500" },
  { id: "frog", emoji: "🐸", bg: "bg-green-600" },
  { id: "cat", emoji: "🐱", bg: "bg-rose-500" },
  { id: "owl", emoji: "🦉", bg: "bg-indigo-600" },
  { id: "robot", emoji: "🤖", bg: "bg-cyan-600" },
  { id: "rocket", emoji: "🚀", bg: "bg-violet-600" },
  { id: "star", emoji: "⭐", bg: "bg-yellow-500" },
  { id: "sun", emoji: "😎", bg: "bg-sky-500" },
];

// Найти пресет по id (для отображения выбранного).
export function presetById(id) {
  return AVATAR_PRESETS.find((p) => p.id === id);
}
