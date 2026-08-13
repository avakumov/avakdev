#!/usr/bin/env bash
#
# deploy.sh — сборка и установка приложения avakumov.
#
# Поддерживает два режима:
#   - Локальная машина (по умолчанию), если DEPLOY_HOST не задан.
#   - Удалённый VDS/VPS-сервер по SSH, если DEPLOY_HOST задан.
#
# Что делает (сборка всегда локальная):
#   1. Собирает React-фронтенд (frontend/dist).
#   2. Копирует его в server/frontend-dist и встраивает в Go-бинарник (embed).
#   3. Собирает один исполняемый файл.
#   4. Ставит его (локально или по VPS) и создаёт служебного пользователя.
#   5. Создаёт и активирует systemd-юнит.
#   6. Настраивает Caddy для HTTPS (реверс-прокси на локальный порт).
#
# Требования:
#   - локально: go, node/npm
#   - локально (для локального деплоя): sudo, systemd
#   - для удалённого деплоя: ssh/scp + ключ без пароля (или ssh-agent)
#   - для HTTPS через Caddy нужен реальный домен или локальный сертификат
#
# Использование:
#   ./deploy.sh                       # локальный деплой; домен спросит или из .env
#   DOMAIN=example.com ./deploy.sh    # то же, но с явным доменом
#   DEPLOY_HOST=vps ./deploy.sh       # удалённый деплой по SSH (алиас или host)
#   ./deploy.sh --install-deps        # дополнительно установить Caddy на цели
#
set -euo pipefail

# ---------------------------------------------------------------------------
# Конфигурация
# ---------------------------------------------------------------------------
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Загружаем переменные из .env (если файл есть).
# Готовые значения .env перекрываются переменными окружения,
# переданными извне (переменная окружения приоритетнее).
ENV_FILE="$SCRIPT_DIR/.env"
if [ -f "$ENV_FILE" ]; then
  echo "==> Использую переменные из $ENV_FILE"
  # Считываем только строки вида KEY=VALUE, не выполняя содержимое.
  while IFS= read -r _line_ || [ -n "$_line_" ]; do
    case "$_line_" in
      ''|\#*) continue ;;
      *=*)
        _key_="${_line_%%=*}"
        _val_="${_line_#*=}"
        # НЕ перезаписываем переменные, уже заданные в окружении.
        if [ -z "${!_key_-}" ]; then
          export "$_key_=$_val_"
        fi
        ;;
    esac
  done < "$ENV_FILE"
fi

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

# ---------------------------------------------------------------------------
# Удалённый VDS/VPS (по SSH)
# ---------------------------------------------------------------------------
# Если DEPLOY_HOST не пуст — деплоим удалённо, иначе локально.
DEPLOY_HOST="${DEPLOY_HOST:-}"
DEPLOY_USER="${DEPLOY_USER:-root}"
DEPLOY_SSH_PORT="${DEPLOY_SSH_PORT:-22}"

# Каталог на удалённой машине, куда кладём бинарник.
# По умолчанию — INSTALL_DIR (затем всё ставится через sudo на удалённой стороне).
DEPLOY_REMOTE_DIR="${DEPLOY_REMOTE_DIR:-$INSTALL_DIR}"

# Путь к приватному SSH-ключу (-i). Можно задать в .env.
DEPLOY_KEY_FILE="${DEPLOY_KEY_FILE:-}"

# Флаги для ssh и scp отличаются: у ssh порт — -p, у scp — -P (заглавная).
SSH_ARGS=(-p "$DEPLOY_SSH_PORT" -o StrictHostKeyChecking=accept-new -o ConnectTimeout=15)
SCP_ARGS=(-P "$DEPLOY_SSH_PORT" -o StrictHostKeyChecking=accept-new -o ConnectTimeout=15)

# Если задан ключ — добавляем его в аргументы ssh и scp.
if [[ -n "$DEPLOY_KEY_FILE" ]]; then
  SSH_ARGS+=(-i "$DEPLOY_KEY_FILE")
  SCP_ARGS+=(-i "$DEPLOY_KEY_FILE")
