#!/bin/sh
set -eu

PUID="${PUID:-0}"
PGID="${PGID:-0}"

case "$PUID" in
  ''|*[!0-9]*) echo "PUID 必须是非负整数，当前值：$PUID" >&2; exit 2 ;;
esac
case "$PGID" in
  ''|*[!0-9]*) echo "PGID 必须是非负整数，当前值：$PGID" >&2; exit 2 ;;
esac

if { [ "$PUID" -eq 0 ] && [ "$PGID" -ne 0 ]; } || { [ "$PUID" -ne 0 ] && [ "$PGID" -eq 0 ]; }; then
  echo "PUID 和 PGID 必须同时为 0（root）或同时为非零整数" >&2
  exit 2
fi

mkdir -p /data

if [ "$PUID" -eq 0 ]; then
  exec "$@"
fi

chown -R "$PUID:$PGID" /data

exec su-exec "$PUID:$PGID" "$@"
