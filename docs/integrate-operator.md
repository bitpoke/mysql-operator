---
title: Integration of the MySQL Operator
linktitle: MySQL Operator Integration
description: How to integrate the MySQL operator with your application.
categories: [mysql operator]
keywords: [mysql operator]
menu:
  docs:
    parent: "mysqloperator"
weight: 3
draft: false
aliases: []
toc: true
---

After cluster creation, you can update the provided secret with a new field named `DB_CONNECT_URL` that contains a [DSN](https://en.wikipedia.org/wiki/Data_source_name) to connect to the writable cluster endpoint. You can check the `_helper.tpl` file for more insights.

The MySQL operator provides 3 services to access the nodes:

 * `<cluster_name>-mysql-master` is the service that points to the master node and this endpoint should be used for writes. This service is usually used to construct the DSN.

 * `<cluster_name>-mysql` is the service that routes traffic to all the _healthy_ nodes from the cluster. You should use this endpoint for reads.

 * `<cluster_name>-mysql-nodes` is the service used internally to access nodes. You can use this service to access a specific node (e.g. `<cluster_name>-mysql-0.<cluster-name>-mysql-nodes.default`)

We use helm to deploy our application into Kubernetes, so we updated our charts to use the MySQL operator to provide one cluster per application.
