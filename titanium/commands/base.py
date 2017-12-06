import sys
import logging


logger = logging.getLogger()


class BaseCommand(object):
    """Base command."""

    def __init__(self, parser):
        self.add_arguments(parser)
        self.parser = parser

    def add_arguments(self, parser):
        pass

    def handle(self, **args):
        raise NotImplementedError

    def __call__(self, args=None):
        _args = vars(args)
        del _args['func']

        try:
            self.handle(**_args)
        except Exception as e:  # pylint: disable=broad-except
            logger.exception(e)
            sys.exit(1)
