#!/bin/sh
set -u

# Mihomo Kit installer for Keenetic + Entware
# Repo layout for one-command install:
#   install.sh
#   bin/mihomo-submanager
# Optional: override BASE_URL before sh, or edit this value after uploading to GitHub.
BASE_URL="${BASE_URL:-https://raw.githubusercontent.com/air404air/mihomo-kit/main}"

MIHOMO_DIR="/opt/etc/mihomo"
UI_DIR="$MIHOMO_DIR/ui"
BACKUP_DIR="$MIHOMO_DIR/backups"
TMP_DIR="/opt/tmp/mihomo-kit-install"

log(){ echo "[mihomo-kit] $*"; }
fail(){ echo "ERROR: $*"; exit 1; }

need_opt(){
  [ -d /opt ] || fail "/opt not found. Install/enable Entware first."
  mkdir -p /opt/bin /opt/etc/init.d "$MIHOMO_DIR" "$UI_DIR" "$BACKUP_DIR" /opt/tmp "$TMP_DIR"
}

fetch(){
  url="$1"; out="$2"
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$url" -o "$out" && return 0
  fi
  wget -qO "$out" "$url"
}

install_deps(){
  if command -v opkg >/dev/null 2>&1; then
    opkg update >/dev/null 2>&1 || true
    opkg install ca-bundle wget-ssl unzip gzip >/dev/null 2>&1 || true
  fi
}

install_mihomo(){
  if [ -x /opt/bin/mihomo ]; then
    log "mihomo already installed: $(/opt/bin/mihomo -v 2>/dev/null | head -n1)"
    return
  fi

  log "mihomo not found, trying to download latest MetaCubeX/mihomo..."
  ARCH="$(uname -m)"
  case "$ARCH" in
    aarch64|arm64) PATTERN='linux-arm64-compatible.*\.gz' ;;
    armv7l|armv7*) PATTERN='linux-armv7.*\.gz' ;;
    armv6l|armv6*) PATTERN='linux-armv6.*\.gz' ;;
    mipsel*) PATTERN='linux-mipsle.*softfloat.*\.gz' ;;
    mips*) PATTERN='linux-mips.*softfloat.*\.gz' ;;
    *) fail "unsupported arch: $ARCH. Put mihomo to /opt/bin/mihomo manually." ;;
  esac

  API="https://api.github.com/repos/MetaCubeX/mihomo/releases/latest"
  URL="$(wget -qO- "$API" | grep 'browser_download_url' | grep -E "$PATTERN" | head -n1 | cut -d '"' -f4)"
  if [ -z "$URL" ]; then
    API="https://api.github.com/repos/MetaCubeX/mihomo/releases"
    URL="$(wget -qO- "$API" | grep 'browser_download_url' | grep -E "$PATTERN" | head -n1 | cut -d '"' -f4)"
  fi
  [ -n "$URL" ] || fail "unable to find mihomo asset for $ARCH"

  log "download: $URL"
  fetch "$URL" "$TMP_DIR/mihomo.gz" || fail "mihomo download failed"
  gzip -dc "$TMP_DIR/mihomo.gz" > /opt/bin/mihomo || fail "unpack mihomo failed"
  chmod +x /opt/bin/mihomo
  log "mihomo installed: $(/opt/bin/mihomo -v 2>/dev/null | head -n1)"
}