fi
SSH_TARGET="${DEPLOY_USER}@${DEPLOY_HOST}"

# Каталоги проекта
FRONTEND_DIR="$SCRIPT_DIR/frontend"
SERVER_DIR="$SCRIPT_DIR/server"
FRONTEND_DIST_STAGING="$SERVER_DIR/frontend-dist"

# ---------------------------------------------------------------------------
# Проверка окружения
# ---------------------------------------------------------------------------
command -v go    >/dev/null 2>&1 || { echo "Ошибка: go не установлен"; exit 1; }
command -v npm   >/dev/null 2>&1 || { echo "Ошибка: npm не установлен"; exit 1; }

if [[ -n "$DEPLOY_HOST" ]]; then
  # Удалённый режим
  command -v ssh >/dev/null 2>&1 || { echo "Ошибка: ssh не установлен"; exit 1; }
  command -v scp >/dev/null 2>&1 || { echo "Ошибка: scp не установлен"; exit 1; }
  echo "==> Режим: удалённый VDS ($DEPLOY_HOST). Сборка выполняется локально."
  SUDO="sudo"
  # Проверяем доступность SSH-соединения
  ssh "${SSH_ARGS[@]}" "$SSH_TARGET" 'echo "SSH OK: $(hostname)"' >/dev/null 2>&1 \
    || { echo "Ошибка: не удалось подключиться по SSH к $SSH_TARGET"; exit 1; }
else
  echo "==> Режим: локальная машина (DEPLOY_HOST не задан)."
  if [[ "$(id -u)" -ne 0 ]]; then
    SUDO="sudo"
    echo "Запуск от непривилегированного пользователя: буду использовать '$SUDO'."
  else
    SUDO=""
  fi
fi

# ---------------------------------------------------------------------------
# Хелперы исполнения команд на цели (локально или удалённо)
# ---------------------------------------------------------------------------
# run_on_target 'cmd' — выполняет команду на цели (удалённо по SSH или локально).
# Всегда принимает команду единой строкой; параметр присоединяется к строке.
run_on_target() {
  if [[ -n "$DEPLOY_HOST" ]]; then
    ssh "${SSH_ARGS[@]}" "$SSH_TARGET" "$*"
  else
    # локально — выполняем как единую shell-строку (bash)
    bash -c "$*"
  fi
}

# tee_on_target — пишет heredoc-содержимое в файл на цели.
# Синтаксис:  tee_on_target [-a] /path <<'EOF' ... EOF
#   -a  — дописать в конец файла (append), полезно для Caddyfile.
tee_on_target() {
  local append=""
  local dst=""
  if [[ "${1:-}" == "-a" ]]; then
    append="-a"
    dst="$2"
  else
    dst="$1"
  fi
  if [[ -n "$DEPLOY_HOST" ]]; then
    ssh "${SSH_ARGS[@]}" "$SSH_TARGET" "sudo tee $append '$dst' >/dev/null"
  else
    $SUDO tee $append "$dst" >/dev/null
  fi
}

