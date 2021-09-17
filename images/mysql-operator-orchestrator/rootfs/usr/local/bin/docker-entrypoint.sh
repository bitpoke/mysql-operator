#!/bin/sh
dockerize -no-overwrite -template /usr/local/share/orchestrator/templates/orchestrator.conf.json:/etc/orchestrator/orchestrator.conf.json
dockerize -no-overwrite -template /usr/local/share/orchestrator/templates/orc-topology.cnf:/etc/orchestrator/orc-topology.cnf

exec "$@"
