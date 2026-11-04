#!/usr/bin/env bash
# Back up EDMS: the SurrealDB database and the stored files.
#
# Usage: scripts/backup.sh [backup-dir]
#
# Reads the same variables as the API (SURREALDB_URL, SURREALDB_USER,
# SURREALDB_PASS, SURREALDB_NAMESPACE, SURREALDB_DATABASE, STORAGE_PROVIDER,
# LOCAL_STORAGE_PATH, S3_BUCKET, S3_ENDPOINT). Set BACKUP_RETENTION_DAYS to
# delete older backups from the backup directory (default 30).
#
# Needs: surreal (CLI matching the server version), and for S3 storage the
# aws CLI. Valkey/Redis and Meilisearch are not backed up: queued tasks are
# retried from the database by "Retry stuck work", and the search index is
# rebuilt with POST /api/search/reindex.
set -euo pipefail

dest_root="${1:-${BACKUP_DIR:-./backups}}"
stamp="$(date -u +%Y%m%dT%H%M%SZ)"
dest="$dest_root/edms-$stamp"
mkdir -p "$dest"

: "${SURREALDB_URL:?set SURREALDB_URL}"
ns="${SURREALDB_NAMESPACE:-edms}"
db="${SURREALDB_DATABASE:-edms}"
# The CLI wants an http(s) endpoint; the API may be configured with ws(s)/rpc.
endpoint="$(printf '%s' "$SURREALDB_URL" | sed -e 's#^ws://#http://#' -e 's#^wss://#https://#' -e 's#/rpc/*$##')"

echo "Exporting SurrealDB $ns/$db from $endpoint"
surreal export --endpoint "$endpoint" --username "${SURREALDB_USER:-root}" --password "${SURREALDB_PASS:-root}" \
  --namespace "$ns" --database "$db" "$dest/database.surql"
gzip -9 "$dest/database.surql"

case "${STORAGE_PROVIDER:-local}" in
  local)
    : "${LOCAL_STORAGE_PATH:?set LOCAL_STORAGE_PATH}"
    echo "Archiving files from $LOCAL_STORAGE_PATH"
    tar -C "$LOCAL_STORAGE_PATH" -czf "$dest/files.tar.gz" .
    ;;
  s3)
    : "${S3_BUCKET:?set S3_BUCKET}"
    echo "Copying files from s3://$S3_BUCKET"
    aws s3 sync ${S3_ENDPOINT:+--endpoint-url "$S3_ENDPOINT"} "s3://$S3_BUCKET" "$dest/files" --only-show-errors
    tar -C "$dest" -czf "$dest/files.tar.gz" files && rm -rf "$dest/files"
    ;;
  sharepoint)
    echo "Files live in SharePoint; rely on Microsoft 365 retention and backup for them."
    ;;
  *)
    echo "Unknown STORAGE_PROVIDER ${STORAGE_PROVIDER}; files were not backed up" >&2
    ;;
esac

(cd "$dest" && sha256sum ./* > SHA256SUMS)
cat > "$dest/MANIFEST" <<MANIFEST
created_at=$stamp
namespace=$ns
database=$db
storage_provider=${STORAGE_PROVIDER:-local}
MANIFEST

retention="${BACKUP_RETENTION_DAYS:-30}"
find "$dest_root" -maxdepth 1 -type d -name 'edms-*' -mtime "+$retention" -exec rm -rf {} +
echo "Backup written to $dest"
