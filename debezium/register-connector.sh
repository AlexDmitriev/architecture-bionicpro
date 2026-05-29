#!/bin/sh
set -eu

CONNECT_URL="http://debezium-connect:8083/connectors"
CONFIG_FILE="/init/crm-connector.json"

echo "Waiting for Debezium Connect..."
until curl -s "${CONNECT_URL}" >/dev/null; do
  sleep 2
done

echo "Registering CRM connector..."
curl -s -X POST "${CONNECT_URL}" \
  -H "Content-Type: application/json" \
  --data @"${CONFIG_FILE}" >/tmp/connector_response.txt || true

if curl -s "${CONNECT_URL}/crm-postgres-connector" >/dev/null; then
  echo "Connector is ready"
  exit 0
fi

echo "Connector registration response:"
cat /tmp/connector_response.txt
exit 1
