---
title: The MySQL operator upgrades
linktitle: Operator upgrades
description: The steps to do for migrating operator to a newer version of the MySQL operator
categories: [mysql operator]
keywords: [mysql operator, upgrades]
menu:
  docs:
    parent: "mysqloperator"
weight: 3
draft: false
aliases: []
toc: true
---

In this section are presented migration path when upgrading to a newer version of the MySQL
operator.

## v0.2.x upgrade

Here are described the steps that should be done when migrating from version `0.1.x` to `0.2.x` of
the operator. 

This upgrade requires Kubernetes cluster upgrade because the `v0.2.x` works only with k8s grater or
equal with `1.11` k8s version.

1. Scale to 0 the current operator deployment. This won't affect your running databases.
2. Upgrade your cluster control plane to 1.11
3. Update the installed CRDs: `kubectl apply -f
   https://raw.githubusercontent.com/presslabs/mysql-operator/master/hack/02x-crds.yaml`
4. Upgrade the mysql-operator `helm upgrade mysql-operator presslabs/mysql-operator`
5. Scale the operator deployment up
6. Now you can upgrade you nodes as well.
7. Enjoy!

