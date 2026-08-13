import { useQuery } from '@tanstack/react-query'

// Базовый url — тот же хост, что отдал страницу (через Go-сервер,
// в dev — через vite-прокси на него).
const BASE = ''

async function request(path) {
  const res = await fetch(`${BASE}${path}`)
  if (!res.ok) {
    throw new Error(`Ошибка запроса ${path}: ${res.status}`)
  }
  return res.json()
}

// Данные endpoint'а /api/health
export function useHealth() {
  return useQuery({
    queryKey: ['health'],
    queryFn: () => request('/api/health'),
    // Серверные данные достаточно обновлять редко.
    staleTime: 30_000,
    retry: 1,
  })
}

// Приветственное сообщение с /api/message
export function useMessage() {
  return useQuery({
    queryKey: ['message'],
    queryFn: () => request('/api/message'),
    staleTime: 30_000,
    retry: 1,
  })
}
