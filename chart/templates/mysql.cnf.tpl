{{- define "mysqld-cnf" }}

# GENERAL #
#user                           = root
default-storage-engine         = InnoDB
gtid-mode                    = on
enforce-gtid-consistency     = on

# MyISAM #
key-buffer-size                = 32M
#myisam-recover                 = FORCE,BACKUP

# SAFETY #
max-allowed-packet             = 16M
max-connect-errors             = 1000000
skip-name-resolve              = on
sql-mode                       = STRICT_TRANS_TABLES,ERROR_FOR_DIVISION_BY_ZERO,NO_AUTO_CREATE_USER,NO_AUTO_VALUE_ON_ZERO,NO_ENGINE_SUBSTITUTION,NO_ZERO_DATE,NO_ZERO_IN_DATE,ONLY_FULL_GROUP_BY
sysdate-is-now                 = 1

# BINARY LOGGING #
expire-logs-days               = 14
sync-binlog                    = 1

# CACHES AND LIMITS #
tmp-table-size                 = 32M
max-heap-table-size            = 32M
query-cache-type               = 0
query-cache-size               = 0
max-connections                = 500
thread-cache-size              = 50
open-files-limit               = 65535
table-definition-cache         = 4096
table-open-cache               = 4096

# INNODB #
innodb-flush-method            = O_DIRECT
innodb-log-files-in-group      = 2
innodb-log-file-size           = 128M
innodb-flush-log-at-trx-commit = 2
innodb-file-per-table          = 1

# LOGGING #
log-queries-not-using-indexes  = 1
slow-query-log                 = 1

{{- end }}
