# pylint: disable=no-member
import time
import logging
import os.path
import configparser

import sh
from sh.contrib import rclone  # pylint: disable=import-error

from titanium import settings, utils


class MysqlNodeContext:
    """
    This class contains all the logic for configuring and exposing backup
    of a mysql pod.
    """

    def __init__(self):
        self.hostname = utils.hostname()
        self.ordinal = utils.get_ordinal()
        self.server_id = 100 + self.ordinal

        if not self.is_master():
            self.source_host = utils.get_host_for(self.ordinal - 1)
            self.master_host = utils.get_host_for(0)


    def is_master(self):
        return self.ordinal == 0

    def exists_data(self):
        return os.path.exists(os.path.join(settings.MYSQL_DATA_DIR, 'mysql'))


class ConfigPhase(MysqlNodeContext):
    """Runs in init container: init-mysql."""

    def configuration_mysqld(self, config):
        config['mysqld']['server-id'] = str(self.server_id)
        config['mysqld']['datadir'] = settings.MYSQL_DATA_DIR
        config['mysqld']['utility_user'] = '{}@%'.format(settings.UTILITY_USER)
        config['mysqld']['utility_user_password'] = settings.UTILITY_PASSWORD
        config['mysqld']['utility_user_schema_access'] = settings.UTILITY_SCHEMA_ACCESS
        config['mysqld']['utility_user_privileges'] = settings.UTILITY_PRIVILEGES

        return config

    def configuration_mysql_client(self, config):
        config['client'] = {}
        config['client']['host'] = settings.MYSQL_HOST
        config['client']['port'] = settings.MYSQL_PORT
        if settings.UTILITY_USER:
            config['client']['user'] = settings.UTILITY_USER
            config['client']['password'] = settings.UTILITY_PASSWORD

        return config

    def create_config_files(self):
        """
        Writes config file for this node.
         - /etc/mysql/my.cnf       = mysqld configuration
         - /etc/mysql/client.cnf  = mysql client configuration

        And rclone configs.
        """

        if not os.path.exists(settings.CONFIG_DIR):
            os.makedirs(settings.CONFIG_DIR)

        src_config_file_name = 'master.cnf' if self.is_master() else 'slave.cnf'
        src = os.path.join(settings.CONFIG_MAP_DIR, src_config_file_name)

        config = configparser.ConfigParser(interpolation=None)
        config.read(src)
        config = self.configuration_mysqld(config)

        dest = os.path.join(settings.CONFIG_MYSQL, 'my.cnf')
        with open(dest, 'w+') as f:
            config.write(f)

        config = configparser.ConfigParser(interpolation=None)
        config = self.configuration_mysql_client(config)

        dest = os.path.join(settings.CONFIG_MYSQL, 'client.cnf')
        with open(dest, 'w+') as f:
            config.write(f)


class InitPhase(MysqlNodeContext):
    """Runs in init container: clone-mysql."""

    def clone(self):
        """Clone data from source."""

        # Skip the clone if data already exists,
        # this may happen when pods restarts
        if self.exists_data():
            return

        if self.is_master():
            if not settings.INIT_BUCKET_URI:
                return
            # if is a master node and INIT_BUCKET_URI is set then clone for storage.
            self.get_data_from_storages(settings.INIT_BUCKET_URI)
        else:
            self.get_data_from_source_node()

        sh.xtrabackup(
            '--prepare', f'--target-dir={settings.MYSQL_DATA_DIR}',
            f'--user={settings.MASTER_REPLICATION_USER}',
            f'--password={settings.MASTER_REPLICATION_PASSWORD}'
        )

    def get_data_from_source_node(self):
        """
        Connects to source_host (usually master node) and downloads a backup and using
        xbstream data is extracted to /var/lib/mysql.
        """
        logging.info(f'Cloning from {self.source_host} ...')
        sh.xbstream(
            sh.ncat('--recv-only', self.source_host, settings.EXPOSE_BACKUPS_PORT, _piped=True),
            '-x', '-C', settings.MYSQL_DATA_DIR,
        )

    def get_data_from_storages(self, bucket_uri):
        """
        Get data from INIT buckets specified in settings.INIT_BUCKET_URI.
        Data is extracted in /var/lib/mysql
        """
        logging.info(f'Cloning from {bucket_uri} ...')
        sh.xbstream(
            sh.gzip(rclone.cat(bucket_uri, _piped=True), '-d', _piped=True),
            '-x', '-C', settings.MYSQL_DATA_DIR
        )


class RunningPhase(MysqlNodeContext):
    """Runs in containers with mysql, named: titanium."""
    def __init__(self):
        super().__init__()
        self.mysql = sh.mysql.bake(f'--defaults-file={settings.CONFIG_MYSQL}/client.cnf', '-NB')

    def is_ready(self):
        try:
            return self.mysql('-e', 'SELECT 1').strip() == '1'
        except sh.ErrorReturnCode:
            return False

    def wait_until_ready(self):
        logging.info('Waiting for mysqld to be ready (accepting connections)...')
        while not self.is_ready():
            time.sleep(1)

    def configure_slave(self):
        """
        Sets the user/password for replication and to use GTID-based
        auto-positioning.
        """

        logging.info('Initializing replication from clone position.')
        query = f"""
        CHANGE MASTER TO MASTER_AUTO_POSITION=1,
        MASTER_HOST='{self.master_host}',
        MASTER_USER='{settings.MASTER_REPLICATION_USER}',
        MASTER_PASSWORD='{settings.MASTER_REPLICATION_PASSWORD}',
        MASTER_CONNECT_RETRY={settings.MASTER_CONNECT_RETRY};

        START SLAVE;
        """

        self.mysql(_in=query)

    def configure_master(self):
        """Configure master replication. Create user for replication."""

        # This query will create the user if not exists.
        self.mysql(_in=(
            f"GRANT SELECT, PROCESS, RELOAD, LOCK TABLES, REPLICATION CLIENT, REPLICATION SLAVE ON *.* "
            f"TO '{settings.MASTER_REPLICATION_USER}'@'%' "  # TODO: limit to: %.{settings.GOVERNING_SERVICE}
            f"IDENTIFIED BY '{settings.MASTER_REPLICATION_PASSWORD}'"
        ))

    def configure(self):
        self.wait_until_ready()

        if self.is_master():
            self.configure_master()
        else:
            self.configure_slave()
