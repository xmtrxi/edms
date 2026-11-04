#!/usr/bin/env bash
# Restore an EDMS backup made by scripts/backup.sh into an EMPTY database
# and storage location. Stop the API and workers first.
#
# Usage: scripts/restore.sh <backup-dir> [--yes]
set -euo pipefail

src="${1:?usage: scripts/restore.sh <backup-dir> [--yes]}"
[[ -f "$src/database.surql.gz" ]] || { echo "No database.surql.gz in $src" >&2; exit 1; }
(cd "$src" && sha256sum --check --quiet SHA256SUMS) || { echo "Checksums do not match; refusing to restore" >&2; exit 1; }

: "${SURREALDB_URL:?set SURREALDB_URL}"
ns="${SURREALDB_NAMESPACE:-edms}"
db="${SURREALDB_DATABASE:-edms}"
endpoint="$(printf '%s' "$SURREALDB_URL" | sed -e 's#^ws://#http://#' -e 's#^wss://#https://#' -e 's#/rpc/*$##')"

if [[ "${2:-}" != "--yes" ]]; then
  echo "This imports $src into $ns/$db at $endpoint and ${STORAGE_PROVIDER:-local} storage."
  read -r -p "Type RESTORE to continue: " answer
  [[ "$answer" == "RESTORE" ]] || { echo "Cancelled"; exit 1; }
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
gunzip -c "$src/database.surql.gz" > "$tmp/database.surql"
echo "Importing database"
surreal import --endpoint "$endpoint" --username "${SURREALDB_USER:-root}" --password "${SURREALDB_PASS:-root}" \
  --namespace "$ns" --database "$db" "$tmp/database.surql"

if [[ -f "$src/files.tar.gz" ]]; then
  case "${STORAGE_PROVIDER:-local}" in
    local)
      : "${LOCAL_STORAGE_PATH:?set LOCAL_STORAGE_PATH}"
      mkdir -p "$LOCAL_STORAGE_PATH"
      tar -C "$LOCAL_STORAGE_PATH" -xzf "$src/files.tar.gz"
      ;;
    s3)
      : "${S3_BUCKET:?set S3_BUCKET}"
      tar -C "$tmp" -xzf "$src/files.tar.gz"
      aws s3 sync ${S3_ENDPOINT:+--endpoint-url "$S3_ENDPOINT"} "$tmp/files" "s3://$S3_BUCKET" --only-show-errors
      ;;
  esac
fi
echo "Restored. Start the API, then rebuild search with POST /api/search/reindex."
