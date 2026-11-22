#!/usr/bin/env bash
# Installiert Rackmon als systemd-Dienst (Linux, Raspberry Pi, Proxmox). Als root ausführen, wiederholbar.
#
#   sudo ./linux.sh agent  --binary ./rackmon --server http://server:8470 --token-file ./token
#   sudo ./linux.sh server --binary ./rackmon --config ./server.yaml
#
# Das Token kommt aus einer Datei, nie von der Kommandozeile (dort sähen es andere Benutzer in der Prozessliste).
set -euo pipefail

usage() { sed -n '2,8p' "$0" | sed 's/^# \{0,1\}//'; exit 2; }
[ "$(id -u)" = 0 ] || { echo "Als root ausführen (sudo)."; exit 1; }
MODE="${1:-}"; [ -n "$MODE" ] && shift || usage
BIN="" SERVER="" TOKEN_FILE="" CONFIG=""
while [ $# -gt 0 ]; do
  case "$1" in
    --binary) BIN="$2"; shift 2 ;;
    --server) SERVER="$2"; shift 2 ;;
    --token-file) TOKEN_FILE="$2"; shift 2 ;;
    --config) CONFIG="$2"; shift 2 ;;
    *) usage ;;
  esac
done
[ -f "$BIN" ] || { echo "--binary fehlt oder Datei nicht gefunden"; exit 1; }
HERE="$(cd "$(dirname "$0")" && pwd)"

install -m 0755 -o root -g root "$BIN" /usr/local/bin/rackmon.new
mv -f /usr/local/bin/rackmon.new /usr/local/bin/rackmon
install -d -m 0755 /etc/rackmon

case "$MODE" in
  agent)
    [ -n "$SERVER" ] && [ -f "$TOKEN_FILE" ] || usage
    umask 077
    printf 'RACKMON_SERVER=%s\nRACKMON_TOKEN=%s\n' "$SERVER" "$(tr -d '\r\n ' < "$TOKEN_FILE")" > /etc/rackmon/agent.env
    chmod 600 /etc/rackmon/agent.env
    install -m 0644 "$HERE/rackmon-agent.service" /etc/systemd/system/rackmon-agent.service
    UNIT=rackmon-agent ;;
  server)
    [ -f "$CONFIG" ] || usage
    install -m 0600 -o root -g root "$CONFIG" /etc/rackmon/server.yaml
    install -m 0644 "$HERE/rackmon-server.service" /etc/systemd/system/rackmon-server.service
    UNIT=rackmon-server ;;
  *) usage ;;
esac

systemctl daemon-reload
systemctl enable "$UNIT" >/dev/null 2>&1
systemctl restart "$UNIT"
sleep 1
systemctl is-active "$UNIT"
