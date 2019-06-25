---
title: Accessing the Orchestrator
linktitle: Accessing Orchestrator
description: The orchestrator part in cluster management
categories: [mysql operator, orchestrator]
keywords: [mysql operator, cluster, orchestrator]
menu:
  docs:
    parent: "mysqloperator"
weight: 4
draft: false
aliases: []
toc: true
---

The MySQL operator uses [orchestrator](https://github.com/github/orchestrator), a tool developed by GitHub for MySQL replication topology management and high availability. By default, the operator is deployed alongside with the operator.

## Access the orchestrator

The service `<release-name>-mysql-operator` exposes port 80. Via this port you will be able to talk to the orchestrator leader.
You can either port fowrwad this service to localhost, or use a service of type load balancer or enable the ingress.

```shell
kubectl port-forward service/<release-name>-mysql-operator 8080:80
```

Then type `localhost:3000` in a browser.
