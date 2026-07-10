#!/usr/bin/env sh
set -e

DB_PATH="${LUNAR_DB_PATH:-db/game.db}"

mkdir -p $(dirname $DB_PATH)
goose -dir /opt/lunar-tear/migrations sqlite3 "$DB_PATH" up

AUTH_FLAG=""
if [ -n "${LUNAR_AUTH_URL}" ]; then
  AUTH_FLAG="--auth-url ${LUNAR_AUTH_URL}"
fi

ADMIN_FLAG=""
if [ -n "${LUNAR_ADMIN_LISTEN}" ]; then
  ADMIN_FLAG="--admin-listen ${LUNAR_ADMIN_LISTEN}"
fi

exec /opt/lunar-tear/lunar-tear \
  --listen "${LUNAR_LISTEN:-0.0.0.0:443}" \
  --public-addr "${LUNAR_PUBLIC_ADDR}" \
  --octo-url "${LUNAR_OCTO_URL}" \
  --db "${DB_PATH}" \
  ${AUTH_FLAG} \
  ${ADMIN_FLAG}
