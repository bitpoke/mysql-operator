#!/bin/bash

GCS_PROJECT=testing-reactor
GCS_ZONE=europe-west3-b

function msg {
    kind=${2:-info}

    case "${kind}" in
        info)
            echo -e " \e[33m+ ${1} \e[39m"
            ;;
        success)
            echo -e " \e[92m+ ${1} \e[39m"
            ;;
        error)
            echo -e " \e[91m+ ${1} \e[39m"
            ;;
        *)
            echo -e " + ${1}"
    esac
}

function gcs_setup_cluster {
    cluster_name=${1:-test-cluster}
    k8s_version=${2:-1.11}

    msg "Start k8s cluster"
    gcloud container clusters create ${cluster_name} \
        --zone ${GCS_ZONE} --project ${GCS_PROJECT} \
        --preemptible  --cluster-version ${k8s_version}
}

function gcs_setdown_cluster {
    cluster_name=${1:-test-cluster}

    msg "Stop k8s cluster"
    gcloud container clusters delete $CLUSTER_NAME \
           --quiet --zone ${GCS_ZONE} --project ${GCS_PROJECT}
}

function gcs_master_upgrade {
    cluster_name=${1:-test-cluster}
    k8s_version=${2:-1.11}

    msg "Upgrading cluster master ${cluster_name} to ${k8s_version} ..."
    gcloud container clusters upgrade ${cluster_name} --master --cluster-version ${k8s_version} \
           --quiet --zone ${GCS_ZONE} --project ${GCS_PROJECT}
}

function gcs_nodes_upgrade {
    cluster_name=${1:-test-cluster}
    k8s_version=${2:-1.11}

    msg "Upgrading cluster nodes ${cluster_name} to ${k8s_version} ..."
    gcloud container clusters upgrade ${cluster_name} --master --cluster-version ${k8s_version} \
           --quiet --zone ${GCS_ZONE} --project ${GCS_PROJECT}
}

function setup_helm {
    msg "Setup helm ..."
    kubectl create serviceaccount -n kube-system tiller
    kubectl create clusterrolebinding tiller --clusterrole=cluster-admin --serviceaccount=kube-system:tiller
    helm init --service-account tiller --wait
    helm repo add presslabs https://presslabs.github.io/charts
    helm dependency update hack/charts/mysql-operator
}

OP_NAMESPACE=kube-system
OP_CHART=presslabs/mysql-operator
OP_RELEASE=test

function install_operator {
    version=${1:-version}
    OP_CHART=${2:-presslabs/mysql-operator}
    OP_RELEASE=${3:-test}

    msg "Install operator (v:${OP_CHART} c:${version}) as ${OP_RELEASE} ..."
    helm install $OP_CHART --namespace $OP_NAMESPACE --name $OP_RELEASE \
         --version $version \
         --set orchestrator.replicas=1
}

function op_scale {
    chart=${1:-${OP_CHART}}
    replicas=${2:-0}

    msg "Scale operator (${OP_RELEASE}) to: ${replicas} (${chart}) ..."
    helm upgrade $OP_RELEASE $chart --set replicaCount=${replicas} \
         --reuse-values
}

function upgrade_operator {
    chart=${1:-${OP_CHART}}
    version=${2:-master}
    tag=${3:-latest}

    msg "Upgrade operator (${OP_RELEASE}) to version: ${version} ..."
    helm upgrade $OP_RELEASE $chart --version $version \
         --reuse-values \
         --set image=quay.io/presslabs/mysql-operator:${tag} \
         --set sidecarImage=quay.io/presslabs/mysql-operator-sidecar:${tag}

}


CL_NAMESPACE=default

function create_cluster {
    name=${1:-my-cluster}
    replicas=${2:-1}
    root_pass=${3:-not-so-secure}

    msg "Creating a new cluster: ${name} with ${replicas} replicas..."
    cat <<EOF | kubectl apply -f-
apiVersion: mysql.presslabs.org/v1alpha1
kind: MysqlCluster
metadata:
  name: ${name}
  namespace: ${CL_NAMESPACE}
spec:
  replicas: ${replicas}
  secretName: my-cluster-secret
  podSpec:
    imagePullPolicy: Always
    resources:
      requests:
        cpu: 10m
        memory: 128Mi
---
apiVersion: v1
kind: Secret
metadata:
  name: my-cluster-secret
type: Opaque
data:
  ROOT_PASSWORD: $(echo -n "${root_pass}" | base64)
EOF
}

