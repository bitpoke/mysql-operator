---
title: Accessing Orchestrator
linktitle: Accessing Orchestrator
description: The Orchestrator part in cluster management
categories: [mysql operator, orchestrator]
keywords: [mysql operator, cluster, orchestrator]
menu:
  docs:
    parent: "mysqloperator"
weight: 3
draft: false
aliases: []
toc: true
---

The MySQL operator uses [Orchestrator](https://github.com/github/orchestrator), a tool developed by
Github, for MySQL replication topology management and High Availability. By default the operator is
deployed alongside with the operator.

## Access orchestrator

To connect to orchestrator dashboard you have to port forward orchestrator port 3000 to your local
machine. Ensure it's a healthy pod if using raft:

```shell
kubectl port-forward mysql-operator-orchestrator-0 3000
```

Then type `localhost:3000` in a browser.
