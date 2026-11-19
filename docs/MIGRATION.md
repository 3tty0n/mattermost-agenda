# Migrating from the `mattermost-tasks` plugin ID

The plugin was renamed from `com.github.3tty0n.mattermost-tasks` to
`com.github.3tty0n.mattermost-agenda` (v0.3.0). The Mattermost plugin KV store namespaces records by
plugin ID, so the new plugin cannot see the old one's data. This is a one-time migration:
the old install **exports** every record to a JSON file, and the new install **imports** it.

## Steps

1. **Upgrade the old plugin to the migration build.** Install the last release that still uses the
   old ID — v0.3.0 or later bundles contain the export endpoint. If the running install is older,
   install the `mattermost-tasks`-ID bundle of v0.3.0 over it (same ID upgrades keep data) — build
   it once with:

   ```bash
   cp plugin.json plugin.json.bak
   sed -i 's/mattermost-agenda/mattermost-tasks/' plugin.json
   make apply dist   # dist/com.github.3tty0n.mattermost-tasks-<version>.tar.gz
   mv plugin.json.bak plugin.json && make apply
   ```

   Upload that bundle in **System Console → Plugins → Plugin Management** (uploading the same ID
   with a newer version replaces the code, not the data), then **disable** the plugin.

2. **Export.** Signed in as a system administrator, open
   `<SiteURL>/plugins/com.github.3tty0n.mattermost-tasks/api/v1/admin/export`
   in the browser. Save the downloaded `agenda-export.json`.

3. **Install the new plugin.** Upload
   `dist/com.github.3tty0n.mattermost-agenda-<version>.tar.gz` and enable it.

4. **Import.** With the admin's session cookies and CSRF token (or the browser devtools, replaying a
   `fetch` from the webapp):

   ```bash
   curl -X POST '<SiteURL>/plugins/com.github.3tty0n.mattermost-agenda/api/v1/admin/import' \
     -H 'X-Requested-With: XMLHttpRequest' \
     -H 'X-CSRF-Token: <MMCSRF cookie value>' \
     -H 'Content-Type: application/json' \
     -b 'MMCSRF=<cookie>; MMAUTHTOKEN=<cookie>' \
     --data @agenda-export.json
   ```

   The response is `{"written": N, "skipped": M}`. Existing keys are skipped, so re-running the
   import never overwrites data created after the switch.

5. **Clean up.** Disable and delete the old `Tasks` plugin in the System Console. Its bot user
   `tasks-bot` can be deactivated; the new install creates `agenda-bot`.

## Notes

- The export file contains every user's task and event content. Treat it as sensitive.
- Reminders are re-queued from the imported records on the next save of each item; already-fired
  buckets are ignored by the stale check.
- Both endpoints are system-administrator only and write an audit record.
