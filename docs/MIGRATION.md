# Migrating from the `mattermost-tasks` plugin ID

The plugin was renamed from `com.github.3tty0n.mattermost-tasks` to
`com.github.3tty0n.mattermost-agenda` (v0.3.0). The Mattermost plugin KV store namespaces records by
plugin ID, so the new plugin cannot see the old one's data. This is a one-time migration:
the old install **exports** every record to a JSON file, and the new install **imports** it.

## One-command migration

`build/migrate-to-agenda.sh` performs every step below: it builds the old-ID migration bundle,
deploys it, exports all records, deploys the new plugin, imports the records, and disables the
old plugin.

```bash
export MM_SERVICESETTINGS_SITEURL=http://localhost:8065
export MM_ADMIN_USERNAME=sysadmin
export MM_ADMIN_PASSWORD=secret
./build/migrate-to-agenda.sh
```

After it finishes: verify your tasks and events in Agenda, delete the old `Tasks` plugin in the
System Console, deactivate the `tasks-bot` user, and remove the `agenda-export.json` it wrote.

## What the script does (manual equivalent)

1. **Upgrade the old plugin to the migration build.** The running install predates the export
   endpoint, so deploy a bundle that still uses the old ID but contains it — build it once with:

   ```bash
   sed -i.bak 's/mattermost-agenda/mattermost-tasks/' plugin.json
   make apply dist   # dist/com.github.3tty0n.mattermost-tasks-<version>.tar.gz
   mv -f plugin.json.bak plugin.json && make apply
   ./build/bin/pluginctl deploy com.github.3tty0n.mattermost-tasks dist/com.github.3tty0n.mattermost-tasks-<version>.tar.gz
   ```

   Uploading the same ID with a newer version replaces the code, not the data.

2. **Export.** The script logs in as the admin and downloads every record with the session's
   bearer token (see "How the script authenticates" below):

   ```bash
   curl -H "Authorization: Bearer $TOKEN" \
     "$SITE_URL/plugins/com.github.3tty0n.mattermost-tasks/api/v1/admin/export" \
     -o agenda-export.json
   ```

3. **Deploy the new plugin.**

   ```bash
   ./build/bin/pluginctl deploy com.github.3tty0n.mattermost-agenda dist/com.github.3tty0n.mattermost-agenda-<version>.tar.gz
   ```

4. **Import.**

   ```bash
   curl -X POST "$SITE_URL/plugins/com.github.3tty0n.mattermost-agenda/api/v1/admin/import" \
     -H "Authorization: Bearer $TOKEN" \
     -H 'Content-Type: application/json' \
     --data @agenda-export.json
   ```

   The response is `{"written": N, "skipped": M}`. Existing keys are skipped, so re-running the
   import never overwrites data created after the switch.

5. **Clean up.** Disable the old plugin (`./build/bin/pluginctl disable
   com.github.3tty0n.mattermost-tasks`), then delete it and deactivate the `tasks-bot` user in the
   System Console; the new install creates `agenda-bot`.

## How the script authenticates

`POST /api/v4/users/login` returns the **session token in its `Token` response header** — the same
value the webapp keeps in the `MMAUTHTOKEN` cookie. Sending it as `Authorization: Bearer <token>`
authenticates both `/api/v4` and plugin routes, and **no CSRF header is needed**: CSRF guards
cookie-authenticated requests only. That is what the migration script uses, and what this repo's
integration tests do to call plugin endpoints.

```bash
# Login and take the session token from the response headers.
curl -sS -D headers.txt -H 'Content-Type: application/json' \
  -d '{"login_id":"sysadmin","password":"secret"}' \
  "$SITE_URL/api/v4/users/login" -o /dev/null
TOKEN="$(awk 'tolower($1)=="token:" {gsub(/\r/,""); print $2}' headers.txt)"

# Then the plugin API needs no cookies at all.
curl -H "Authorization: Bearer $TOKEN" \
  "$SITE_URL/plugins/com.github.3tty0n.mattermost-tasks/api/v1/admin/export" -o agenda-export.json
```

If the login returns no `Token` header, the URL is usually being redirected (http→https, or a
trailing-slash redirect) or a reverse proxy is swallowing response headers — point
`MM_SERVICESETTINGS_SITEURL` at the URL the browser actually uses.

### Browser fallback (cookies)

Prefer trying from a browser session instead? The webapp authenticates with the `MMAUTHTOKEN`
cookie plus the `MMCSRF` cookie in an `X-CSRF-Token` header on POSTs. Both are `HttpOnly`; copy
them from devtools → Application (Firefox: Storage) → Cookies → the site's origin, or simpler:
devtools → Network → any request → "Copy as cURL" and swap in the export/import URL — the cookies
and header travel with the copied command.

A **personal access token** (`MM_ADMIN_TOKEN`, Account Settings → Security → Personal Access
Tokens) authenticates `make deploy`'s Client4 calls and works as a bearer token for `/api/v4`,
but a *session* token (login) is what the script uses so the import's audit trail names a real
session.

## Notes

- The export file contains every user's task and event content. Treat it as sensitive.
- Reminders are re-queued from the imported records on the next save of each item; already-fired
  buckets are ignored by the stale check.
- Both endpoints are system-administrator only and write an audit record.
