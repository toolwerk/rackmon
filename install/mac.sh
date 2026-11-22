#!/usr/bin/env bash
# Installiert den Rackmon-Agenten auf macOS als LaunchDaemon. Als root ausführen, wiederholbar.
#
#   sudo ./mac.sh --binary ./rackmon --server http://server:8470 --token-file ./token
#
# Der Agent läuft als Benutzer "nobody". Das Token liegt in einer Datei, die nur dieser Benutzer liest.
set -euo pipefail

usage() { sed -n '2,6p' "$0" | sed 's/^# \{0,1\}//'; exit 2; }
[ "$(id -u)" = 0 ] || { echo "Als root ausführen (sudo)."; exit 1; }
BIN="" SERVER="" TOKEN_FILE=""
while [ $# -gt 0 ]; do
  case "$1" in
    --binary) BIN="$2"; shift 2 ;;
    --server) SERVER="$2"; shift 2 ;;
    --token-file) TOKEN_FILE="$2"; shift 2 ;;
    *) usage ;;
  esac
done
[ -f "$BIN" ] && [ -n "$SERVER" ] && [ -f "$TOKEN_FILE" ] || usage

LABEL=de.toolwerk.rackmon-agent
PLIST=/Library/LaunchDaemons/$LABEL.plist
install -d -m 0755 /usr/local/bin /usr/local/etc/rackmon /usr/local/var/log
install -m 0755 -o root -g wheel "$BIN" /usr/local/bin/rackmon.new
mv -f /usr/local/bin/rackmon.new /usr/local/bin/rackmon

umask 077
tr -d '\r\n ' < "$TOKEN_FILE" > /usr/local/etc/rackmon/token
chown nobody /usr/local/etc/rackmon/token
chmod 600 /usr/local/etc/rackmon/token

cat > "$PLIST" <<PL
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key><string>$LABEL</string>
  <key>ProgramArguments</key>
  <array>
    <string>/usr/local/bin/rackmon</string>
    <string>agent</string>
    <string>-server</string><string>$SERVER</string>
    <string>-token-file</string><string>/usr/local/etc/rackmon/token</string>
  </array>
  <key>UserName</key><string>nobody</string>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><true/>
  <key>ThrottleInterval</key><integer>5</integer>
  <key>StandardErrorPath</key><string>/usr/local/var/log/rackmon-agent.log</string>
  <key>StandardOutPath</key><string>/usr/local/var/log/rackmon-agent.log</string>
</dict>
</plist>
PL
chown root:wheel "$PLIST"; chmod 644 "$PLIST"
touch /usr/local/var/log/rackmon-agent.log; chown nobody /usr/local/var/log/rackmon-agent.log

launchctl bootout system "$PLIST" 2>/dev/null || true
launchctl bootstrap system "$PLIST"
sleep 2
launchctl print system/$LABEL | sed -n '1,4p;/state =/p;/pid =/p'
