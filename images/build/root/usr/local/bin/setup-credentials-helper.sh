#!/bin/bash
: "${GOOGLE_CREDENTIALS:="$(cat "$PLUGIN_GOOGLE_CREDENTIALS_FILE" 2>/dev/null)"}"
: "${GOOGLE_CLOUD_PROJECT:="$PLUGIN_GOOGLE_CLOUD_PROJECT"}"
: "${GOOGLE_CLOUD_CLUSTER:="$PLUGIN_GOOGLE_CLOUD_CLUSTER"}"
: "${GOOGLE_CLOUD_ZONE:="$PLUGIN_GOOGLE_CLOUD_ZONE"}"
: "${SSH_KEY:="$PLUGIN_SSH_KEY"}"
: "${DOCKER_USERNAME:="$PLUGIN_DOCKER_USERNAME"}"
: "${DOCKER_PASSWORD:="$PLUGIN_DOCKER_PASSWORD"}"
: "${DOCKER_REGISTRY:="${PLUGIN_DOCKER_REGISTRY:-docker.io}"}"

DOCKER_REGISTRY_HOST="$(echo "${DOCKER_REGISTRY}" | awk 'BEGIN{ FS="/" }{print $1}')"

export PATH="$CI_WORKSPACE/bin:$PATH"

require_param() {
    declare name="$1"
    local env_name
    env_name="$(echo "$name" | tr /a-z/ /A-Z/)"
    if [ -z "${!env_name}" ] ; then
        echo "You must define \"$name\" parameter or define $env_name environment variable" >&2
        exit 2
    fi
}

require_google_credentials() {
    if [ -z "$GOOGLE_CREDENTIALS" ] ; then
        echo "You must define \"google_credentials_file\" parameter or define GOOGLE_CREDENTIALS environment variable" >&2
        exit 2
    fi
}

run() {
    echo "+" "$@"
    "$@"
}

if [ -n "$DOCKER_PASSWORD" ] ; then
    require_param DOCKER_USERNAME
    echo "+ docker login $DOCKER_REGISTRY_HOST -u $DOCKER_USERNAME"
    echo "$DOCKER_PASSWORD" | docker login "$DOCKER_REGISTRY_HOST" -u "$DOCKER_USERNAME" --password-stdin
fi

if [ -n "$GOOGLE_CREDENTIALS" ] ; then
    echo "$GOOGLE_CREDENTIALS" > /run/google-credentials.json
    run gcloud auth activate-service-account --quiet --key-file=/run/google-credentials.json
    run gcloud auth configure-docker --quiet
fi

if [ -n "$GOOGLE_CLOUD_PROJECT" ] ; then
    run gcloud config set project "$GOOGLE_CLOUD_PROJECT"
fi

if [ -n "$GOOGLE_CLOUD_CLUSTER" ] ; then
    require_google_credentials
    require_param "google_cloud_project"
    require_param "google_cloud_zone"

    run gcloud container clusters get-credentials "$GOOGLE_CLOUD_CLUSTER" --project "$GOOGLE_CLOUD_PROJECT" --zone "$GOOGLE_CLOUD_ZONE"
    # Display kubernetees versions (usefull for debugging)
    run kubectl version
fi

if [ -n "$SSH_KEY" ] ; then
    require_param "home"
    test -d "$HOME/.ssh" || mkdir -p "$HOME/.ssh"
    echo "$SSH_KEY" > "$HOME/.ssh/id_rsa"
    chmod 0400 "$HOME/.ssh/id_rsa"
    echo "Installed ssh key into $HOME/.ssh/id_rsa"
    run ssh-keygen -y -f "$HOME/.ssh/id_rsa"
fi

if [[ -n "${GIT_USER}" && -n "${GIT_PASSWORD}" ]] ; then
    git config --global user.email "${GIT_EMAIL:-bot@bitpoke.cloud}"
    git config --global user.name "$GIT_USER"

    cat <<EOF >> ~/.netrc
machine ${GIT_HOST:-github.com}
       login ${GIT_USER}
       password ${GIT_PASSWORD}
EOF
fi
