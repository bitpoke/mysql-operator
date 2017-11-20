import logging

from .base import BaseCommand
from titanium.mysql import MysqlNode


class Command(BaseCommand):
    """This command will config the mysql."""
    def add_arguments(self, parser):
        pass

    def handle(self):
        logging.info("Starting the backup server...")
        node = MysqlNode()

        node.configure_slave()
        node.expose_backup()
