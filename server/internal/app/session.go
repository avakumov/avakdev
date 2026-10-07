package app

import "time"

// CookieName — имя cookie с токеном сессии (общее для handlers и agent).
const CookieName = "avakumov_session"

// SessionTTL — время жизни сессии (30 дней). Само хранилище сессий живёт
// в store (store.Sessions); здесь остаётся только политика срока жизни.
const SessionTTL = 30 * 24 * time.Hour
