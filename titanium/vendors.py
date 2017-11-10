import logging
from logging.config import dictConfig


from titanium.settings import SENTRY_DSN, LOGGING


# Configure logging
dictConfig(LOGGING)


# configure sentry
if SENTRY_DSN:
    from raven.handlers.logging import SentryHandler
    from raven.conf import setup_logging
    handler = SentryHandler(SENTRY_DSN)
    handler.setLevel(logging.ERROR)
    setup_logging(handler)
