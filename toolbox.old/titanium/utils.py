import os
import socket

from titanium import settings


def hostname():
    """Get hostname."""
    return socket.gethostname()


def get_ordinal():
    """Compute the ordinal from hostname."""
    host = hostname()
    # <statefulset-name>-<ordinal-index>
    return int(host.rsplit('-', 1)[1])


def get_host_for(ordinal):
    """Compound the host based on context."""
    host = hostname()
    base = host.rsplit('-', 1)[0]

    return f'{base}-{ordinal}.{settings.GOVERNING_SERVICE}'
