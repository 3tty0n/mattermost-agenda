#!/usr/bin/env bash
# Migrates a Mattermost server from the old Tasks plugin (com.github.3tty0n.mattermost-tasks)
# to the renamed Agenda plugin (com.github.3tty0n.mattermost-agenda).
#
# The plugin KV store namespaces records by plugin ID, so the new plugin cannot read the old
# one's data. This script deploys a migration build under the OLD ID, exports every record,
# deploys the NEW plugin, imports the records, and cleans up.
#
# Prerequisites (same as `make deploy`):
#   - MM_SERVICESETTINGS_SITEURL          e.g. http://localhost:8065
#   - MM_ADMIN_USERNAME + MM_ADMIN_PASSWORD   (a system administrator)
#   - jq and curl installed
#
# Authentication: the login response carries the session token in its Token header (the same
# value the webapp keeps in the MMAUTHTOKEN cookie). With "Authorization: Bearer <token>" the
# plugin API authenticates without cookies, and CSRF does not apply — that is how this repo's
# integration tests call plugin endpoints too.
#
# Notes:
#   - The export contains every user's task and event content. Treat the output file as
#     sensitive and delete it once the migration is verified.
#   - Importing skips keys that already exist, so re-running the import is safe.
#   - The bot username cannot be renamed; the old install keeps tasks-bot, the new one creates
#     agenda-bot. Deactivate tasks-bot manually after the cleanup step.

set -euo pipefail

OLD_ID="com.github.3tty0n.mattermost-tasks"
NEW_ID="com.github.3tty0n.mattermost-agenda"
VERSION="$(jq -r .version plugin.json)"
OLD_BUNDLE="dist/${OLD_ID}-${VERSION}.tar.gz"
NEW_BUNDLE="dist/${NEW_ID}-${VERSION}.tar.gz"
EXPORT_FILE="${EXPORT_FILE:-agenda-export.json}"
PLUGINCTL=./build/bin/pluginctl

log() { printf '\n\033[1;34m==> %s\033[0m\n' "$*"; }
die() { printf '\033[1;31mError: %s\033[0m\n' "$*" >&2; exit 1; }

command -v jq >/dev/null || die "jq is required"
command -v curl >/dev/null || die "curl is required"

# The admin session: logging in returns the session token in the Token response header. The
# plugin API accepts it as a bearer token, so no cookies or CSRF header are needed.
SITE_URL="${MM_SERVICESETTINGS_SITEURL:?MM_SERVICESETTINGS_SITEURL is not set}"
HEADERS_FILE=$(mktemp)
trap 'rm -f "$HEADERS_FILE"' EXIT

# A reverse proxy often redirects http to https; curl treats the 3xx as success and the login
# would find no token header. Resolve the real URL with an unauthenticated ping first.
resolve_site_url() {
    log "Checking $SITE_URL"
    local probe status effective
    probe="$(curl -sS -L -o /dev/null -w '%{http_code} %{url_effective}' "$SITE_URL/api/v4/system/ping")" ||
        die "cannot reach $SITE_URL ($SITE_URL/api/v4/system/ping)"
    status="${probe%% *}"
    effective="${probe#* }"
    [ "$status" = "200" ] || die "Mattermost at $SITE_URL answered HTTP $status to /api/v4/system/ping"
    effective="${effective%/api/v4/system/ping}"
    if [ "$effective" != "$SITE_URL" ]; then
        log "Following the redirect: using $effective"
        SITE_URL="$effective"
    fi
}

admin_login() {
    : "${MM_ADMIN_USERNAME:?MM_ADMIN_USERNAME is not set}"
    : "${MM_ADMIN_PASSWORD:?MM_ADMIN_PASSWORD is not set}"

    log "Logging in as $MM_ADMIN_USERNAME against $SITE_URL"
    local status
    status="$(curl -sS -L --post301 --post302 --post303 -D "$HEADERS_FILE" -o /dev/null \
        -w '%{http_code}' -H 'Content-Type: application/json' \
        -d "{\"login_id\":\"$MM_ADMIN_USERNAME\",\"password\":\"$MM_ADMIN_PASSWORD\"}" \
        "$SITE_URL/api/v4/users/login")" ||
        die "the login request failed against $SITE_URL"
    [ "$status" = "200" ] || die "login answered HTTP $status — check the admin credentials"

    TOKEN="$(awk 'tolower($1) == "token:" {gsub(/\r/, ""); print $2}' "$HEADERS_FILE")"
    if [ -z "$TOKEN" ]; then
        sed 's/^/    /' "$HEADERS_FILE"
        die "login returned no session token; check that $SITE_URL reaches Mattermost directly"
    fi

    # Verify the session before touching any plugin data.
    status="$(curl -sS -o /dev/null -w '%{http_code}' \
        -H "Authorization: Bearer $TOKEN" "$SITE_URL/api/v4/users/me")" ||
        die "the session check request failed"
    [ "$status" = "200" ] || die "the session token was rejected by /api/v4/users/me (HTTP $status)"
}

build_old_bundle() {
    log "Building the migration bundle under the old ID ($OLD_ID, v$VERSION)"
    [ -f "$NEW_BUNDLE" ] || make dist
    [ -f "$OLD_BUNDLE" ] || {
        sed -i.bak "s/$NEW_ID/$OLD_ID/" plugin.json
        trap 'mv -f plugin.json.bak plugin.json' EXIT
        make apply dist
        mv -f plugin.json.bak plugin.json
        trap - EXIT
    }
}

echo "Agenda migration: $OLD_ID -> $NEW_ID (v$VERSION)"
resolve_site_url
admin_login
build_old_bundle

log "Step 1/5: deploying the migration build under the old plugin ID"
"$PLUGINCTL" deploy "$OLD_ID" "$OLD_BUNDLE"

log "Step 2/5: exporting every record from the old namespace"
curl -Ssf -H "Authorization: Bearer $TOKEN" \
    -H 'X-Requested-With: XMLHttpRequest' \
    "$SITE_URL/plugins/$OLD_ID/api/v1/admin/export" -o "$EXPORT_FILE"
jq -e '.version == 1' "$EXPORT_FILE" >/dev/null || die "unexpected export format in $EXPORT_FILE"
echo "Wrote $EXPORT_FILE ($(jq '.keys | length' "$EXPORT_FILE") records)"

log "Step 3/5: deploying the new plugin ($NEW_ID)"
"$PLUGINCTL" deploy "$NEW_ID" "$NEW_BUNDLE"

log "Step 4/5: importing the records into the new namespace"
curl -Ssf -X POST -H "Authorization: Bearer $TOKEN" \
    -H 'X-Requested-With: XMLHttpRequest' \
    -H 'Content-Type: application/json' \
    --data @"$EXPORT_FILE" \
    "$SITE_URL/plugins/$NEW_ID/api/v1/admin/import"

log "Step 5/5: disabling the old plugin (delete it and tasks-bot in the System Console)"
"$PLUGINCTL" disable "$OLD_ID"

log "Done. Open Agenda and verify your tasks and events, then:"
echo "  - delete the old Tasks plugin in System Console -> Plugins"
echo "  - deactivate the old tasks-bot user (Agenda uses agenda-bot)"
echo "  - remove $EXPORT_FILE; it contains every user's data"