# ---------------------------------------------------------------------------
# Шаг 0. Установка зависимостей (опционально)
# ---------------------------------------------------------------------------
if [[ "${1:-}" == "--install-deps" ]]; then
  echo "==> Установка caddy на целевой машине"
  if [[ "$(run_on_target 'if command -v caddy >/dev/null 2>&1; then echo yes; else echo no; fi')" == "yes" ]]; then
    echo "Caddy уже установлен на цели."
  elif [[ "$(run_on_target 'if command -v apt-get >/dev/null 2>&1; then echo apt; elif command -v dnf >/dev/null 2>&1; then echo dnf; else echo none; fi')" == "apt" ]]; then
    echo "==> Устанавливаю Caddy через официальный apt-репозиторий"
    # Caddy не входит в официальные репозитории Ubuntu/Debian,
    # поэтому подключаем его официальный репозиторий.
    run_on_target 'sudo apt-get update
      sudo apt-get install -y curl gnupg apt-transport-https ca-certificates'
    run_on_target "curl -fsSL 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' \
      | sudo gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg"
    # Список источников для Caddy (добавляем только если его нет)
    run_on_target "if [ ! -f /etc/apt/sources.list.d/caddy-stable.list ]; then \
        echo 'deb [signed-by=/usr/share/keyrings/caddy-stable-archive-keyring.gpg] https://dl.cloudsmith.io/public/caddy/stable/deb/debian any-version main' \
        | sudo tee /etc/apt/sources.list.d/caddy-stable.list >/dev/null; \
      fi"
    run_on_target 'sudo apt-get update
      sudo apt-get install -y caddy'
  elif [[ "$(run_on_target 'if command -v dnf >/dev/null 2>&1; then echo dnf; else echo none; fi')" == "dnf" ]]; then
    run_on_target 'sudo dnf install -y caddy'
  else
    echo "Не найден менеджер пакетов (apt/dnf). Установите Caddy вручную."; exit 1
  fi
  echo "==> Проверяю установку Caddy"
  run_on_target 'if ! command -v caddy >/dev/null 2>&1; then echo "Ошибка: Caddy всё ещё не установлен на цели."; exit 1; fi'
fi

# ---------------------------------------------------------------------------
# Шаг 1. Сборка фронтенда
# ---------------------------------------------------------------------------
echo "==> Сборка фронтенда"

# Проверяем права на каталог сборки фронтенда. Если он принадлежит root
# (например, скрипт ранее запускался через sudo), обычный пользователь
# не сможет его очистить — Vite упадёт с загадочным EACCES. Остановимся
# заранее с понятным сообщением.
# Проверяем владельца каталога сборки и вложенного assets (могут быть от root
# из-за прежних запусков через sudo). Если кто-то из них root — обычный
# пользователь не сможет их очистить, и Vite упадёт с загадочным EACCES.
need_chown=0
for _d_ in "$FRONTEND_DIR/dist" "$FRONTEND_DIR/dist/assets"; do
  if [ -d "$_d_" ] && [ "$(stat -c '%U' "$_d_" 2>/dev/null || echo unknown)" = "root" ]; then
    echo
    echo "ОШИБКА: каталог $_d_ принадлежит root."
    echo "Скрипт ранее запускался через 'sudo'. Верните владельца пользователю:"
    echo "  sudo chown -R \"\$(whoami):\$(whoami)\" $FRONTEND_DIR/dist $FRONTEND_DIST_STAGING"
    echo "После этого запустите скрипт БЕЗ sudo."
    exit 1
  fi
done
unset _d_
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

# Аналогичная проверка для staging-каталога embed-статики.
# Проверяем владельца staging-каталога embed-статики и вложенного assets.
for _d_ in "$FRONTEND_DIST_STAGING" "$FRONTEND_DIST_STAGING/assets"; do
  if [ -d "$_d_" ] && [ "$(stat -c '%U' "$_d_" 2>/dev/null || echo unknown)" = "root" ]; then
    echo
    echo "ОШИБКА: каталог $_d_ принадлежит root."
    echo "Скрипт ранее запускался через 'sudo'. Верните владельца пользователю:"
    echo "  sudo chown -R \"\$(whoami):\$(whoami)\" $FRONTEND_DIST_STAGING"
    echo "После этого запустите скрипт БЕЗ sudo."
    exit 1
  fi
done
unset _d_

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
# Шаг 4. Доставка бинарника на цель
# ---------------------------------------------------------------------------
if [[ -n "$DEPLOY_HOST" ]]; then
  echo "==> Копирую бинарник на $SSH_TARGET:$DEPLOY_REMOTE_DIR/$BIN_NAME"
  ssh "${SSH_ARGS[@]}" "$SSH_TARGET" "sudo mkdir -p '$DEPLOY_REMOTE_DIR' && sudo chown '$APP_USER':'$APP_GROUP' '$DEPLOY_REMOTE_DIR' 2>/dev/null || true"
  scp "${SCP_ARGS[@]}" "$BIN_ARCHIVE" "$SSH_TARGET:/tmp/$BIN_NAME"
  ssh "${SSH_ARGS[@]}" "$SSH_TARGET" "sudo install -m 0755 /tmp/$BIN_NAME '$DEPLOY_REMOTE_DIR/$BIN_NAME' && rm -f /tmp/$BIN_NAME"
  BIN_PATH="$DEPLOY_REMOTE_DIR/$BIN_NAME"
