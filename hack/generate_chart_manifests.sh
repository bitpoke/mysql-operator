#!/bin/bash

CHART_PATH=../charts/mysql-operator
CONFIG_PATH=../config

# crds/crds.yaml update
awk 'FNR==1 && NR!=1 {print "---"}{print}' ${CONFIG_PATH}/crds/*.yaml > ${CHART_PATH}/crds/crds.yaml
yq w -d'*' -i ${CHART_PATH}/crds/crds.yaml 'metadata.annotations[helm.sh/hook]' crd-install
yq w -d'*' -i ${CHART_PATH}/crds/crds.yaml 'metadata.labels[app]' mysql-operator
yq d -d'*' -i ${CHART_PATH}/crds/crds.yaml metadata.creationTimestamp
yq d -d'*' -i ${CHART_PATH}/crds/crds.yaml status
yq d -d'*' -i ${CHART_PATH}/crds/crds.yaml spec.validation

# add shortName to CRD until https://github.com/kubernetes-sigs/kubebuilder/issues/404 is solved
yq w -d1 -i ${CHART_PATH}/crds/crds.yaml 'spec.names.shortNames[+]' mysql

# templates/rbac.yaml update
cp ${CONFIG_PATH}/rbac/rbac_role.yaml ${CHART_PATH}/templates/rbac.yaml
yq m -d'*' -i ${CHART_PATH}/templates/rbac.yaml chart-metadata.yaml
yq d -d'*' -i ${CHART_PATH}/templates/rbac.yaml metadata.creationTimestamp
yq w -d'*' -i ${CHART_PATH}/templates/rbac.yaml metadata.name '{{ template "mysql-operator.fullname" . }}'
echo '{{- if .Values.rbac.create }}' > ${CHART_PATH}/templates/clusterrole.yaml
cat ${CHART_PATH}/templates/rbac.yaml >> ${CHART_PATH}/templates/clusterrole.yaml
echo '{{- end }}' >> ${CHART_PATH}/templates/clusterrole.yaml
rm ${CHART_PATH}/templates/rbac.yaml

