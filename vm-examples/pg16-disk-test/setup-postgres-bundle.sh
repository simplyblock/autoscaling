#!/bin/sh
set -e

# Pre-fetch and unpack Supabase Postgres into an OCI bundle for runc
mkdir -p /opt/postgres-oci /opt/postgres-bundle
skopeo copy docker://docker.io/supabase/postgres:15.1.0.147 oci:/opt/postgres-oci:latest
umoci unpack --image /opt/postgres-oci:latest /opt/postgres-bundle
cd /opt/postgres-bundle

# Make config host-network and add host bind mounts
jq '
  .linux.namespaces |= map(select(.type != "network")) 
  | .mounts += [
      {
        "destination": "/var/lib/postgresql/data",
        "type": "bind",
        "source": "/var/lib/postgresql/data",
        "options": ["rbind","rw"]
      },
      {
        "destination": "/etc/postgresql/pg_hba.conf",
        "type": "bind",
        "source": "/etc/pg_hba.conf",
        "options": ["rbind","ro"]
      }
    ]
  | .process.env = []
' config.json > config.template.json

rm config.json
rm -rf /opt/postgres-oci
