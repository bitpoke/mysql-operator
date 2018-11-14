#!/bin/sh
set -e

echo "Create rclone.conf file."
cat <<EOF > /etc/rclone.conf
[s3]
type = s3
env_auth = false
provider = ${S3_PROVIDER:-"AWS"}
access_key_id = ${AWS_ACCESS_KEY_ID}
secret_access_key = ${AWS_SECRET_KEY}
region = ${AWS_REGION:-"us-east-1"}
endpoint = ${S3_ENDPOINT}
acl = ${AWS_ACL}
storage_class = ${AWS_STORAGE_CLASS}
[gs]
type = google cloud storage
project_number = ${GCS_PROJECT_ID}
service_account_file = /tmp/google-credentials.json
object_acl = ${GCS_OBJECT_ACL}
bucket_acl = ${GCS_BUCKET_ACL}
location =  ${GCS_LOCATION}
storage_class = ${GCS_STORAGE_CLASS:-"MULTI_REGIONAL"}
[http]
type = http
url = ${HTTP_URL}
[azure]
type = azure blob storage
account = ${AZUREBLOB_ACCOUNT}
key = ${AZUREBLOB_KEY}
EOF

echo "Create google-credentials.json file."
cat <<EOF > /tmp/google-credentials.json
${GCS_SERVICE_ACCOUNT_JSON_KEY}
EOF

SIDECAR_BIN=mysql-operator-sidecar
VERBOSE="-v 1 --debug"

# exec command
case "$1" in
    files-config)
        shift 1
        $SIDECAR_BIN $VERBOSE init-configs "$@"
        ;;
    clone)
        shift 1
        $SIDECAR_BIN $VERBOSE clone "$@"
        ;;
    config-and-serve)
        shift 1
        $SIDECAR_BIN $VERBOSE run "$@"
        ;;
    take-backup-to)
        shift 1
        $SIDECAR_BIN $VERBOSE take-backup-to "$@"
        ;;
    schedule-backup)
        shift 1
        $SIDECAR_BIN $VERBOSE schedule-backup "$@"
        ;;
    *)
        echo "Usage: $0 {files-config|clone|config-and-serve}"
        echo "Now runs your command."
        echo "$@"

        exec "$@"
esac
