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

To connect to the orchestrator dashboard you have to point the orchestrator port 3000 to your local machine. Ensure it's a healthy pod if using raft:

```shell
kubectl port-forward mysql-operator-orchestrator-0 3000
```

Then type `localhost:3000` in a browser.
