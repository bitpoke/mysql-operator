# pylint: disable=no-member
import tempfile
import os
from random import choice
from string import ascii_lowercase

import pytest
import yaml
import sh
from kubernetes import client, config
from kubernetes.client.rest import ApiException
import pymysql
import backoff


TITANIUM_IMAGE = 'gcr.io/pl-infra/titanium-toolbox'
NAMESPACE = 'titanium-testing'
CHART_PATH = os.path.join(os.path.dirname(__file__), '../chart/')
INIT_CONFIG = {
    'gsCredentialsFile': os.getenv('TITANIUM_TEST_GS_CREDENTIALS',''),
    # 'initBucketURI': None,
    'backupBucket': 'gs:pl-test-mysql-backups',
    'persistenceDisabled': True,  # for fasts tests
    'mysql': {
        'replicas': 2,
        #'allowEmptyPassword': True,
        'rootPassword': 'supersecret'
    },
    'scheduleBackup': None
}
config.load_kube_config()


def pytest_addoption(parser):
    parser.addoption("--deploy", action="store_true", default=False,
                     help="Build image and publish it to GS.")


def deploy():
    """Helper for local development."""
    print(f'\nStart building image {TITANIUM_IMAGE} ...')
    sh.docker.build('-t', TITANIUM_IMAGE, '.')
    print(f'Pushing docker image to GS ...')
    sh.gcloud.docker('--', 'push', TITANIUM_IMAGE)


class Release:
    def __init__(self, release, namespace=NAMESPACE, values=None):
        self.release = release
        self.namespace = namespace
        values = values or {}

        self.values = {
            **INIT_CONFIG,
            **values
        }
        if 'mysql' in values:
            self.values['mysql'] = {
                **INIT_CONFIG['mysql'],
                **values['mysql']
            }

        self.mysql_password = self.values['mysql'].get('rootPassword', '')
        self.kubeV1 = client.CoreV1Api()

    def apply(self):
        with tempfile.NamedTemporaryFile('w+', suffix='.yaml') as config_file:
            config_file.write(yaml.dump(self.values))
            config_file.flush()

            out = sh.helm.install('-f', config_file.name, '--name', self.release, '--debug',
                                  '--namespace', self.namespace, CHART_PATH)
            print(out)

    def delete(self):
        sh.helm.delete('--purge', self.release, '--timeout', 10, '--no-hooks')

    def execute(self, pod, cmd, container='titanium'):
        return sh.kubectl.exec('--namespace', NAMESPACE, '-it', f'{self.release}-titanium-{pod}',
                               '-c', container, '--', cmd, _tty_out=False, _tty_in=False)

    def pod_forward_ports(self, pod, ports):
        ports = map(str, ports)
        print('Starting port forwarding...')
        process = sh.kubectl('port-forward', '--namespace', self.namespace,
                          f'{self.release}-titanium-{pod}', ' '.join(ports),
                          _bg=True)
        return process

    @backoff.on_predicate(backoff.fibo, max_value=15)
    @backoff.on_exception(backoff.expo, ApiException, max_tries=8)
    def wait_for_pod(self, pod_ordinal, desired_state='Running'):
        pod = self.get_pod_status(pod_ordinal)
        print(f'POD[{pod_ordinal}]: {pod.status.phase}')
        if pod.status.phase == desired_state:
            return True
        return False

    @backoff.on_exception(backoff.expo, ApiException, max_tries=3)
    def get_pod_status(self, pod):
        return self.kubeV1.read_namespaced_pod_status(
            f'{self.release}-titanium-{pod}', self.namespace
        )


class Helm:
    def __init__(self):
        self._releases = []

    def install(self, values=None, release_name=None):
        name = release_name or ''.join(choice(ascii_lowercase) for i in range(7))
        release = Release(name, values=values)
        release.apply()
        self._releases.append(release)
        return release

    def cleanup(self):
        for release in self._releases:
            release.delete()


@pytest.fixture(autouse=True, scope='session')
def helm(request):
    if request.config.getoption('--deploy'):
        # trigger deploy
        deploy()

    helm_client = Helm()
    yield helm_client
    helm_client.cleanup()


class DBFixture:
    def __init__(self, release):
        self.release = release
        self.p = None
        self.conn = None
        self.init = False

    def connect_to_pod(self, pod, user=None, password=None):
        print(f'Connect to pod {pod} ...')
        if self.init:
            self.conn.close()
            self.p.terminate()
            self.init = False

        self.p = self.release.pod_forward_ports(pod, [3306])
        self._connect_to_mysql(user, password)

    @backoff.on_exception(backoff.expo, pymysql.err.OperationalError, max_tries=10)
    def _connect_to_mysql(self, user=None, password=None):
        print('Trying to connect to MYSQL...')
        self.conn = pymysql.connect(
            host='127.0.0.1', user=(user or 'root'),
            password=(password or self.release.mysql_password)
        )
        self.init = True

    def create_db(self, name):
        self.query(f'CREATE DATABASE {name};')
        self.conn.select_db(name)

    def use_db(self, name):
        self.conn.select_db(name)

    def insertQ(self, query, values):
        with self.conn.cursor() as cursor:
            cursor.execute(query, values)
        self.conn.commit()

    def query(self, query, size=1):
        with self.conn.cursor() as cursor:
            cursor.execute(query)
            return cursor.fetchmany(size)

    def cleanup(self):
        try:
            self.conn.close()
            self.p.terminate()
        except (pymysql.err.Error, ProcessLookupError):
            pass



@pytest.fixture(scope='session')
def db(helm):
    release = helm.install()

    release.wait_for_pod(0)
    release.wait_for_pod(1)

    db_fixture = DBFixture(release)
    yield db_fixture
    db_fixture.cleanup()
