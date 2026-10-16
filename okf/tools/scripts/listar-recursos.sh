#!/bin/sh
# Lista os recursos da account no Nexus (nome, classe, escrita) para conferir antes de montar um fluxo.
ACCOUNT="${1:-cond-001}"
curl -s -H "Authorization: Bearer ${NEXUS_TOKEN}" "${NEXUS_BASE_URL:-http://127.0.0.1:8081}/v1/accounts/${ACCOUNT}/resources"
