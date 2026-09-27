#!/usr/bin/env bash
#
# db-pull.sh — копирует production-базу в локальную (только для разработки).
#
# Дамп снимается НА сервере по SSH (теми же реквизитами, что и deploy.sh),
# поэтому креды прода не уезжают на локальную машину. Затем дамп
# восстанавливается в локальную БД, на которую указывает DATABASE_URL.
#
# Использование:
#   ./scripts/db-pull.sh                  # прод → локальная БД (с подтверждением)
#   YES=1 ./scripts/db-pull.sh            # без подтверждения
#   SCHEMA_ONLY=1 ./scripts/db-pull.sh    # только структура, без данных
#   ALLOW_REMOTE=1 ./scripts/db-pull.sh   # разрешить цель не на localhost
#
# Параметры берутся из .env (как в deploy.sh);
# переменные окружения приоритетнее файла:
#   DEPLOY_HOST, DEPLOY_USER, DEPLOY_SSH_PORT, DEPLOY_KEY_FILE, INSTALL_DIR
# Локальная цель:
#   DATABASE_URL либо PG_USER / PG_PASS / PG_PORT / PG_DB
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# --- .env: те же правила, что в deploy.sh (окружение приоритетнее файла) ---
ENV_FILE="$SCRIPT_DIR/.env"
if [ -f "$ENV_FILE" ]; then
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
  done < "$ENV_FILE"
fi

# --- Реквизиты сервера (как в deploy.sh) ---
DEPLOY_HOST="${DEPLOY_HOST:-}"
DEPLOY_USER="${DEPLOY_USER:-root}"
DEPLOY_SSH_PORT="${DEPLOY_SSH_PORT:-22}"
DEPLOY_KEY_FILE="${DEPLOY_KEY_FILE:-}"
INSTALL_DIR="${INSTALL_DIR:-/opt/avakumov}"

# --- Локальная цель ---
PG_USER="${PG_USER:-avakumov}"
PG_PASS="${PG_PASS:-}"
PG_PORT="${PG_PORT:-5432}"
PG_DB="${PG_DB:-avakumov}"
# Строка подключения: из DATABASE_URL или собранная из PG_*. Пароль в скрипте
# не хранится — берётся из .env (окружение приоритетнее).
if [ -z "${DATABASE_URL:-}" ]; then
  if [ -z "$PG_PASS" ]; then
    echo "Ошибка: задайте DATABASE_URL или PG_PASS в .env" >&2
    exit 1
  fi
  DATABASE_URL="postgres://$PG_USER:$PG_PASS@127.0.0.1:$PG_PORT/$PG_DB?sslmode=disable"
fi

SCHEMA_ONLY="${SCHEMA_ONLY:-0}"
YES="${YES:-0}"

die() { echo "Ошибка: $*" >&2; exit 1; }

[ -n "$DEPLOY_HOST" ] || die "DEPLOY_HOST не задан — укажите сервер в .env или окружении"

# --- Безопасность: восстанавливаем только в локальную БД ---
case "$DATABASE_URL" in
  *@127.0.0.1:*|*@localhost:*|*@\[::1\]:*) : ;;
  *)
    if [ "${ALLOW_REMOTE:-0}" != "1" ]; then
      die "DATABASE_URL указывает не на localhost — отказ во избежание записи в чужую БД. Переопределить: ALLOW_REMOTE=1"
    fi
    ;;
esac

command -v ssh        >/dev/null 2>&1 || die "не найден ssh"
command -v pg_restore >/dev/null 2>&1 || die "не найден pg_restore (пакет postgresql-client)"
command -v psql       >/dev/null 2>&1 || die "не найден psql (пакет postgresql-client)"

SSH_ARGS=(-p "$DEPLOY_SSH_PORT" -o StrictHostKeyChecking=accept-new -o ConnectTimeout=15)
if [ -n "$DEPLOY_KEY_FILE" ]; then SSH_ARGS+=(-i "$DEPLOY_KEY_FILE"); fi
SSH_TARGET="${DEPLOY_USER}@${DEPLOY_HOST}"

if [ "$SCHEMA_ONLY" = "1" ]; then
  DUMP_MODE="только схема"
  DUMP_ARG="--schema-only"
else
  DUMP_MODE="схема + данные"
  DUMP_ARG=""
fi

echo "==> Источник: $SSH_TARGET ($INSTALL_DIR/.env → DATABASE_URL)"
echo "==> Цель:     ${DATABASE_URL%%\?*}  ($DUMP_MODE)"

# Локальная БД должна отвечать до того, как снимаем дамп.
psql "$DATABASE_URL" -c 'SELECT 1' >/dev/null 2>&1 \
  || die "локальная БД недоступна по DATABASE_URL (запустите: make pg-start)"

if [ "$YES" != "1" ]; then
  printf "Локальная база будет перезаписана данными из production. Продолжить? [y/N] "
  read -r _ans_ || true
  case "$_ans_" in
    y|Y|yes|YES) : ;;
    *) echo "Отменено."; exit 0 ;;
  esac
fi

echo "==> Освобождаю подключения к локальной БД (dev-сервер переподключится сам)…"
psql "$DATABASE_URL" -v ON_ERROR_STOP=0 -c \
  "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = current_database() AND pid <> pg_backend_pid();" \
  >/dev/null 2>&1 || true

# Удалённый скрипт: берёт DATABASE_URL из .env на сервере и пишет дамп в stdout.
# $1 — путь к .env, $2 — доп. аргумент pg_dump (пусто или --schema-only).
# Внутри — только двойные кавычки, чтобы не ломать внешние одинарные.
REMOTE_SCRIPT='
set -e
env_file="$1"
[ -f "$env_file" ] || { echo "на сервере нет файла $env_file" >&2; exit 1; }
if [ -r "$env_file" ]; then read_env=cat; else read_env="sudo cat"; fi
url=$($read_env "$env_file" | sed -n "s/^[[:space:]]*DATABASE_URL=//p" | head -n1 | tr -d "\r" | tr -d "\"")
[ -n "$url" ] || { echo "DATABASE_URL не найден в $env_file" >&2; exit 1; }
command -v pg_dump >/dev/null 2>&1 || { echo "на сервере не найден pg_dump" >&2; exit 1; }
exec pg_dump "$url" -Fc --no-owner --no-privileges $2
'

echo "==> Снимаю дамп на сервере и восстанавливаю локально…"
# pipefail: падение ssh/pg_dump уронит и всю цепочку.
set -o pipefail
printf '%s\n' "$REMOTE_SCRIPT" \
  | ssh "${SSH_ARGS[@]}" "$SSH_TARGET" bash -s -- "$INSTALL_DIR/.env" "$DUMP_ARG" \
  | pg_restore --clean --if-exists --no-owner --no-privileges -d "$DATABASE_URL"

echo "==> Готово."
if [ "$SCHEMA_ONLY" != "1" ]; then
  psql "$DATABASE_URL" -tAc "SELECT 'таблиц: ' || count(*) FROM pg_tables WHERE schemaname='public';" 2>/dev/null || true
fi
echo "    Недостающие миграции goose применит при старте: make dev"
