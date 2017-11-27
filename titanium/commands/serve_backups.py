# pylint: disable=no-member
import logging

import sh

from .base import BaseCommand
from titanium import settings


class Command(BaseCommand):
    """This command will config the mysql."""
    def add_arguments(self, parser):
        pass

    def handle(self):
        logging.info("Starting the backup server...")

        xtrabackup_cmd = [
            'xtrabackup', '--backup', '--slave-info', '--stream=xbstream', '--host=127.0.0.1',
            f'--user={settings.MASTER_REPLICATION_USER}', f'--password={settings.MASTER_REPLICATION_PASSWORD}'
        ]

        sh.ncat(
            '--listen', '--keep-open', '--send-only', '--max-conns=1',
            settings.EXPOSE_BACKUPS_PORT, '-c', ' '.join(xtrabackup_cmd)
        )
