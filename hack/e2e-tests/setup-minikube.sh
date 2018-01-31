#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

# Create a cluster. We do this as root as we are using the 'docker' driver.
# sudo -E CHANGE_MINIKUBE_NONE_USER=true minikube start --vm-driver=none
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

json_key="$(echo ${TITANIUM_TEST_GS_CREDENTIALS} | base64 | awk -v ORS='' '$0' )"
project_id="$(echo $GOOGLE_PROJECT_ID | base64 | awk -v ORS='' '$0' )"

cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Secret
metadata:
  name: ${SECRET_NAME}
  namespace: ${NAMESPACE}
type: Opaque
data:
  GCS_PROJECT_ID: "$project_id"
  GCS_SERVICE_ACCOUNT_JSON_KEY: "$json_key"

EOF

echo "Deploy controller..."
OPERATOR_IMAGE=${OPERATOR_IMAGE:-"gcr.io/pl-infra/titanium-operator:latest"}
echo "Using image: $OPERATOR_IMAGE"

cat <<EOF | kubectl apply -f -
apiVersion: apps/v1beta2
kind: Deployment
metadata:
  name: titanium-controller
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

echo "Wait for controller to be up..."

# wait for pod
j=0
while [ $j -le 50 ]; do
    pod="$(kubectl get pods --namespace $NAMESPACE | awk '$1~/titanium-controller/{print $1}')"
    phase="$(kubectl get pods $pod --namespace $NAMESPACE -o jsonpath='{.status.phase}')"
    if [ "$phase" == "Running" ]; then
        break
    fi
    echo "Waiting 2s for pod ($pod) to be ready..."
    sleep 2
    j=$(( j + 1 ))
done


j=0
while [ $j -le 20 ]; do
    kubectl get mysql &> /dev/null
    if [ $? -ne 1 ]; then
        break
    fi
    echo "Wait 2s for mysql resource to be ready..."
    sleep 2
    j=$(( j + 1 ))
done