install_zashboard(){
  mkdir -p "$UI_DIR" "$TMP_DIR"
  if [ -s "$UI_DIR/index.html" ]; then
    log "Zashboard UI already exists"
    return
  fi

  log "installing Zashboard UI..."
  ZURL="https://github.com/Zephyruso/zashboard/archive/refs/heads/gh-pages-no-fonts.zip"
  rm -rf "$TMP_DIR/zashboard" "$TMP_DIR/zashboard.zip"
  fetch "$ZURL" "$TMP_DIR/zashboard.zip" || { echo "WARNING: zashboard download failed"; return; }
  unzip -q "$TMP_DIR/zashboard.zip" -d "$TMP_DIR/zashboard" || { echo "WARNING: zashboard unzip failed"; return; }
  rm -rf "$UI_DIR"/*
  cp -r "$TMP_DIR"/zashboard/zashboard-gh-pages-no-fonts/* "$UI_DIR"/ 2>/dev/null || true
  [ -s "$UI_DIR/index.html" ] && log "Zashboard installed to $UI_DIR" || echo "WARNING: Zashboard files not found after unzip"
}

ensure_config(){
  CFG="$MIHOMO_DIR/config.yaml"
  if [ ! -s "$CFG" ]; then
    log "creating default mihomo config"
    cat > "$CFG" <<'YAML'
mixed-port: 7890
socks-port: 7891
redir-port: 7892
allow-lan: true
bind-address: '*'
mode: rule
log-level: info
ipv6: false
external-controller: 0.0.0.0:9090
secret: ''
external-ui: /opt/etc/mihomo/ui
geodata-mode: true
geo-auto-update: true
geo-update-interval: 24

proxies: []
proxy-groups:
  - name: PROXY
    type: select
    proxies:
      - DIRECT
rules:
  - GEOSITE,private,DIRECT
  - GEOIP,private,DIRECT
  - GEOIP,RU,DIRECT
  - MATCH,DIRECT
YAML
    return
  fi

  cp "$CFG" "$BACKUP_DIR/config.before-kit.$(date +%Y%m%d-%H%M%S).yaml" 2>/dev/null || true
  sed -i \
    -e '/^mixed-port:/d' \
    -e '/^socks-port:/d' \
    -e '/^redir-port:/d' \
    -e '/^allow-lan:/d' \
    -e '/^bind-address:/d' \
    -e '/^external-controller:/d' \
    -e '/^secret:/d' \
    -e '/^external-ui:/d' "$CFG"

  cat > "$TMP_DIR/config.head" <<'YAML'
mixed-port: 7890
socks-port: 7891
redir-port: 7892
allow-lan: true
bind-address: '*'
external-controller: 0.0.0.0:9090
secret: ''
external-ui: /opt/etc/mihomo/ui
YAML
  cat "$TMP_DIR/config.head" "$CFG" > "$TMP_DIR/config.yaml"
  mv "$TMP_DIR/config.yaml" "$CFG"
}

install_mihomo_init(){
  cat > /opt/etc/init.d/S99mihomo <<'EOF2'
#!/bin/sh
ENABLED=yes
PROCS=mihomo
ARGS="-d /opt/etc/mihomo -f /opt/etc/mihomo/config.yaml"
PREARGS=""
DESC="mihomo core"
PATH=/opt/sbin:/opt/bin:/usr/sbin:/usr/bin:/sbin:/bin
. /opt/etc/init.d/rc.func
EOF2
  chmod +x /opt/etc/init.d/S99mihomo
}

install_redirect(){
  cat > /opt/etc/init.d/S98mihomo_redirect <<'EOF2'
#!/bin/sh
ENABLED=yes
DESC="mihomo transparent redirect"
PATH=/opt/sbin:/opt/bin:/usr/sbin:/usr/bin:/sbin:/bin
LAN_IF_FILE="/opt/etc/mihomo/lan_if"
CHAIN="MIHOMO_REDIRECT"
REDIR_PORT="7892"

detect_lan_if() {
    if [ -s "$LAN_IF_FILE" ]; then cat "$LAN_IF_FILE"; return; fi
    IFACE="$(ip -4 addr show | awk '/^[0-9]+: br[0-9]+:/ {i=$2; gsub(":","",i)} /inet 192\.168\./ {print i; exit}')"
    [ -z "$IFACE" ] && IFACE="br0"
    echo "$IFACE" > "$LAN_IF_FILE"
    echo "$IFACE"
}

stop_rules() {
    LAN_IF="$(detect_lan_if)"
    iptables -t nat -D PREROUTING -i "$LAN_IF" -p tcp -j "$CHAIN" 2>/dev/null
    iptables -t nat -D PREROUTING -i br0 -p tcp -j "$CHAIN" 2>/dev/null
    iptables -t nat -D PREROUTING -i br1 -p tcp -j "$CHAIN" 2>/dev/null
    iptables -t nat -F "$CHAIN" 2>/dev/null
    iptables -t nat -X "$CHAIN" 2>/dev/null
}

start_rules() {
    LAN_IF="$(detect_lan_if)"
    stop_rules
    iptables -t nat -N "$CHAIN"
    iptables -t nat -A "$CHAIN" -d 0.0.0.0/8 -j RETURN
    iptables -t nat -A "$CHAIN" -d 10.0.0.0/8 -j RETURN
    iptables -t nat -A "$CHAIN" -d 100.64.0.0/10 -j RETURN
    iptables -t nat -A "$CHAIN" -d 127.0.0.0/8 -j RETURN
    iptables -t nat -A "$CHAIN" -d 169.254.0.0/16 -j RETURN
    iptables -t nat -A "$CHAIN" -d 172.16.0.0/12 -j RETURN
    iptables -t nat -A "$CHAIN" -d 192.168.0.0/16 -j RETURN
    iptables -t nat -A "$CHAIN" -d 224.0.0.0/4 -j RETURN
    iptables -t nat -A "$CHAIN" -p tcp -j REDIRECT --to-ports "$REDIR_PORT"
    iptables -t nat -I PREROUTING 1 -i "$LAN_IF" -p tcp -j "$CHAIN"
    echo "OK: mihomo redirect enabled on $LAN_IF -> $REDIR_PORT"
}

case "$1" in
    start) start_rules ;;
    stop) stop_rules; echo "OK: mihomo redirect disabled" ;;
    restart) stop_rules; start_rules ;;
    status) echo "LAN_IF=$(detect_lan_if)"; iptables -t nat -L PREROUTING -n -v | head -20; echo; iptables -t nat -L "$CHAIN" -n -v 2>/dev/null || echo "not enabled" ;;
    *) echo "Usage: $0 {start|stop|restart|status}"; exit 1 ;;
esac
EOF2
  chmod +x /opt/etc/init.d/S98mihomo_redirect
}

install_submanager(){
  log "installing SubManager on :9091"
  fetch "$BASE_URL/bin/mihomo-submanager" /opt/bin/mihomo-submanager || fail "download mihomo-submanager failed. Check BASE_URL=$BASE_URL"
  chmod +x /opt/bin/mihomo-submanager
  cat > /opt/etc/init.d/S97mihomo_submanager <<'EOF2'
#!/bin/sh
ENABLED=yes
PROCS=mihomo-submanager
ARGS=""
PREARGS=""
DESC="Mihomo SubManager"
PATH=/opt/sbin:/opt/bin:/usr/sbin:/usr/bin:/sbin:/bin
. /opt/etc/init.d/rc.func
EOF2
  chmod +x /opt/etc/init.d/S97mihomo_submanager
}

start_all(){
  log "testing config"
  /opt/bin/mihomo -t -d /opt/etc/mihomo -f /opt/etc/mihomo/config.yaml || true
  /opt/etc/init.d/S99mihomo restart || true
  /opt/etc/init.d/S98mihomo_redirect restart || true
  /opt/etc/init.d/S97mihomo_submanager restart || true
}

need_opt
install_deps
install_mihomo
install_zashboard
ensure_config
install_mihomo_init
install_redirect
install_submanager
start_all

LAN_IF="$(cat /opt/etc/mihomo/lan_if 2>/dev/null || echo br0)"
cat <<EOF2

DONE.

Zashboard:   http://ROUTER_IP:9090/ui/
SubManager:  http://ROUTER_IP:9091/
LAN redirect: $LAN_IF -> 7892

After install: open SubManager and paste subscription URL.
EOF2
