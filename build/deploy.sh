#!/usr/bin/env bash
# Builds the plugin and uploads it to a Mattermost server over the REST API — the scripted
# equivalent of `make deploy` that also handles what pluginctl cannot:
#   - resolves http->https redirects (a reverse proxy would otherwise swallow the login POST)
#   - accepts MM_ADMIN_EMAIL for servers that disable username sign-in
#   - accepts a personal access token (MM_ADMIN_TOKEN), which works with MFA/SSO-only servers
#
# Environment:
#   MM_SERVICESETTINGS_SITEURL                            e.g. https://mattermost.example.com
#   MM_ADMIN_TOKEN  _or_  MM_ADMIN_EMAIL|MM_ADMIN_USERNAME + MM_ADMIN_PASSWORD
#       (a system administrator; plugin uploads must be enabled in the System Console)
#   SKIP_BUILD=1   optional; upload the existing dist bundle without rebuilding
#
# Prerequisites: jq and curl. Nothing is deleted server-side; re-running is safe.

set -euo pipefail

PLUGIN_ID="$(jq -r .id plugin.json)"
VERSION="$(jq -r .version plugin.json)"
BUNDLE="dist/${PLUGIN_ID}-${VERSION}.tar.gz"

log() { printf '\n\033[1;34m==> %s\033[0m\n' "$*"; }
die() { printf '\033[1;31mError: %s\033[0m\n' "$*" >&2; exit 1; }

command -v jq >/dev/null || die "jq is required"
command -v curl >/dev/null || die "curl is required"

SITE_URL="${MM_SERVICESETTINGS_SITEURL:?MM_SERVICESETTINGS_SITEURL is not set}"
HEADERS_FILE=$(mktemp)
trap 'rm -f "$HEADERS_FILE"' EXIT

# A reverse proxy often redirects http to https; a login or upload POST that curl turns into a
# GET would fail confusingly. Resolve the real URL with an unauthenticated ping first and hand
# it to everything that follows.
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

# Either a personal access token, or a password login whose Token response header carries the
# session. Both authenticate /api/v4 and the plugin routes as a bearer token.
admin_login() {
    local status

    if [ -n "${MM_ADMIN_TOKEN:-}" ]; then
        TOKEN="$MM_ADMIN_TOKEN"
        log "Using MM_ADMIN_TOKEN (personal access token)"
    else
        local login_id="${MM_ADMIN_EMAIL:-${MM_ADMIN_USERNAME:?MM_ADMIN_USERNAME (or MM_ADMIN_EMAIL) is not set}}"
        : "${MM_ADMIN_PASSWORD:?MM_ADMIN_PASSWORD is not set}"

        log "Logging in as $login_id against $SITE_URL"
        status="$(curl -sS -L --post301 --post302 --post303 -D "$HEADERS_FILE" -o /dev/null \
            -w '%{http_code}' -H 'Content-Type: application/json' \
            -d "$(jq -n --arg id "$login_id" --arg pw "$MM_ADMIN_PASSWORD" '{login_id: $id, password: $pw}')" \
            "$SITE_URL/api/v4/users/login")" ||
            die "the login request failed against $SITE_URL"
        if [ "$status" = "401" ]; then
            die "Mattermost rejected the credentials (HTTP 401):
  - check the password (watch for \$ or quotes mangled by the shell in .env)
  - if username sign-in is disabled on this server, set MM_ADMIN_EMAIL instead
  - if the account uses MFA or SSO sign-in, create a personal access token
    (Account Settings -> Security) and export MM_ADMIN_TOKEN"
        fi
        [ "$status" = "200" ] || die "login answered HTTP $status"

        TOKEN="$(awk 'tolower($1) == "token:" {gsub(/\r/, ""); print $2}' "$HEADERS_FILE")"
        if [ -z "$TOKEN" ]; then
            sed 's/^/    /' "$HEADERS_FILE"
            die "login returned no session token; check that $SITE_URL reaches Mattermost directly"
        fi
    fi

    # Verify the session before uploading anything.
    status="$(curl -sS -o /dev/null -w '%{http_code}' \
        -H "Authorization: Bearer $TOKEN" "$SITE_URL/api/v4/users/me")" ||
        die "the session check request failed"
    [ "$status" = "200" ] || die "the token was rejected by /api/v4/users/me (HTTP $status)"
}

build_bundle() {
    if [ "${SKIP_BUILD:-0}" = "1" ]; then
        [ -f "$BUNDLE" ] || die "SKIP_BUILD=1 but $BUNDLE does not exist"
        log "Using the existing $BUNDLE"
        return
    fi
    log "Building $BUNDLE"
    make dist
    [ -f "$BUNDLE" ] || die "make dist did not produce $BUNDLE"
}

upload_plugin() {
    log "Uploading $BUNDLE to $SITE_URL"
    local status
    status="$(curl -sS -o /dev/null -w '%{http_code}' \
        -H "Authorization: Bearer $TOKEN" \
        -F force=true \
        -F "plugin=@$BUNDLE" \
        "$SITE_URL/api/v4/plugins")" ||
        die "the upload request failed"
    case "$status" in
    200 | 201) ;;
    403) die "upload answered HTTP 403 — the account is not a system administrator" ;;
    400) die "upload answered HTTP 400 — plugin uploads may be disabled (System Console -> Plugins -> Plugin Management -> Allow File Uploads)" ;;
    *) die "upload answered HTTP $status" ;;
    esac

    log "Enabling $PLUGIN_ID"
    status="$(curl -sS -o /dev/null -w '%{http_code}' \
        -H "Authorization: Bearer $TOKEN" \
        -X POST "$SITE_URL/api/v4/plugins/$PLUGIN_ID/enable")" ||
        die "the enable request failed"
    [ "$status" = "200" ] || die "enabling the plugin answered HTTP $status"
}

verify_plugin() {
    log "Verifying the plugin API"
    local status
    status="$(curl -sS -o /dev/null -w '%{http_code}' \
        -H "Authorization: Bearer $TOKEN" \
        "$SITE_URL/plugins/$PLUGIN_ID/api/v1/config")" ||
        die "the verify request failed"
    [ "$status" = "200" ] || die "the plugin API answered HTTP $status after enabling — check the plugin logs"
}

echo "Deploying $PLUGIN_ID v$VERSION"
resolve_site_url
admin_login
build_bundle
upload_plugin
verify_plugin

log "Deployed $PLUGIN_ID v$VERSION to $SITE_URL"
echo "Reload the webapp (hard refresh) to pick up the new JavaScript bundle."
