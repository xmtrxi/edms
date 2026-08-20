# Changelog

### Changed

- **Storage**: Migrated document storage from local/in-memory to an S3-compatible backend (MinIO).
- **Backend API**:
  - Wired S3 storage into document upload, download, and delete handlers.
  - Added a new endpoint `GET /api/documents/:id/url` to generate presigned URLs for direct frontend downloads.
  - S3 bucket is automatically created on application startup if it doesn't exist.
- **Configuration**: `STORAGE_PROVIDER` now defaults to `s3`. Removed `LOCAL_STORAGE_PATH` and SharePoint configuration blocks.
- **Docker**: Added `docker-compose.minio.yml` to spin up a local MinIO instance for development.

### Removed

- Removed the `local` and `sharepoint` storage providers entirely from the backend source code.
- Removed the in-memory content storage (`[]byte`) from `storedDocument`.
  s
