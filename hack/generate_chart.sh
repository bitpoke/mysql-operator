#!/bin/bash

version="$1"
if [ -z "$version" ] ; then
    echo "Usage: $0 <image tag>" >&2
    exit 1
fi
chart_version="${version#v}"

CHART_PATH=charts/mysql-operator

echo "Updating chart to version to: ${chart_version}"
sed -i.bak -E "
    s#version: .*#version: ${chart_version}#
    s#appVersion: .*#appVersion: ${version}#
" ${CHART_PATH}/Chart.yaml
rm ${CHART_PATH}/Chart.yaml.bak

echo "Updating chart images tag to: ${version}"
sed -i.bak -E "
    s#image: (.*):.*#image: \\1:${version}#
    s#sidecarImage: (.*):.*#sidecarImage: \\1:${version}#
" ${CHART_PATH}/values.yaml
rm ${CHART_PATH}/values.yaml.bak
