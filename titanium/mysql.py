# pylint: disable=no-member
import time
import logging
import os.path
import configparser

import sh
from sh.contrib import rclone  # pylint: disable=import-error

from titanium import settings, utils


class MysqlNode:
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

        self.mysql = sh.mysql.bake(f'--defaults-file={settings.CONFIG_MYSQL}/client.cnf', '-NB')

    def is_master(self):
        return self.ordinal == 0

    def exists_data(self):
        return os.path.exists(os.path.join(settings.MYSQL_DATA_DIR, 'mysql'))

    def is_ready(self):
        try:
            return self.mysql('-e', 'SELECT 1').strip() == '1'
        except sh.ErrorReturnCode:
            return False

    def configuration_mysqld(self, config):
        config['mysqld']['server-id'] = str(self.server_id)
        config['mysqld']['datadir'] = settings.MYSQL_DATA_DIR

        if not config['mysqld'].get('innodb-buffer-pool-size'):
            config['mysqld']['innodb-buffer-pool-size'] = utils.get_innodb_buffer_pool_size()

        return config

    def configuration_mysql_client(self, config):
        config['client'] = {}
        config['client']['host'] = settings.MYSQL_HOST
        config['client']['port'] = settings.MYSQL_PORT
        if settings.MYSQL_ROOT_PASSWORD:
            config['client']['password'] = settings.MYSQL_ROOT_PASSWORD

        return config

    def create_config_files(self):
        """
        Writes config file for this node.
         - /etc/mysql/my.cnf       = mysqld configuration
         - /etc/mysql/client.cnf  = mysql client configuration
        """

        if not os.path.exists(settings.CONFIG_DIR):
            os.makedirs(settings.CONFIG_DIR)

        src_config_file_name = 'master.cnf' if self.is_master() else 'slave.cnf'
        src = os.path.join(settings.CONFIG_MAP_DIR, src_config_file_name)

        config = configparser.ConfigParser()
        config.read(src)
        config = self.configuration_mysqld(config)

        dest = os.path.join(settings.CONFIG_MYSQL, 'my.cnf')
        with open(dest, 'w+') as f:
            config.write(f)

        config = configparser.ConfigParser()
        config = self.configuration_mysql_client(config)

        dest = os.path.join(settings.CONFIG_MYSQL, 'client.cnf')
        with open(dest, 'w+') as f:
            config.write(f)

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
            logging.info(f'Clone from storage. URI: {settings.INIT_BUCKET_URI}')
            self.get_data_from_storages()
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
        sh.xbstream(
            sh.ncat('--recv-only', self.source_host, settings.EXPOSE_BACKUPS_PORT, _piped=True),
            '-x', '-C', settings.MYSQL_DATA_DIR,
        )

    def get_data_from_storages(self):
        """
        Get data from INIT buckets specified in settings.INIT_BUCKET_URI.
        Data is extracted in /var/lib/mysql
        """
        sh.xbstream(
            sh.gzip(rclone.cat(settings.INIT_BUCKET_URI, _piped=True), '-d', _piped=True),
            '-x', '-C', settings.MYSQL_DATA_DIR
        )

    def get_binlog_position(self):
        """
        Determine binlog position from cloned data.

        Returns: <binlog file name>, <binlog position>
        """
        xtb_slave_info_file_name = os.path.join(settings.MYSQL_DATA_DIR, 'xtrabackup_slave_info')
        if os.path.exists(xtb_slave_info_file_name):
            # XtraBackup already generated a partial "CHANGE MASTER TO" query
            # because we're cloning from an existing slave.
            info = utils.parse_slave_info_xtb_file(xtb_slave_info_file_name)
            if info:
                return info

        xtb_binlog_file_name = os.path.join(settings.MYSQL_DATA_DIR, 'xtrabackup_binlog_info')
        if os.path.exists(xtb_binlog_file_name):
            # We're cloning directly from master. Parse binlog position.
            info = utils.parse_xtb_binlog_file(xtb_binlog_file_name)
            if info:
                return info

        if self.is_master():
            data = self.mysql('-e', 'SHOW MASTER STATUS').split()
            return data[0], data[1]

        return None, None

    def configure_slave_replication(self, binlog_file, binlog_pos):
        logging.info('Initializing replication from clone position.')
        query = f"""
        CHANGE MASTER TO MASTER_LOG_FILE='{binlog_file}',
        MASTER_LOG_POS={binlog_pos},
        MASTER_HOST='{self.master_host}',
        MASTER_USER='{settings.MASTER_REPLICATION_USER}',
        MASTER_PASSWORD='{settings.MASTER_REPLICATION_PASSWORD}',
        MASTER_CONNECT_RETRY={settings.MASTER_CONNECT_RETRY};

        START SLAVE;
        """

        self.mysql(_in=query)

    def configure_slave(self):
        """Configure slave replication."""
        if self.is_master():
            return

        binlog_file, binlog_pos = self.get_binlog_position()
        if binlog_file:
            logging.info('Waiting for mysqld to be ready (accepting connections)')
            while not self.is_ready():
                time.sleep(1)
            self.configure_slave_replication(binlog_file, binlog_pos)

    def configure_master(self):
        """Configure master replication. Create user for replication."""
        if not self.is_master():
            return

        while not self.is_ready():
            time.sleep(1)

        # This query will create the user if not exists.
        self.mysql(_in=(
            f"GRANT SELECT, PROCESS, RELOAD, LOCK TABLES, REPLICATION CLIENT, REPLICATION SLAVE ON *.* "
            f"TO '{settings.MASTER_REPLICATION_USER}'@'%' "  # TODO: limit to: %.{settings.GOVERNING_SERVICE}
            f"IDENTIFIED BY '{settings.MASTER_REPLICATION_PASSWORD}'"
        ))
