{
  "Debug": <% default .Env.ORC_DEBUG "false" %>,
  "ListenAddress": ":3000",

  "BackendDB": "sqlite",
  "SQLite3DataFile": "/var/lib/orchestrator/orc.db",

  "MySQLTopologyCredentialsConfigFile": "/orchestrator/conf/orc-topology.cnf",

  "InstancePollSeconds": <% default .Env.ORC_INSTANCE_POLL_SECONDS "5" %>,
  "DiscoverByShowSlaveHosts": <% default .Env.ORC_DISCOVERY_BY_SHOW_SLAVE_HOSTS "false" %>,

  "HostnameResolveMethod": "none",
  "MySQLHostnameResolveMethod": "@@hostname",

  "RemoveTextFromHostnameDisplay": ".presslabs.net:3306",

  "DetectClusterDomainQuery": "SELECT SUBSTRING_INDEX(SUBSTRING_INDEX(@@hostname, '.', 1), '-', 1)",
  "DetectClusterAliasQuery": "SELECT SUBSTRING_INDEX(SUBSTRING_INDEX(@@hostname, '.', 1), '-', 1)",
  "DetectInstanceAliasQuery": "SELECT SUBSTRING_INDEX(@@hostname, '.', 1)",

  "SlaveLagQuery": "SELECT TIMESTAMPDIFF(SECOND,ts,NOW()) as drift FROM tools.heartbeat WHERE server_id <> @@server_id ORDER BY drift ASC LIMIT 1"
}