function wait_cluster_ready {
    name=${1:-my-cluster}
    replicas=${2:-1}

    tries=3
    msg "Wait for cluster: ${name} to be ready (${tries} tries)..."
    for i in $(seq 1 $tries); do
        sleep 2
        kubectl -n $CL_NAMESPACE wait mysqlcluster/$name --for condition=ready \
                --timeout 300s
    done

}

function read_endpoint {
    name=${1:-my-cluster}

    echo "${name}-mysql.${CL_NAMESPACE}"
}

function start_check_job {
    name=${1:-my-cluster}
    root_pass=${2:-not-so-secure}

    msg "Create check job ($name) ..."
    cat <<EOF | kubectl apply -f -
apiVersion: batch/v1
kind: Job
metadata:
  name: check-mysql-cluster
spec:
  backoffLimit: 0
  template:
    spec:
      restartPolicy: Never
      containers:
      - name: check
        image: percona:5.7
        command: ["/bin/bash", "-c"]
        args:
          - |
            set -e
            while true; do
            mysql --host=$(read_endpoint ${name}) --user=root --password=${root_pass} -e 'SELECT 1'
            sleep 5
            done
EOF
}

function check_job_ok {
    JSONPATH='{range @.status.conditions[*]}{@.type}={@.status};{end}'

    msg "Check job status ..."
    sleep 10
    out=$(kubectl get jobs check-mysql-cluster -o jsonpath="$JSONPATH")
    if [[ "$out" == *"Failed=True"* ]]; then
        msg "Cluser was down!!" error
        return 1
    else
        msg "Cluster was ok!!" success
    fi
}

function apply_new_crds {
    file=${1:-02x-crds.yaml}

    msg "Apply new crds ($file)..."
    kubectl apply -f ${file}
}

CLUSTER_NAME=mytest-cluster
CLUSTER_VERSION=1.10

function cmd_setup {
    # create the gcs cluster
    gcs_setup_cluster $CLUSTER_NAME $CLUSTER_VERSION

    # configure cluster with helm (tiller)
    setup_helm

    # operator install
    install_operator 0.1.13 presslabs/mysql-operator test
}

function cmd_teardown {
    # delete the cluster from gcs
    gcs_setdown_cluster $CLUSTER_NAME
}

function cmd_test {
    CL_NAME=my-cluster
    # install a cluster with 2 replica
    create_cluster $CL_NAME 2

    # wait for cluster to be ready
    wait_cluster_ready $CL_NAME

    # start a job that checks for mysql to be up
    start_check_job $CL_NAME

    # scale down operator to zero
    op_scale presslabs/mysql-operator 0

    # check job
    check_job_ok
    [ $? -ne 0 ] && exit 1

    CLUSTER_NEW_VERSION=1.11
    # upgrade cluster control plain
    gcs_master_upgrade $CLUSTER_NAME $CLUSTER_NEW_VERSION

    # update to the lastest chart version
    upgrade_operator ./charts/mysql-operator/ master latest

    # apply new creds
    apply_new_crds 02x-crds.yaml

    # scale back up the operator
    op_scale ./charts/mysql-operator/ 1

    # wait to wake up the operator and to restart the cluster
    sleep 10

    # wait cluster to be ready
    wait_cluster_ready $CL_NAME

    # check the job status to determine if the cluster was down
    check_job_ok
    [ $? -ne 0 ] && exit 1

}


HELP="Use [up, test, down] commands to setup gcs cluster, start tests, teardown gcs cluster."

if [ $# -eq 0 ]; then
    echo $HELP
fi

while [ $# -ne 0 ]
do
    arg="$1"
    case "$arg" in
        up)
            cmd_setup
            ;;
        test)
            cmd_test
            ;;
        down)
            cmd_teardown
            ;;
        *)
            echo $HELP
            ;;
    esac
    shift
done
