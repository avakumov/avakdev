#!/usr/bin/env bash
#
# dev-pg.sh — управление системным PostgreSQL (без Docker) для локальной разработки.
#
# Использование (требует sudo для операций с системным сервисом/БД):
#   ./scripts/dev-pg.sh status    — показать статус и строку подключения
#   ./scripts/dev-pg.sh init      — создать/инициализировать кластер и запустить сервис
#   ./scripts/dev-pg.sh start     — запустить сервис
#   ./scripts/dev-pg.sh stop      — остановить сервис
#   ./scripts/dev-pg.sh setup     — создать роль/БД/таблицу users и админа
#   ./scripts/dev-pg.sh psql      — открыть psql (от пользователя postgres)
#   ./scripts/dev-pg.sh url       — вывести строку подключения
#
# Подходит для систем на базе Arch/Manjaro (systemd + data dir /var/lib/postgres/data)
# и Debian/Ubuntu (data dir /var/lib/postgresql/<major>/main).
#
# Конфигурацию можно переопределить переменными окружения:
#   DEV_PG_USER       — роль БД                (по умолчанию: avakumov)
#   DEV_PG_PASSWORD   — пароль роли БД         (по умолчанию: PG_PASS из .env)
#   DEV_PG_DB         — название БД            (по умолчанию: avakumov)
#   DEV_PG_PORT       — порт сервера           (по умолчанию: 5432)
#   DEV_PG_DATA       — data directory         (автоопределение при пустом)
#
# Пароли берутся из .env (в git не хранятся): PG_PASS или пароль внутри
# DATABASE_URL. Администратор при 'setup' — из ADMIN_USER/ADMIN_PASSWORD/ADMIN_EMAIL.
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

DEV_PG_USER="${DEV_PG_USER:-avakumov}"
DEV_PG_DB="${DEV_PG_DB:-avakumov}"
DEV_PG_PORT="${DEV_PG_PORT:-5432}"
DEV_PG_DATA="${DEV_PG_DATA:-}"

# Пароль роли: DEV_PG_PASSWORD → PG_PASS → пароль из DATABASE_URL.
DEV_PG_PASSWORD="${DEV_PG_PASSWORD:-${PG_PASS:-}}"
if [ -z "$DEV_PG_PASSWORD" ] && [ -n "${DATABASE_URL:-}" ]; then
  DEV_PG_PASSWORD="$(printf '%s' "$DATABASE_URL" | sed -nE 's#^[a-zA-Z][a-zA-Z0-9+.-]*://[^:/@]+:([^@]*)@.*#\1#p')"
fi

db_url() {
  echo "postgres://$DEV_PG_USER:$DEV_PG_PASSWORD@127.0.0.1:$DEV_PG_PORT/$DEV_PG_DB?sslmode=disable"
}

