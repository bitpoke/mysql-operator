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
    if not os.path.exists(settings.RCLONE_CONFIG_FILE):
        logging.warning(f'RCLONE config file ({settings.RCLONE_CONFIG_FILE}) does not exists.')

    return rclone.bake(f'--config={settings.RCLONE_CONFIG_FILE}')