else
  echo "==> Установка в $INSTALL_DIR"
  # Если пользователя ещё нет — создать его заранее
  if ! id "$APP_USER" >/dev/null 2>&1; then
    echo "==> Создание служебного пользователя $APP_USER"
    $SUDO useradd --system --home "$INSTALL_DIR" --shell /usr/sbin/nologin "$APP_USER" || true
  fi

  $SUDO install -d -o "$APP_USER" -g "$APP_GROUP" "$INSTALL_DIR"
  $SUDO rm -f "$BIN_PATH"
  $SUDO install -m 0755 "$BIN_ARCHIVE" "$BIN_PATH"

  # Гарантируем владельца каталога (и сам бинарь) — важно для WorkingDirectory
  $SUDO chown -R "$APP_USER":"$APP_GROUP" "$INSTALL_DIR"
fi

# ---------------------------------------------------------------------------
# Шаг 5. Systemd-юнит
# ---------------------------------------------------------------------------
echo "==> Настройка systemd"

if [[ -n "$DEPLOY_HOST" ]]; then
  # Удалённо: создаём пользователя и каталог, затем юнит
  ssh "${SSH_ARGS[@]}" "$SSH_TARGET" "
    set -e
    if ! id '$APP_USER' >/dev/null 2>&1; then
      sudo useradd --system --home '$INSTALL_DIR' --shell /usr/sbin/nologin '$APP_USER' || true
    fi
    sudo mkdir -p '$INSTALL_DIR'
    sudo chown -R '$APP_USER':'$APP_GROUP' '$INSTALL_DIR'
  "
fi

# Генерируем юнит через tee_on_target (умеет работать и локально, и удалённо)
tee_on_target "/etc/systemd/system/$SERVICE_NAME.service" <<EOF
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

run_on_target "sudo systemctl daemon-reload && sudo systemctl enable '$SERVICE_NAME' && sudo systemctl restart '$SERVICE_NAME'"
echo "==> Сервис '$SERVICE_NAME' запущен"

# ---------------------------------------------------------------------------
# Шаг 6. Конфигурация Caddy (HTTPS)
# ---------------------------------------------------------------------------
caddy_present="$(run_on_target 'if command -v caddy >/dev/null 2>&1 || [ -x /usr/bin/caddy ]; then echo yes; else echo no; fi')"

if [[ "$caddy_present" == "yes" ]]; then
  # Если домен не задан — запрашиваем
  if [[ -z "$DOMAIN" ]]; then
    read -r -p "Введите домен для HTTPS (Enter — localhost): " DOMAIN
    DOMAIN="${DOMAIN:-localhost}"
  fi

  echo "==> Прописываю Caddy для $DOMAIN -> 127.0.0.1:$APP_PORT"
  CADDY_CONF="/etc/caddy/Caddyfile"

  # Проверяем, есть ли домен уже в конфиге; если нет — добавляем
  if run_on_target "sudo grep -q '$DOMAIN' '$CADDY_CONF'" 2>/dev/null; then
    echo "Домен $DOMAIN уже есть в $CADDY_CONF — пропускаю."
  else
    tee_on_target -a "$CADDY_CONF" <<EOF

$DOMAIN {
	reverse_proxy 127.0.0.1:$APP_PORT
}
EOF
  fi

  run_on_target "sudo systemctl enable caddy 2>/dev/null || true; sudo systemctl restart caddy 2>/dev/null || true"
  echo "==> Caddy перезапущен. HTTPS: https://$DOMAIN"
else
  echo "(!) Caddy не установлен на цели. Пропускаю настройку HTTPS."
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
