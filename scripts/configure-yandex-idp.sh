#!/bin/sh
# Подставляет YANDEX_CLIENT_ID / YANDEX_CLIENT_SECRET в Keycloak после старта (опционально).
set -e

: "${KEYCLOAK_URL:=http://localhost:8080}"
: "${KEYCLOAK_ADMIN:=admin}"
: "${KEYCLOAK_ADMIN_PASSWORD:=admin}"
: "${KEYCLOAK_REALM:=reports-realm}"

if [ -z "$YANDEX_CLIENT_ID" ] || [ -z "$YANDEX_CLIENT_SECRET" ]; then
  echo "Задайте YANDEX_CLIENT_ID и YANDEX_CLIENT_SECRET в .env"
  exit 1
fi

docker compose exec keycloak /opt/keycloak/bin/kcadm.sh config credentials \
  --server "$KEYCLOAK_URL" --realm master --user "$KEYCLOAK_ADMIN" --password "$KEYCLOAK_ADMIN_PASSWORD"

docker compose exec keycloak /opt/keycloak/bin/kcadm.sh update "identity-provider/instances/yandex" -r "$KEYCLOAK_REALM" \
  -s 'config.clientId='"$YANDEX_CLIENT_ID" \
  -s 'config.clientSecret='"$YANDEX_CLIENT_SECRET"

echo "Яндекс ID credentials обновлены в Keycloak."
