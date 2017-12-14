# pylint: disable=no-member
import logging
from datetime import datetime
from random import choice
from string import ascii_uppercase

import sh
from sh.contrib import rclone  # pylint: disable=import-error

from .base import BaseCommand
from titanium import settings


class Command(BaseCommand):
    """This command will config the mysql."""
    def add_arguments(self, parser):
        parser.add_argument('--current-pod', '-c', action='store_true', default=False,
                            help='Use current pod for backup.')

    def handle(self, current_pod=False):
        logging.info("Starting backup process...")
        if not settings.BACKUP_BUCKET:
            logging.error("BACKUP_BUCKET is not configured.")
            return
        if not settings.BACKUP_PREFIX:
            logging.error("TITANIUM_RELEASE_NAME or TITANIUM_BACKUP_PREFIX is not configured.")
            return

        self.backup(current_pod)

    def get_backup_uri(self):
        date = datetime.utcnow().isoformat(timespec='minutes')
        rand_str = ''.join(choice(ascii_uppercase) for i in range(7))
        return f'{settings.BACKUP_BUCKET}/{settings.BACKUP_PREFIX}/{date}-{rand_str}.xbackup.gz'

    def get_source_host(self, use_current_pod):
        """Get the master mysql hostname."""
        if use_current_pod:
            return '127.0.0.1'

        return f'{settings.RELEASE_NAME}-0.{settings.GOVERNING_SERVICE}'

    def backup(self, use_current_pod):
        """If BACKUP_BUCKET is set this will push backups to that bucket."""

        backup_uri = self.get_backup_uri()
        rclone.rcat(
            sh.gzip(sh.ncat('--recv-only', self.get_source_host(use_current_pod),
                            settings.EXPOSE_BACKUPS_PORT, _piped=True),
                    '-c', _piped=True),
            backup_uri
        )
        logging.info(f'Backup done. URI: {backup_uri}')

        # use print to send to stdout
        print(f'Backup done. URI: {backup_uri}')

