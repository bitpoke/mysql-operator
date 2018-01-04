import logging
import os.path
from logging.config import dictConfig

from sh import contrib

from titanium import settings


# Configure logging
dictConfig(settings.LOGGING)


# configure sentry
if settings.SENTRY_DSN:
    from raven.handlers.logging import SentryHandler
    from raven.conf import setup_logging
    handler = SentryHandler(settings.SENTRY_DSN)
    handler.setLevel(logging.ERROR)
    setup_logging(handler)


@contrib('rclone')
def rclone_cmd(rclone):
    config_file = os.path.join(settings.CONFIG_MAP_DIR, 'rclone.conf')
    if not os.path.exists(config_file):
        logging.warning(f'RCLONE config file ({config_file}) does not exists.')

    return rclone.bake(f'--config={config_file}')
