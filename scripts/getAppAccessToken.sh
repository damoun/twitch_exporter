#!/usr/bin/env bash

# always healthy to set -e
set -e

tokenEndpoint="https://id.twitch.tv/oauth2/token"
configPath="${1}"

help() {
    echo "Usage: ./scripts/getAppAccessToken.sh <config-path>"
    echo "  Ensure that your config file is also properly structured, see README.md"
}

# jq and yq are required programs for this script, but easy to install
if test -z "$(command -v yq)" || test -z "$(command -v jq)"; then
    echo "yq and jq are used to parse responses and regenerate the config files"
    echo "yq: https://mikefarah.gitbook.io/yq"
    echo "jq: https://jqlang.org/"
    exit 2
fi

# cannot see configPath
if test -z "${configPath}"; then
    help
    exit 2
fi

# nab the credentials from the config file
clientId="$(yq '.twitch["client-id"]' < "${configPath}")"
clientSecret="$(yq '.twitch["client-secret"]' < "${configPath}")"

# either is empty, we cannot proceed
if test -z "${clientId}" || test -z "${clientSecret}"; then
    help
    exit 2
fi

body="client_id=${clientId}&client_secret=${clientSecret}&grant_type=client_credentials"

# todo: handle invalid credentials
resp="$(curl -H "Content-Type: application/x-www-form-urlencoded" --data "${body}" ${tokenEndpoint} | jq -r)"
xc="$?"

if test "${xc}" -gt 0; then
    echo "failed to get token: ${xc}"
    echo "  response: ${resp}"
    exit 2
fi

if test -f "${configPath}.bak"; then
    # backup the backup, juuuust in case
    cp "${configPath}.bak" "${configPath}.bak.1"
fi

# backup config
cp "${configPath}" "${configPath}.bak"

accessToken="$(echo "${resp}" | jq '.access_token')"

# replace the contents using yq
yq -i ".twitch[\"access-token\"]=${accessToken}" "${configPath}"

# send a reload to the exporter, ideally configurable but this will do for now
# this or `kill -SIGHUP <pid>` will trigger a config reload, recreating the
# client with a new access token
exporterUri="http://localhost:9184/-/reload"
curl -X POST "${exporterUri}"