# Строка автоинициализации под конкретный дистрибутив.
# Возвращает путь к data directory (или пустую строку, если не нашли).
detect_data_dir() {
  if [[ -n "$DEV_PG_DATA" ]]; then
    echo "$DEV_PG_DATA"
    return
  fi
  for _d_ in /var/lib/postgres/data /var/lib/postgresql/*/main; do
    if [ -e "$(dirname "$_d_")" ] || [ -d "$_d_" ]; then
      echo "$_d_"
      return
    fi
  done
  echo ""
}

require_sudo() {
  if ! sudo -n true 2>/dev/null; then
    echo "Требуется sudo. Выполните команду с правами root или введите пароль." >&2
    # Пробуем один раз запросить пароль интерактивно.
    sudo -v
  fi
}

run_as_postgres() {
  sudo -u postgres env PATH="$PATH" "$@"
}

cmd="${1:-status}"

case "$cmd" in
  status)
    if systemctl is-active --quiet postgresql.service 2>/dev/null; then
      echo "Статус: сервис postgresql запущен"
    else
      echo "Статус: сервис postgresql НЕ запущен (запустите: ./scripts/dev-pg.sh start)"
    fi
    if command -v pg_isready >/dev/null 2>&1 && pg_isready -q -p "$DEV_PG_PORT" 2>/dev/null; then
      echo "PostgreSQL принимает подключения на порту $DEV_PG_PORT"
    else
      echo "PostgreSQL на порту $DEV_PG_PORT не отвечает."
    fi
    echo "DATABASE_URL: $(db_url)"
    ;;

  init)
    require_sudo
    data_dir="$(detect_data_dir)"
    if [[ -z "$data_dir" ]]; then
      data_dir="/var/lib/postgres/data"
      echo "==> Не удалось определить data directory. Использую: $data_dir"
    fi
    echo "==> Создаю data directory: $data_dir"
    sudo install -d -o postgres -g postgres "$data_dir"
    if [ ! -f "$data_dir/PG_VERSION" ]; then
      echo "==> Инициализирую кластер (initdb)…"
      sudo -u postgres initdb -D "$data_dir" --encoding=UTF8 --locale=en_US.UTF-8
    else
      echo "==> Кластер уже инициализирован в $data_dir"
    fi
    echo "==> Включаю сервис и запускаю…"
    sudo systemctl enable --now postgresql.service
    echo "==> Готово. Создайте пользователя/БД: ./scripts/dev-pg.sh setup"
    ;;

  start)
    require_sudo
    data_dir="$(detect_data_dir)"
    if [[ -z "$data_dir" || ! -f "$data_dir/PG_VERSION" ]]; then
      echo "Кластер не инициализирован. Сначала: ./scripts/dev-pg.sh init" >&2
      exit 1
    fi
    sudo systemctl enable --now postgresql.service
    echo "==> postgresql.service запущен"
    ;;

  stop)
    require_sudo
    sudo systemctl disable --now postgresql.service
    echo "==> postgresql.service остановлен"
    ;;

  setup)
    require_sudo
    if [ -z "$DEV_PG_PASSWORD" ]; then
      echo "Ошибка: задайте пароль dev-роли в .env (PG_PASS) или через DEV_PG_PASSWORD." >&2
      exit 1
    fi
    echo "==> Создаю роль $DEV_PG_USER (если нет)"
    run_as_postgres psql -tAc "SELECT 1 FROM pg_roles WHERE rolname='$DEV_PG_USER'" | grep -q 1 \
      || run_as_postgres psql -c "CREATE ROLE $DEV_PG_USER LOGIN PASSWORD '$DEV_PG_PASSWORD';"
    echo "==> Создаю базу данных $DEV_PG_DB (если нет)"
    run_as_postgres psql -tAc "SELECT 1 FROM pg_database WHERE datname='$DEV_PG_DB'" | grep -q 1 \
      || run_as_postgres createdb -O "$DEV_PG_USER" "$DEV_PG_DB"
    echo "==> Создаю таблицу users"
    run_as_postgres psql -d "$DEV_PG_DB" -v ON_ERROR_STOP=1 <<'SQL'
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
      run_as_postgres psql -d "$DEV_PG_DB" -v ON_ERROR_STOP=1 \
        -v admin_user="${ADMIN_USER:-avakdev}" \
        -v admin_password="$ADMIN_PASSWORD" \
        -v admin_email="${ADMIN_EMAIL:-}" \
        -c "INSERT INTO users (username, password, email, is_admin) VALUES (:'admin_user', :'admin_password', :'admin_email', true) ON CONFLICT (username) DO NOTHING;"
    else
      echo "    ADMIN_PASSWORD не задан — администратор не создан (укажите в .env)."
    fi
    echo "==> Выдаю права прикладной роли"
    run_as_postgres psql -d "$DEV_PG_DB" -c "GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO $DEV_PG_USER;"
    run_as_postgres psql -d "$DEV_PG_DB" -c "GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO $DEV_PG_USER;"
    echo "==> Готово. Проверяю:"
    run_as_postgres psql -d "$DEV_PG_DB" -c "SELECT id, username, email, is_admin FROM users ORDER BY id;"
    ;;

  psql)
    require_sudo
    run_as_postgres psql -d "$DEV_PG_DB"
    ;;

  url)
    echo "$(db_url)"
    ;;

  *)
    echo "Использование: $0 {status|init|start|stop|setup|psql|url}" >&2
    exit 1
    ;;
esac
