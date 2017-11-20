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
            },
    },
    'root': {
        'handlers': ['console'],
        'level': LOG_LEVEL
    }
}


CONFIG_DIR = os.getenv('TITANIUM_CONFIG_DIR', '/etc/mysql/conf.d/')
CONFIG_MAP_DIR = os.getenv('TITANIUM_CONFIG_MAP_DIR', '/mnt/config-map/')

MYSQL_DATA_DIR = os.getenv('TITANIUM_MYSQL_DATA_DIR', '/var/lib/mysql/')

GOVERNING_SERVICE = os.getenv('TITANIUM_GOVERNING_SERVICE', 'mysql')

EXPOSE_BACKUPS_PORT = os.getenv('TITANIUM_EXPOSE_BACKUPS_PORT', '3307')

PROJECT_NAME = os.getenv('TITANIUM_PROJECT_NAME', 'mysql')


# https://github.com/ncw/rclone
BACKUP_BUCKET = os.getenv('TITANIUM_BACKUP_BUCKET')
INIT_BUCKET_URI = os.getenv('TITANIUM_INIT_BUCKET_URI')


# replication
MASTER_REPLICATION_USER = 'root'
MASTER_REPLICATION_PASSWORD = ''
MASTER_CONNECT_RETRY = 10
