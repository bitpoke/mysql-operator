#!/bin/bash
set -e

echo "Create Google Drive service-account.json file."
echo "${GDRIVE_SERVICE_ACCOUNT}" > /tmp/gdrive-service-account.json

echo "Create rclone.conf file."
cat <<EOF > /tmp/rclone.conf
[gd]
type = drive
scope = drive
service_account_file = /tmp/gdrive-service-account.json
client_id = ${GDRIVE_CLIENT_ID}
root_folder_id = ${GDRIVE_ROOT_FOLDER_ID}
impersonate = ${GDRIVE_IMPERSONATOR}

[s3]
type = s3
env_auth = true
provider = ${S3_PROVIDER:-"AWS"}
access_key_id = ${AWS_ACCESS_KEY_ID}
secret_access_key = ${AWS_SECRET_ACCESS_KEY:-$AWS_SECRET_KEY}
region = ${AWS_REGION:-"us-east-1"}
endpoint = ${S3_ENDPOINT}
acl = ${AWS_ACL}
storage_class = ${AWS_STORAGE_CLASS}
session_token = ${AWS_SESSION_TOKEN}
no_check_bucket = true

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
type = azureblob
account = ${AZUREBLOB_ACCOUNT}
key = ${AZUREBLOB_KEY}

[hdfs]
type = hdfs
namenode = ${HDFS_NAMENODE}
username = ${HDFS_USERNAME}
EOF

if [[ -n "${GCS_SERVICE_ACCOUNT_JSON_KEY:-}" ]]; then 
    echo "Create google-credentials.json file."
    cat <<EOF > /tmp/google-credentials.json
    ${GCS_SERVICE_ACCOUNT_JSON_KEY}
EOF
else 
    touch /tmp/google-credentials.json
fi

SIDECAR_BIN=mysql-operator-sidecar
VERBOSE="--debug"

# exec command
case "$1" in
    clone-and-init)
        shift 1
        exec $SIDECAR_BIN $VERBOSE clone-and-init "$@"
        ;;
    config-and-serve)
        shift 1
        exec $SIDECAR_BIN $VERBOSE run "$@"
        ;;
    take-backup-to)
        shift 1
        exec $SIDECAR_BIN $VERBOSE take-backup-to "$@"
        ;;
    schedule-backup)
        shift 1
        exec $SIDECAR_BIN $VERBOSE schedule-backup "$@"
        ;;
    *)
        echo "Usage: $0 {clone-and-init|config-and-serve|take-backup-to|schedule-backup}"
        echo "Now runs your command."
        echo "$@"

        exec "$@"
esac
