#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

# Create a cluster. We do this as root as we are using the 'docker' driver.
### sudo -E CHANGE_MINIKUBE_NONE_USER=true minikube start --vm-driver=none
#sudo -E CHANGE_MINIKUBE_NONE_USER=true minikube addons enable ingress

# wait for minikube
while true; do
    if kubectl get nodes; then
        break
    fi
    echo "Waiting 5s for kubernetes to be ready..."
    sleep 5
done

NAMESPACE="titanium-testing"
echo "Create testing namespace ($NAMESPACE)..."

cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Namespace
metadata:
  name: ${NAMESPACE}

EOF

SECRET_NAME="backups-secret-for-gs"
echo "Create backup secret ..."

cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Secret
metadata:
  name: ${SECRET_NAME}
  namespace: ${NAMESPACE}
type: Opaque
data:
  GCS_PROJECT_ID: "$(echo $GOOGLE_PROJECT_ID | base64 -w0 )"
  GCS_SERVICE_ACCOUNT_KEY: "$(echo $GOOGLE_CREDENTIALS | base64 -w0 )"

EOF

echo "Deploy controller..."
OPERATOR_IMAGE=${OPERATOR_IMAGE:-"gcr.io/pl-infra/titanium-operator:latest"}

cat <<EOF | kubectl apply -f -
apiVersion: apps/v1beta2
kind: Deployment
metadata:
  name: titanium-controler
  namespace: titanium-testing
  labels:
    app: titanium
spec:
  replicas: 1
  selector:
    matchLabels:
      app: titanium
  template:
    metadata:
      labels:
        app: titanium
    spec:
      containers:
      - name: controller
        image: ${OPERATOR_IMAGE}
        imagePullPolicy: IfNotPresent
        args: ["-namespace", "titanium-testing"]

EOF
