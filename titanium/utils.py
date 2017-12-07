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


def parse_slave_info_xtb_file(xtb_file_name):
    """
    Parse file xtrabackup_slave_info.

    File content example:
    CHANGE MASTER TO MASTER_LOG_FILE='my-titanium-0-bin.000009', MASTER_LOG_POS=154
    """
    with open(xtb_file_name, 'r') as f:
        try:
            content = f.read()
            pairs = {pair.split('=')[0].strip(): pair.split('=')[1].strip() for pair in content.split(',')}
            return pairs['CHANGE MASTER TO MASTER_LOG_FILE'], pairs['MASTER_LOG_POS']
        except IndexError:
            print(f'File {xtb_file_name} contains: {content}')


def parse_xtb_binlog_file(xtb_file_name):
    """
    Parse file xtrabackup_binlog_info.

    File example:
    dthhwim-titanium-0-bin.000003	154
    """
    with open(xtb_file_name, 'r') as f:
        try:
            content = f.read()
            result = content.split()
            return result[0], result[1]
        except IndexError:
            print("XTB_FILE content: ", content)


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

