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


def sizeof_fmt(num, suffix='B'):
    for unit in ['','K','M','G','T']:
        if abs(num) < 1024.0:
            return "%d%s%s" % (num, unit, suffix)
        num /= 1024.0
    return "%d%s%s" % (num, 'P', suffix)


def get_innodb_buffer_pool_size():
    file_name = '/mnt/podinfo/mem_request'
    if not os.path.exists(file_name):
        return settings.INNODB_BUFFER_POOL_SIZE_DEFAULT

    with open(file_name, 'r') as f:
        try:
            content = f.read()
            size = int(content)
            size = size * 80 / 100
            return sizeof_fmt(size)
        except ValueError:
            print("MEM_REQUEST file content: ", content)
            raise

