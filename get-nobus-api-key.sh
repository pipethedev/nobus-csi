#!/usr/bin/env bash
set -euo pipefail

base_url="${NOBUS_API_URL:-https://cloud-api.nobus.io}"
email="${NOBUS_EMAIL:-}"

command -v curl >/dev/null || { echo "curl is required" >&2; exit 1; }
command -v jq >/dev/null || { echo "jq is required" >&2; exit 1; }

if [[ -z "$email" ]]; then
  printf "Nobus email: " >&2
  IFS= read -r email
else
  printf "Using Nobus email: %s\n" "$email" >&2
fi

printf "Nobus password: " >&2
IFS= read -r -s password
printf "\n" >&2

if ! response="$(
  curl --silent --show-error --fail-with-body \
    "$base_url/api/v3/auth/login/initiate" \
    -H "Content-Type: application/json" \
    -d "$(jq -n \
      --arg email "$email" \
      --arg password "$password" \
      '{email: $email, password: $password}')"
)"; then
  printf "Login initiation failed:\n%s\n" "$response" >&2
  exit 1
fi

if ! jq -e '.mail_sent == true and .is_active == true' \
  >/dev/null <<<"$response"; then
  printf "Unexpected response:\n%s\n" "$response" >&2
  exit 1
fi

printf "Verification code sent.\nEnter code: " >&2
IFS= read -r code

if ! response="$(
  curl --silent --show-error --fail-with-body \
    "$base_url/api/v3/auth/login/complete" \
    -H "Content-Type: application/json" \
    -d "$(jq -n \
      --arg email "$email" \
      --arg code "$code" \
      '{email: $email, code: $code}')"
)"; then
  printf "Login completion failed:\n%s\n" "$response" >&2
  exit 1
fi

if ! login_token="$(jq -er \
  '.token | select(type == "string" and length > 0)' <<<"$response")"; then
  printf "Login response did not contain a token:\n%s\n" "$response" >&2
  exit 1
fi

if ! response="$(
  curl --silent --show-error --fail-with-body \
    -X POST "$base_url/api/v3/auth/api-key" \
    -H "Authorization: Bearer $login_token" \
    -H "Content-Type: application/json" \
    -d '{}'
)"; then
  printf "API-key creation failed:\n%s\n" "$response" >&2
  exit 1
fi

if ! api_key="$(jq -er \
  '.api_key | select(type == "string" and length > 0)' <<<"$response")"; then
  printf "Response did not contain an API key:\n%s\n" "$response" >&2
  exit 1
fi

printf '%s\n' "$api_key"
