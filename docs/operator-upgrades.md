---
title: The MySQL Operator Upgrades
linktitle: Operator upgrades
description: The steps to make in order to migrate the operator to a newer version of the MySQL operator.
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

In this section ayou can find the migration path when upgrading to a newer version of the MySQL operator.

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

