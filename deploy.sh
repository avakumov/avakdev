#!/usr/bin/env bash
#
# deploy.sh — сборка и установка приложения avakumov на локальную машину.
#
# Что делает:
#   1. Собирает React-фронтенд (frontend/dist).
#   2. Копирует его в server/frontend-dist и встраивает в Go-бинарник (embed).
#   3. Собирает один исполняемый файл bin.
#   4. Устанавливает его как service systemd.
#   5. Настраивает Caddy для HTTPS (реверс-прокси на локальный порт).
#
# По умолчанию разворачивается на ТЕКУЩЕЙ машине (не на удалённом VPS).
#
# Требования:
#   - go, node/npm, sudo, systemd
#   - для HTTPS через Caddy нужен реальный домен или локальный сертификат
#
# Использование:
#   ./deploy.sh                       # домен возьмётся из DOMAIN или интерактивно
#   DOMAIN=example.com ./deploy.sh    # собрать и задеплоить с доменом
#   ./deploy.sh --install-deps        # дополнительно установить caddy
#
set -euo pipefail

# ---------------------------------------------------------------------------
# Конфигурация
# ---------------------------------------------------------------------------
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Домен для Caddy. Если пуст — во время работы спросим.
DOMAIN="${DOMAIN:-}"

# Служебный пользователь и каталог установки
APP_USER="${APP_USER:-avakumov}"
APP_GROUP="${APP_GROUP:-$APP_USER}"
INSTALL_DIR="${INSTALL_DIR:-/opt/$APP_USER}"

# Имя службы systemd и бинарника
SERVICE_NAME="${SERVICE_NAME:-avakumov}"
BIN_NAME="${BIN_NAME:-server}"
BIN_PATH="$INSTALL_DIR/$BIN_NAME"

# Локальный порт приложения, который слушает Go-сервер
APP_PORT="${APP_PORT:-8080}"

# Каталоги проекта
FRONTEND_DIR="$SCRIPT_DIR/frontend"
SERVER_DIR="$SCRIPT_DIR/server"
FRONTEND_DIST_STAGING="$SERVER_DIR/frontend-dist"

# ---------------------------------------------------------------------------
# Проверка окружения
# ---------------------------------------------------------------------------
command -v go    >/dev/null 2>&1 || { echo "Ошибка: go не установлен"; exit 1; }
command -v npm   >/dev/null 2>&1 || { echo "Ошибка: npm не установлен"; exit 1; }

if [[ "$(id -u)" -ne 0 ]]; then
  SUDO="sudo"
  echo "Запуск от непривилегированного пользователя: буду использовать '$SUDO'."
else
  SUDO=""
fi

# ---------------------------------------------------------------------------
# Шаг 0. Установка зависимостей (опционально)
# ---------------------------------------------------------------------------
if [[ "${1:-}" == "--install-deps" ]]; then
  echo "==> Установка caddy и создание служебного пользователя"
  if command -v apt-get >/dev/null 2>&1; then
    $SUDO apt-get update
    $SUDO apt-get install -y caddy
  elif command -v dnf >/dev/null 2>&1; then
    $SUDO dnf install -y caddy
  else
    echo "Не найден менеджер пакетов (apt/dnf). Установите Caddy вручную."
    exit 1
  fi
fi

# ---------------------------------------------------------------------------
# Шаг 1. Сборка фронтенда
# ---------------------------------------------------------------------------
echo "==> Сборка фронтенда"
(
  cd "$FRONTEND_DIR"

  # Устанавливаем зависимости только если их ещё нет (один раз),
  # а не при каждом деплое — это быстрее и без лишнего вывода npm.
  if [ ! -d node_modules ]; then
    echo "node_modules не найден — устанавливаю зависимости"
    if [ -f package-lock.json ]; then
      npm ci
    else
      npm install
    fi
  fi

  npm run build
)

# ---------------------------------------------------------------------------
# Шаг 2. Копирование статики для embed
# ---------------------------------------------------------------------------
echo "==> Подготовка статики для embed"
rm -rf "$FRONTEND_DIST_STAGING"/assets "$FRONTEND_DIST_STAGING"/index.html
cp -r "$FRONTEND_DIR"/dist/index.html "$FRONTEND_DIST_STAGING"/
cp -r "$FRONTEND_DIR"/dist/assets    "$FRONTEND_DIST_STAGING"/

