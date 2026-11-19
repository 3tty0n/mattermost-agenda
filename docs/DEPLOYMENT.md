# Deployment

## Requirements

- Mattermost 11.7.0 or newer. Prebuilt releases target **linux-amd64** by default; a maintainer
  can explicitly publish a bundle with **darwin-amd64**/**darwin-arm64** macOS binaries. Other
  server OS/architectures must build from source.
- No external database or service. The plugin uses the Mattermost plugin KV store.
- No special outbound network access.

## Install

1. Build or download the bundle: `dist/com.github.3tty0n.mattermost-agenda-<version>.tar.gz`.
   Release bundles are stripped (`-s -w`). The default release contains one linux-amd64 server
   binary; macOS binaries are included only in a release explicitly built with `macos=true`.
2. **System Console → Plugins → Plugin Management → Upload Plugin**.
3. Enable **Tasks**.
4. Optionally disable assignment notifications in the plugin settings.

## Upgrade

Upload the new bundle and enable it. Task records and indexes are stored under the plugin ID; no
database migration is required. Restarting the plugin or Mattermost does not clear data.

## Automatic deploy

`build/deploy.sh` (or `make deploy-auto`) builds the bundle and uploads it over the REST API —
the same upload `make deploy` uses, plus redirect resolution and flexible admin login:

```bash
export MM_SERVICESETTINGS_SITEURL=https://mattermost.example.com
export MM_ADMIN_TOKEN=...        # or MM_ADMIN_EMAIL + MM_ADMIN_PASSWORD
./build/deploy.sh
```

The account must be a system administrator and plugin uploads must be enabled. `SKIP_BUILD=1`
re-uploads the existing `dist` bundle without rebuilding; re-running is otherwise safe (uploads
are forced, the plugin is re-enabled, and its API is verified at the end).

## Rollback

Disable the plugin. Data remains in the plugin KV store. Re-enabling the same or a newer bundle
continues to use it.

## Backup

Tasks live in the Mattermost database's plugin KV table. Use the normal Mattermost database backup
procedure; there is no separate volume or file store.

## Resource use

The plugin has one RHS, one notification worker and no outbound calls. List requests read up to
four index shards per scope, making it suitable for a 1 vCPU / 512 MB app node.
