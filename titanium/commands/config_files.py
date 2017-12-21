import logging

from .base import BaseCommand
from titanium.mysql import ConfigPhase


class Command(BaseCommand):
    """This command will config the mysql."""
    def add_arguments(self, parser):
        pass

    def handle(self):
        logging.info("Configure node.")
        node = ConfigPhase()
        node.create_config_files()
