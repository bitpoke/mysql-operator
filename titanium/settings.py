import os
import logging


SENTRY_DSN = os.getenv('TITANIUM_SENTRY_DSN')

LOG_LEVEL = os.getenv('TITANIUM_LOG_LEVEL', logging.INFO)

LOGGING = {
    'version': 1,
    'handlers': {
        'console': {
            'level': 'DEBUG',
            'class': 'logging.StreamHandler',
            'formatter': 'console'
            },
    },
    'root': {
        'handlers': ['console'],
        'level': logging.DEBUG
    }
}


CONFIG_DIR = os.getenv('TITANIUM_CONFIG_DIR', '/mnt/config.d/')
CONFIG_MAP_DIR = os.getenv('TITANIUM_CONFIG_MAP_DIR', '/mnt/config-map/')

MYSQL_DATA_DIR = os.getenv('TITANIUM_MYSQL_DATA_DIR', '/var/lib/mysql/')

GOVERNING_SERVICE = os.getenv('TITANIUM_GOVERNING_SERVICE', 'mysql')

EXPOSE_BACKUPS_PORT = os.getenv('TITANIUM_EXPOSE_BACKUPS_PORT', '3307')


# https://github.com/ncw/rclone
BACKUP_BUCKET = os.getenv('TITANIUM_BACKUP_BUCKET')
INIT_BUCKET = os.getenv('TITANIUM_INIT_BUCKET')


# replication
MASTER_REPLICATION_USER = 'root'
MASTER_REPLICATION_PASSWORD = ''
MASTER_CONNECT_RETRY = 10
