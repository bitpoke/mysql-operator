# pylint: disable=no-member
import time
import logging
import os.path
from shutil import copyfile

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

        self.mysql = sh.mysql.bake('-NB', '-h', '127.0.0.1')

    def is_master(self):
        return self.ordinal == 0

    def exists_data(self):
        return os.path.exists(os.path.join(settings.MYSQL_DATA_DIR, 'mysql'))

    def is_ready(self):
        try:
            return self.mysql('-e', 'SELECT 1') == '1'
        except sh.ErrorReturnCode:
            return False

    def configure(self):
        """Writes config file for this node."""
        with open(os.path.join(settings.CONFIG_DIR, 'server-id.cnf'), 'w+') as f:
            f.write(
                '[mysqld]\n'
                f'server-id={self.server_id}\n'
            )

        src_config_file_name = 'master.cnf' if self.is_master() else 'slave.cnf'
        src_path = os.path.join(settings.CONFIG_MAP_DIR, src_config_file_name)
        dest_path = os.path.join(settings.CONFIG_DIR, src_config_file_name)
        copyfile(src_path, dest_path)

        # TODO: create replication user if not exists.

    def clone(self):
        """Clone data from source."""
        # Skip the clone if data already exists or is master.
        if self.exists_data():
            return

        if self.is_master():
            if not settings.INIT_BUCKET_URI:
                return
            # if is a master node and INIT_BUCKET_URI is set then clone for storage.
            self.get_data_from_storages()
        else:
            self.get_data_from_source_node()

        sh.xtrabackup(
            '--prepare', f'--target-dir={settings.MYSQL_DATA_DIR}',
            '--user={settings.MASTER_REPLICATION_USER}',
            '--password={settings.MASTER_REPLICATION_PASSWORD}'
        )

    def get_data_from_source_node(self):
        sh.xbstream(
            sh.ncat('--recv-only', self.source_host, settings.EXPOSE_BACKUPS_PORT, _piped=True),
            '-x', '-C', settings.MYSQL_DATA_DIR,
        )

    def get_data_from_storages(self):
        """Get data from INIT buckets specified in settings.INIT_BUCKET_URI."""
        sh.gzip(rclone.cat(settings.INIT_BUCKET_URI, _piped=True), '-d', _cwd=settings.MYSQL_DATA_DIR)

    def get_binlog_position(self):
        """
        Determine binlog position from cloned data.

        Returns: <binlog file name>, <binlog position>
        """
        xtb_slave_info_file_name = os.path.join(settings.MYSQL_DATA_DIR, 'xtrabackup_slave_info')
        if os.path.exists(xtb_slave_info_file_name):
            return utils.parse_slave_info_xtb_file(xtb_slave_info_file_name)

        xtb_binlog_file_name = os.path.join(settings.MYSQL_DATA_DIR, 'xtrabackup_binlog_info')
        if os.path.exists(xtb_binlog_file_name):
            # data cloned from master
            return utils.parse_xtb_binlog_file(xtb_binlog_file_name)

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

    def expose_backup(self):
        """Run xtrabackup for backups."""

        xtrabackup_cmd = [
            'xtrabackup', '--backup', '--slave-info', '--stream=xbstream', '--host=127.0.0.1',
            f'--user={settings.MASTER_REPLICATION_USER}', f'--password={settings.MASTER_REPLICATION_PASSWORD}'
        ]

        sh.ncat(
            '--listen', '--keep-open', '--send-only', '--max-conns=1',
            settings.EXPOSE_BACKUPS_PORT, '-c', ' '.join(xtrabackup_cmd)
        )
