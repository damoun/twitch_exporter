#!/usr/bin/env bash


tokenEndpoint="https://id.twitch.tv/oauth2/token"

help() {
    echo "Usage: ./scripts/getAppAccessToken.sh <client-id> <client-secret>"
    exit 2
}

clientId="${1?help}"
clientSecret="${2?help}"

if test -z $clientId || test -z $clientSecret; then
    help
fi

body="client_id=${clientId}&client_secret=${clientSecret}&grant_type=client_credentials"

curl -H "Content-Type: application/x-www-form-urlencoded" --data "${body}" ${tokenEndpoint} | jq