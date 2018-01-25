#!/bin/sh
set -e

echo "Create rclone.conf file."
cat <<EOF > /etc/rclone.conf
[s3]
type = s3
env_auth = false
access_key_id = ${AWS_ACCESS_KEY_ID}
secret_access_key = ${AWS_ACCESS_KEY_ID}
region = ${AWS_REGION:-"us-east-1"}
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
EOF

echo "Create google-credentials.json file."
cat <<EOF > /tmp/google-credentials.json
${GCS_SERVICE_ACCOUNT_JSON_KEY}
EOF

# exec command
case "$1" in
    files-config)
        db config_files
        ;;
    clone)
        db clone
        ;;
    config-and-serve)
        db configure; db serve_backups
        ;;
    *)
        echo "Usage: $0 {files-config|clone|config-and-serve}"
        echo "Now runs your command."

        exec "$@"
esac
