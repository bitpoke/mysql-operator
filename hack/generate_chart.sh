#!/bin/bash

tag=${1:-$(git describe --tags)}
APP_VERSION=$(echo ${tag} | sed 's/^v//')
CHART_PATH=charts/mysql-operator

echo "Updating chart to version to: ${APP_VERSION}"
sed -i "
    s/version: .*/version: ${APP_VERSION}/
    s/appVersion: .*/appVersion: ${APP_VERSION}/
" ${CHART_PATH}/Chart.yaml

echo "Updating chart images tag to: ${tag}"
sed -i "
    s/image: \(.*\):.*/image: \1:${tag}/
    s/sidecarImage: \(.*\):.*/sidecarImage: \1:${tag}/
" ${CHART_PATH}/values.yaml

