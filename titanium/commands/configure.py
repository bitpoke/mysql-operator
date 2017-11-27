import logging

from .base import BaseCommand
from titanium.mysql import MysqlNode


class Command(BaseCommand):
    """This command will config the mysql."""
    def add_arguments(self, parser):
        pass

    def handle(self):
        logging.info("Configure mysql replication...")
        node = MysqlNode()

        if node.is_master():
            logging.info('Configuring MASTER node.')
            node.configure_master()
        else:
            logging.info('Configuring SLAVE node.')
            node.configure_slave()

