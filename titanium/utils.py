import re
import socket
from shutil import copyfile

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


def parse_slave_info_xtb_file(xtb_file_name):
    """
    Parse file xtrabackup_slave_info.

    File content example:
    CHANGE MASTER TO MASTER_LOG_FILE='my-titanium-0-bin.000009', MASTER_LOG_POS=154
    """
    with open(xtb_file_name, 'r') as f:
        pairs = {pair.split('=')[0].strip(): pair.split('=')[1].strip() for pair in f.read().split(',')}
        return pairs['CHANGE MASTER TO MASTER_LOG_FILE'], pairs['MASTER_LOG_POS']


def parse_xtb_binlog_file(xtb_file_name):
    """
    Parse file xtrabackup_binlog_info.

    File example:
    dthhwim-titanium-0-bin.000003	154
    """
    with open(xtb_file_name, 'r') as f:
        try:
            result = f.read().split()
            return result[0], result[1]
        except IndexError:
            f.seek(0)
            print("XTB_FILE content: ", f.read())
            raise