# ---------------------------------------------------------------------------
# Шаг 3. Сборка бинарника (с тегом embed)
# ---------------------------------------------------------------------------
echo "==> Сборка Go-бинарника"
BUILD_DIR="$(mktemp -d)"
trap 'rm -rf "$BUILD_DIR"' EXIT

(
  cd "$SERVER_DIR"
  CGO_ENABLED=0 go build -trimpath -tags embed -ldflags '-s -w' \
      -o "$BUILD_DIR/$BIN_NAME" .
)
BIN_ARCHIVE="$BUILD_DIR/$BIN_NAME"

# ---------------------------------------------------------------------------
# Шаг 4. Установка и права
# ---------------------------------------------------------------------------

# Если пользователя ещё нет — создать его заранее
if ! id "$APP_USER" >/dev/null 2>&1; then
  echo "==> Создание служебного пользователя $APP_USER"
  $SUDO useradd --system --home "$INSTALL_DIR" --shell /usr/sbin/nologin "$APP_USER" || true
fi

echo "==> Установка в $INSTALL_DIR"
$SUDO install -d -o "$APP_USER" -g "$APP_GROUP" "$INSTALL_DIR"
$SUDO rm -f "$BIN_PATH"
$SUDO install -m 0755 "$BIN_ARCHIVE" "$BIN_PATH"

# Гарантируем владельца каталога (и сам бинарь) — важно для WorkingDirectory
$SUDO chown -R "$APP_USER":"$APP_GROUP" "$INSTALL_DIR"

# ---------------------------------------------------------------------------
# Шаг 5. Systemd-юнит
# ---------------------------------------------------------------------------
echo "==> Настройка systemd"
UNIT_FILE="/etc/systemd/system/$SERVICE_NAME.service"

$SUDO tee "$UNIT_FILE" >/dev/null <<EOF
[Unit]
Description=avakumov — Go + React приложение
After=network.target

[Service]
Type=simple
User=$APP_USER
Group=$APP_GROUP
WorkingDirectory=$INSTALL_DIR
ExecStart=$BIN_PATH
Restart=on-failure
RestartSec=5
Environment=PORT=$APP_PORT
Environment=GIN_MODE=release

[Install]
WantedBy=multi-user.target
EOF

$SUDO systemctl daemon-reload
$SUDO systemctl enable "$SERVICE_NAME"
$SUDO systemctl restart "$SERVICE_NAME"
echo "==> Сервис '$SERVICE_NAME' запущен"

# ---------------------------------------------------------------------------
# Шаг 6. Конфигурация Caddy (HTTPS)
# ---------------------------------------------------------------------------
if command -v caddy >/dev/null 2>&1 || $SUDO test -x /usr/bin/caddy; then
  # Если домен не задан — запрашиваем
  if [[ -z "$DOMAIN" ]]; then
    read -r -p "Введите домен для HTTPS (Enter — localhost): " DOMAIN
    DOMAIN="${DOMAIN:-localhost}"
  fi

  # Разрешаем браузеру обращение к 80/443 в SELinux/AppArmor средах
  # (раскомментируйте при необходимости)
  # $SUDO setsebool -P httpd_can_network_connect 1 2>/dev/null || true

  CADDY_CONF="/etc/caddy/Caddyfile"
  $SUDO mkdir -p "$(dirname "$CADDY_CONF")"

  echo "==> Прописываю Caddy для $DOMAIN -> 127.0.0.1:$APP_PORT"
  if $SUDO grep -q "$DOMAIN" "$CADDY_CONF" 2>/dev/null; then
    echo "Домен $DOMAIN уже есть в $CADDY_CONF — пропускаю."
  else
    $SUDO tee -a "$CADDY_CONF" >/dev/null <<EOF

$DOMAIN {
	reverse_proxy 127.0.0.1:$APP_PORT
}
EOF
  fi

  $SUDO systemctl enable caddy 2>/dev/null || true
  $SUDO systemctl restart caddy 2>/dev/null || true
  echo "==> Caddy перезапущен. HTTPS: https://$DOMAIN"
else
  echo "(!) Caddy не установлен. Пропускаю настройку HTTPS."
  echo "    Приложение доступно по http://127.0.0.1:$APP_PORT"
fi

echo
echo "Готово ✅"
echo "  Сервис:  systemctl status $SERVICE_NAME"
echo "  Логи:    journalctl -u $SERVICE_NAME -f"
echo "  Бинарь:  $BIN_PATH"
if [[ -n "$DOMAIN" ]]; then
  echo "  URL:     https://$DOMAIN"
fi
