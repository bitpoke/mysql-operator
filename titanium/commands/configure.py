import logging

from .base import BaseCommand
from titanium.mysql import RunningPhase


class Command(BaseCommand):
    """This command will config the mysql."""
    def add_arguments(self, parser):
        pass

    def handle(self):
        logging.info("Configure mysql replication...")
        node = RunningPhase()
        node.configure()
