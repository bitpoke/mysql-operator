# pylint: disable=no-member
import logging
from datetime import datetime
from random import choice
from string import ascii_uppercase

import sh
from sh.contrib import rclone  # pylint: disable=import-error

from .base import BaseCommand
from titanium import settings
from titanium.utils import hostname


class Command(BaseCommand):
    """This command will config the mysql."""
    def add_arguments(self, parser):
        pass

    def handle(self):
        logging.info("Starting backup process...")
        if not settings.BACKUP_BUCKET:
            logging.error("BACKUP_BUCKET is not configured.")
            return
        if not settings.PROJECT_NAME:
            logging.error("PROJECT_NAME is not configured.")
            return

        self.backup()

    def get_backup_uri(self):
        date = datetime.utcnow().isoformat(timespec='minutes')
        rand_str = ''.join(choice(ascii_uppercase) for i in range(7))
        return f'{settings.BACKUP_BUCKET}/{settings.PROJECT_NAME}/{date}-{rand_str}.xbackup.gz'

    def get_source_host(self):
        """Get the master mysql hostname."""
        return f'{settings.PROJECT_NAME}-0.{settings.GOVERNING_SERVICE}'

    def backup(self):
        """If BACKUP_BUCKET is set this will push backups to that bucket."""

        rclone.rcat(
            sh.gzip(sh.ncat('--recv-only', self.get_source_host(), settings.EXPOSE_BACKUPS_PORT, _piped=True),
                    '-c', _piped=True),
            self.get_backup_uri()
        )
