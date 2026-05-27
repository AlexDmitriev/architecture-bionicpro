#!/bin/sh
set -eu

KEYCLOAK_SERVER="${KEYCLOAK_SERVER:-http://keycloak:8080}"
KEYCLOAK_ADMIN="${KEYCLOAK_ADMIN:-admin}"
KEYCLOAK_ADMIN_PASSWORD="${KEYCLOAK_ADMIN_PASSWORD:-admin}"
KEYCLOAK_REALM="${KEYCLOAK_REALM:-reports-realm}"
CLIENT_ID="${KEYCLOAK_CLIENT_ID:-bionicpro-auth}"
CLIENT_SECRET="${KEYCLOAK_CLIENT_SECRET:-bionicpro-auth-secret-change-me}"
REDIRECT_URI="${REDIRECT_URI:-http://localhost:8081/auth/callback}"
FRONTEND_URL="${FRONTEND_URL:-http://localhost:3000}"

echo "Waiting for Keycloak at ${KEYCLOAK_SERVER}..."
until /opt/keycloak/bin/kcadm.sh config credentials \
  --server "${KEYCLOAK_SERVER}" \
  --realm master \
  --user "${KEYCLOAK_ADMIN}" \
  --password "${KEYCLOAK_ADMIN_PASSWORD}" >/dev/null 2>&1; do
  sleep 2
done

echo "Keycloak is ready. Ensuring client ${CLIENT_ID} in realm ${KEYCLOAK_REALM}..."

CLIENT_UUID="$(
  /opt/keycloak/bin/kcadm.sh get clients -r "${KEYCLOAK_REALM}" -q "clientId=${CLIENT_ID}" --fields id,clientId \
  | sed -n 's/.*"id" : "\([^"]*\)".*/\1/p' \
  | sed -n '1p'
)"

if [ -z "${CLIENT_UUID}" ]; then
  echo "Client ${CLIENT_ID} not found, creating..."
  /opt/keycloak/bin/kcadm.sh create clients -r "${KEYCLOAK_REALM}" \
    -s "clientId=${CLIENT_ID}" \
    -s "enabled=true" \
    -s "protocol=openid-connect" \
    -s "publicClient=false" \
    -s "secret=${CLIENT_SECRET}" \
    -s "standardFlowEnabled=true" \
    -s "implicitFlowEnabled=false" \
    -s "directAccessGrantsEnabled=false" \
    -s "serviceAccountsEnabled=false" \
    -s "redirectUris=[\"${REDIRECT_URI}\"]" \
    -s "webOrigins=[\"${FRONTEND_URL}\",\"http://localhost:8081\"]" \
    -s 'attributes."pkce.code.challenge.method"=S256' \
    -s 'attributes."oauth2.token.exchange.grant.enabled"=true' >/dev/null
else
  echo "Client ${CLIENT_ID} exists (${CLIENT_UUID}), updating..."
  /opt/keycloak/bin/kcadm.sh update "clients/${CLIENT_UUID}" -r "${KEYCLOAK_REALM}" \
    -s "enabled=true" \
    -s "secret=${CLIENT_SECRET}" \
    -s "standardFlowEnabled=true" \
    -s "implicitFlowEnabled=false" \
    -s "directAccessGrantsEnabled=false" \
    -s "serviceAccountsEnabled=false" \
    -s "redirectUris=[\"${REDIRECT_URI}\"]" \
    -s "webOrigins=[\"${FRONTEND_URL}\",\"http://localhost:8081\"]" \
    -s 'attributes."pkce.code.challenge.method"=S256' \
    -s 'attributes."oauth2.token.exchange.grant.enabled"=true' >/dev/null
fi

echo "Keycloak bootstrap completed."
