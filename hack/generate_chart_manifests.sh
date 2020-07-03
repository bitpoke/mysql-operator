#!/bin/bash

CHART_PATH=../charts/mysql-operator
CONFIG_PATH=../config

rm ${CHART_PATH}/crds/*.yaml

# crds/crds.yaml update
for file in ${CONFIG_PATH}/crds/*.yaml; do
    # copy crd file to dest dir
    destFile=${CHART_PATH}/crds/${file#${CONFIG_PATH}/crds/}
    cp $file $destFile

    yq w -d'*' -i $destFile 'metadata.annotations[helm.sh/hook]' crd-install
    yq w -d'*' -i $destFile 'metadata.labels[app]' mysql-operator
    yq d -d'*' -i $destFile metadata.creationTimestamp
    yq d -d'*' -i $destFile status
    yq d -d'*' -i $destFile spec.validation

done

# templates/rbac.yaml update
cp ${CONFIG_PATH}/rbac/role.yaml ${CHART_PATH}/templates/rbac.yaml
yq m -d'*' -i ${CHART_PATH}/templates/rbac.yaml chart-metadata.yaml
yq d -d'*' -i ${CHART_PATH}/templates/rbac.yaml metadata.creationTimestamp
yq w -d'*' -i ${CHART_PATH}/templates/rbac.yaml metadata.name '{{ template "mysql-operator.fullname" . }}'
echo '{{- if .Values.rbac.create }}' > ${CHART_PATH}/templates/clusterrole.yaml
cat ${CHART_PATH}/templates/rbac.yaml >> ${CHART_PATH}/templates/clusterrole.yaml
echo '{{- end }}' >> ${CHART_PATH}/templates/clusterrole.yaml
rm ${CHART_PATH}/templates/rbac.yaml

