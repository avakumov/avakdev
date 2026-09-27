#!/usr/bin/env bash
#
# setup_db.sh — создаёт на сервере PostgreSQL роль avakumov, базу данных avakumov
# и заполняет таблицу users справочным администратором (если её ещё нет).
#
# Запуск от root (или с sudo) на целевом сервере:
#   sudo ./setup_db.sh
#
# Пароли/админ берутся из .env (в git не хранятся):
#   PG_PASS (или пароль внутри DATABASE_URL),
#   ADMIN_USER / ADMIN_PASSWORD / ADMIN_EMAIL.
#
set -euo pipefail

# --- .env в корне репозитория: окружение приоритетнее файла ---
_env_file="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/.env"
if [ -f "$_env_file" ]; then
  while IFS= read -r _line_ || [ -n "$_line_" ]; do
    case "$_line_" in
      ''|\#*) continue ;;
      *=*)
        _key_="${_line_%%=*}"
        _val_="${_line_#*=}"
        # НЕ перезаписываем переменные, уже заданные в окружении.
        if [ -z "${!_key_-}" ]; then export "$_key_=$_val_"; fi
        ;;
    esac
  done < "$_env_file"
fi

DB_APP_USER="${DB_APP_USER:-avakumov}"
DB_APP_NAME="${DB_APP_NAME:-avakumov}"

# Пароль прикладной роли: DB_APP_PASSWORD → PG_PASS → пароль из DATABASE_URL.
DB_APP_PASSWORD="${DB_APP_PASSWORD:-${PG_PASS:-}}"
if [ -z "$DB_APP_PASSWORD" ] && [ -n "${DATABASE_URL:-}" ]; then
  DB_APP_PASSWORD="$(printf '%s' "$DATABASE_URL" | sed -nE 's#^[a-zA-Z][a-zA-Z0-9+.-]*://[^:/@]+:([^@]*)@.*#\1#p')"
fi
if [ -z "$DB_APP_PASSWORD" ]; then
  echo "Ошибка: задайте пароль роли БД в .env (PG_PASS) или через DB_APP_PASSWORD." >&2
  exit 1
fi

echo "==> Проверяю, установлен ли PostgreSQL"
if ! command -v psql >/dev/null 2>&1 && ! [ -x /usr/lib/postgresql/*/bin/psql ]; then
  echo "Ошибка: PostgreSQL не установлен." >&2
  exit 1
fi

run_psql() {
  sudo -u postgres psql -v ON_ERROR_STOP=1 "$@"
}

echo "==> Создаю роль $DB_APP_USER (если нет)"
run_psql -tAc "SELECT 1 FROM pg_roles WHERE rolname='$DB_APP_USER'" | grep -q 1 \
  || run_psql -c "CREATE ROLE $DB_APP_USER LOGIN PASSWORD '$DB_APP_PASSWORD';"

echo "==> Создаю базу данных $DB_APP_NAME (если нет)"
run_psql -tAc "SELECT 1 FROM pg_database WHERE datname='$DB_APP_NAME'" | grep -q 1 \
  || run_psql -c "CREATE DATABASE $DB_APP_NAME OWNER $DB_APP_USER;"

echo "==> Создаю таблицу users"
run_psql -d "$DB_APP_NAME" <<'SQL'
CREATE TABLE IF NOT EXISTS users (
    id       SERIAL PRIMARY KEY,
    username TEXT NOT NULL UNIQUE,
    password TEXT NOT NULL,
    email    TEXT NOT NULL DEFAULT '',
    is_admin BOOLEAN NOT NULL DEFAULT false
);
SQL

if [ -n "${ADMIN_PASSWORD:-}" ]; then
  echo "==> Добавляю администратора ${ADMIN_USER:-avakdev}"
  run_psql -d "$DB_APP_NAME" \
    -v admin_user="${ADMIN_USER:-avakdev}" \
    -v admin_password="$ADMIN_PASSWORD" \
    -v admin_email="${ADMIN_EMAIL:-}" \
    -c "INSERT INTO users (username, password, email, is_admin) VALUES (:'admin_user', :'admin_password', :'admin_email', true) ON CONFLICT (username) DO NOTHING;"
else
  echo "    ADMIN_PASSWORD не задан — администратор не создан (укажите в .env)."
fi

echo "==> Готово. Проверяю пользователей:"
run_psql -d "$DB_APP_NAME" -c "SELECT id, username, email, is_admin FROM users ORDER BY id;"
