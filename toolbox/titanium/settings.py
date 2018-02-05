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

APP_NAME = 'mysql'

# used in init-mysql init container
CONFIG_MYSQL = os.getenv('TITANIUM_CONFIG_MYSQL_DIR', '/etc/mysql')
CONFIG_DIR = os.getenv('TITANIUM_CONFIG_DIR', os.path.join(CONFIG_MYSQL, 'conf.d/'))

CONFIG_MAP_DIR = os.getenv('TITANIUM_CONFIG_MAP_DIR', '/mnt/config-map/')
SECRETS_DIR = os.getenv('TITANIUM_SECRETS_DIR', '/var/run/secrets/')

RCLONE_CONFIG_FILE = '/etc/rclone.conf'

# xtrabackup expose backups
EXPOSE_BACKUPS_PORT = os.getenv('TITANIUM_EXPOSE_BACKUPS_PORT', '3307')

GOVERNING_SERVICE = os.getenv('TITANIUM_GOVERNING_SERVICE', 'mysql')
RELEASE_NAME = os.getenv('TITANIUM_RELEASE_NAME')

# https://github.com/ncw/rclone
INIT_BUCKET_URI = os.getenv('TITANIUM_INIT_BUCKET_URI')
BACKUP_BUCKET_URI = os.getenv('TITANIUM_BACKUP_BUCKET_URI')

# replication
MASTER_REPLICATION_USER = os.getenv('MYSQL_REPLICATION_USER', 'root')
MASTER_REPLICATION_PASSWORD = os.getenv('MYSQL_REPLICATION_PASSWORD', '')
MASTER_CONNECT_RETRY = 10


# mysql
MYSQL_HOST = '127.0.0.1'
MYSQL_PORT = '3306'
MYSQL_ROOT_PASSWORD = os.getenv('MYSQL_ROOT_PASSWORD', '')
MYSQL_DATA_DIR = os.getenv('TITANIUM_MYSQL_DATA_DIR', '/var/lib/mysql/')

INNODB_BUFFER_POOL_SIZE_DEFAULT = '800M'
