# BKSync

BKSync synchronizes local directories to either an FTP server or another local
directory. It can also remove destination entries that no longer exist in the
source.

## Requirements

- Go 1.27.4 or newer
- For FTP storage, an FTP server reachable with the configured host and port

## Configuration

Edit `config/config.json` before running. For FTP storage:

```json
{
  "storage": "ftp",
  "ftp": {
    "host": "ftp.example.com",
    "port": "21",
    "user": "your-user",
    "password": "your-password"
  },
  "sync": [
    {
      "local": "C:/path/to/source",
      "remote": "/path/to/destination"
    }
  ]
}
```

Set `"storage"` to `"local"` to copy to a destination on the local filesystem;
in that case, `"remote"` is a local destination path. Each item in `"sync"`
defines one source/destination pair.

Keep real credentials out of version control. The FTP settings are not needed
when using local storage.

## Run

The application currently resolves its config, log, and hash-cache paths
relative to the working directory. Run it from `cmd`:

```powershell
Set-Location .\cmd
go run .
```

Logs are written to `logs/` in the project directory. Directory fingerprints
are cached in the project-root `hashes.json`.

## Sync behavior

- Entries whose names start with `.` are skipped.
- Existing files are compared by modification time. A local file with the same
  or earlier modification time is skipped; a newer local file is uploaded.
- If the FTP server does not support modification-time queries, existing files
  are uploaded because BKSync cannot compare their timestamps.
- After syncing each pair, BKSync removes destination entries that are absent
  from the source. Directory removal is recursive. **Review the configured
  paths carefully; this cleanup can delete destination data.**
- Directory fingerprints use visible entry names, file sizes, and modification
  times. They are metadata fingerprints, not content hashes. Existing cache
  entries are readable, but the fingerprint format changed; directories will
  be checked and their cache values refreshed on the next run.

## Development

Run tests and static analysis from the repository root:

```powershell
go test ./...
go vet ./...
```
