---
title: The MySQL Operator Upgrades
linktitle: Operator upgrades
description: The steps to make in order to migrate the operator to a newer version of the MySQL operator.
keywords: [mysql operator, upgrades]
menu:
  global:
    parent: "mysql-operator"
    weight: 3
slug: operator-upgrades
---

In this section you can find the migration path when upgrading to a newer version of the MySQL operator.

## v0.2.x upgrade

Here are described the steps that you should follow when migrating from version `0.1.x` to `0.2.x` of the operator.

This upgrade requires a Kubernetes cluster upgrade because the `v0.2.x` works only with a Kubernetes version greater or equal to `1.11`.

1. Scale the current operator deployment to 0. This won't affect your running databases.
2. Upgrade your cluster control plane to 1.11.
3. Update the installed CRDs: `kubectl apply -f
   https://raw.githubusercontent.com/presslabs/mysql-operator/master/hack/02x-crds.yaml`
4. Upgrade the MySQL operator `helm upgrade mysql-operator presslabs/mysql-operator`
5. Scale up the operator deployment
6. Now you can upgrade your nodes, as well.
7. Enjoy!

## v0.3.x upgrade

This version requires at list Kubernetes `1.11`. The upgrade should be tried on staging environment
first because it's with downtime. It requires the deletion of the statefulset and recreation of it.
The reason behind is that we changed the headless service name (which is an immutable statefulset
field) to make it smaller that will prevent you from hitting this
[bug](https://github.com/presslabs/mysql-operator/issues/170).

The operator should do all the work for you but you have to make sure that you have the latest
`0.2.x` version of the operator. For a smooth upgrade, it's recommended to have clusters with only
one node or the master node to be node 0. Doing so will prevent the operator to wait for a failover.

This release drop support for `emptyDir` volume source.
