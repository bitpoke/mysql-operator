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

OP_NAMESPACE=default
OP_CHART=presslabs/mysql-operator
OP_RELEASE=test_upgrade

function install_operator {
    version=${1:-version}
    OP_CHART=${2:-presslabs/mysql-operator}
    OP_RELEASE=${3:-test}

    msg "Install operator (v:${OP_CHART} c:${version}) as ${OP_RELEASE} ..."
    helm install $OP_CHART --namespace $OP_NAMESPACE --name $OP_RELEASE \
         --version $version \
         -f ./development/dev-values.yaml \
         --set orchestrator.replicas=1 --wait
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
         --set sidecarImage=quay.io/presslabs/mysql-operator-sidecar:${tag} \
         --wait

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

function run_query {
    pod=${1}-mysql-${2}
    query=${3}

    kubectl exec $pod -- mysql --defaults-file=/etc/mysql/client.conf -NB -e "$query"
}

function run_query_old {
    pod=${1}-mysql-${2}
    query=${3}

    kubectl exec $pod -- mysql --defaults-file=/etc/mysql/client.cnf -NB -e "$query"
}


function check_cluster_version {
    name=${1}
    expected=${2:-300}
    version_annotation="mysql.presslabs.org/version"

    version=$(kubectl get mysql ${name} -o json | jq -r .metadata.annotations.\"${version_annotation}\")
    msg "Check version of ${name} cluster that is: ${version} to be ${expected} ..."
    if [[ "$version" -eq "$expected" ]]; then
        msg "Ok version" success
        return 0
    fi

    msg "Bad version" error
    return 1

}

CLUSTER_NAME=mytest-cluster
CLUSTER_VERSION=1.10

function cmd_setup {
    # create the gcs cluster
    gcs_setup_cluster $CLUSTER_NAME $CLUSTER_VERSION

    # configure cluster with helm (tiller)
    setup_helm

}

function cmd_teardown {
    # delete the cluster from gcs
    gcs_setdown_cluster $CLUSTER_NAME
}

function cmd_test {

    # operator install
    install_operator 0.2.10 presslabs/mysql-operator

    # install a cluster with 2 replica
    CL_NAME=my-cluster
    create_cluster $CL_NAME 2

    # wait for cluster to be ready
    wait_cluster_ready $CL_NAME

    run_query_old $CL_NAME 0 "CREATE DATABASE IF NOT EXISTS test; CREATE TABLE IF NOT EXISTS test.test (name varchar(10) PRIMARY KEY)"
    run_query_old $CL_NAME 0 "INSERT INTO test.test VALUES ('test1')"

    # trigger failover
    kubectl delete pod $CL_NAME-mysql-0

    wait_cluster_ready $CL_NAME

    # update to the lastest chart version
    #upgrade_operator ./charts/mysql-operator/ master latest
    msg "Upgrade operator to latest version"
    cd ../ && make skaffold-run

    # wait to wake up the operator and to restart the cluster
    sleep 30

    # wait cluster to be ready
    wait_cluster_ready $CL_NAME

    check_cluster_version $CL_NAME 300

    current=$(run_query $CL_NAME 0 "SELECT 'ok' FROM test.test WHERE name='test1'")
    if [ "$current" == "ok" ]; then
        msg "Ok sql test" success
    else
        msg "Bad sql test" error
    fi
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
