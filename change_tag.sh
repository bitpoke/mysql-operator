#!/bin/bash

new_tag=${1:-latest}
sed -i "/gcr.io\/pl-infra\/titanium-toolbox:latest/c\ \ image:\ gcr.io\/pl-infra\/titanium-toolbox:$new_tag" \
    charts/titanium/values.yaml
