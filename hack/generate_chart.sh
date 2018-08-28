#/bin/sh

CHART_PATH=hack/charts/mysql-operator
CHART_TEMPLATE_DIR=${CHART_PATH}/templates

#
# rbac content
#
RBAC=${CHART_TEMPLATE_DIR}/clusterrole.yaml
cat <<EOF > ${RBAC}
{{- if .Values.rbac.create -}}
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: ClusterRole
metadata:
  name: {{ template "mysql-operator.fullname" . }}
  labels:
    app: {{ template "mysql-operator.name" . }}
    chart: {{ template "mysql-operator.chart" . }}
    release: {{ .Release.Name }}
    heritage: {{ .Release.Service }}
rules:
EOF

sed -n -e '
		/rules:/,$ {
			/rules:/n
			p
		}
	' config/rbac/rbac_role.yaml >> ${RBAC}

echo "{{- end -}}" >> ${RBAC}

#
# crds
#
CRDS=${CHART_TEMPLATE_DIR}/crds.yaml
echo "{{- if .Values.installCRDs }}" > ${CRDS}

cat config/crds/*.yaml >> ${CRDS}
sed -i '
/apiVersion: apiextensions.k8s.io\/.*/ i\
---
' $CRDS

echo "{{- end -}}" >> ${CRDS}

#
# Update chart version
#


# Usually $TAG value should be what git describe --tags returns
TAG=${1:-$(git describe --tags)}

version=$(echo ${TAG} | sed 's/^v//')
echo "Updating chart to version: ${version}"
sed -i "
    s/version: .*/version: ${version}/
    s/appVersion: .*/appVersion: ${version}/
" ${CHART_PATH}/Chart.yaml

echo "Updating chart images to tag: ${TAG}"
sed -i "
    s/image: \(.*\):.*/image: \1:${TAG}/
    s/helperImage: \(.*\):.*/helperImage: \1:${TAG}/
" ${CHART_PATH}/values.yaml

