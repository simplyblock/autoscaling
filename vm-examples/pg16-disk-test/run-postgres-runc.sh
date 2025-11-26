#!/bin/sh
set -e

# Load env / secrets if provided by the platform
if [ -f /neonvm/runtime/env.sh ]; then
  . /neonvm/runtime/env.sh
fi

: "${POSTGRES_PASSWORD:?POSTGRES_PASSWORD not set}"
: "${JWT_SECRET:=$POSTGRES_PASSWORD}"

cd /opt/postgres-bundle

# Generate config.json from template, injecting env vars
jq '
  .process.env = [
    "POSTGRES_PASSWORD=" + env.POSTGRES_PASSWORD,
    "JWT_SECRET=" + env.JWT_SECRET,
    "JWT_EXP=3600"
  ]
' config.template.json > config.json

exec runc run vela-db
