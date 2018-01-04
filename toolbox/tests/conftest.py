# pylint: disable=no-member,redefined-outer-name
import time
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
TITANIUM_IMAGE_TAG = os.getenv('TITANIUM_IMAGE_TAG', 'latest')
NAMESPACE = 'titanium-testing'
CHART_PATH = os.path.join(os.path.dirname(__file__), '../charts/titanium/')
INIT_CONFIG = {
    'gsCredentialsFile': os.getenv('TITANIUM_TEST_GS_CREDENTIALS', os.getenv('GOOGLE_CREDENTIALS', '')),
    # 'initBucketURI': None,
    'backupBucket': 'gs:pl-test-mysql-backups',
    'persistenceDisabled': True,  # for fasts tests
    'mysql': {
        'replicas': 2,
        #'allowEmptyPassword': True,
        'rootPassword': 'supersecret'
    },
    'scheduleBackup': None,
    'tools': {
        'image': '{}:{}'.format(TITANIUM_IMAGE, TITANIUM_IMAGE_TAG)
    }
}
config.load_kube_config()


def pytest_addoption(parser):
    parser.addoption("--deploy", action="store_true", default=False,
                     help="Build image and publish it to GS.")


def deploy():
    """Helper for local development."""
    print('\nStart building image {} ...'.format(TITANIUM_IMAGE))
    sh.docker.build('-t', TITANIUM_IMAGE, '.')
    print('Pushing docker image to GS ...')
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
        print('Tools image: ', self.values['tools']['image'])

        self.mysql_password = self.values['mysql'].get('rootPassword', '')
        self.kubeV1 = client.CoreV1Api()
        self.no_pods = self.values['mysql']['replicas']

    def apply(self):
        with tempfile.NamedTemporaryFile('w+', suffix='.yaml') as config_file:
            config_file.write(yaml.dump(self.values))
            config_file.flush()

            out = sh.helm.install('-f', config_file.name, '--name', self.release, '--debug',
                                  '--namespace', self.namespace, CHART_PATH)
            print('\n'.join(out.stdout.decode('utf-8').splitlines()[-25:]))

    def delete(self):
        sh.helm.delete('--purge', self.release, '--timeout', 10, '--no-hooks')

    def execute(self, pod, cmd, container='titanium'):
        return sh.kubectl.exec('--namespace', NAMESPACE, '-it', '{}-titanium-{}'.format(self.release, pod),
                               '-c', container, '--', cmd, _tty_out=False, _tty_in=False)

    def pod_forward_ports(self, pod, ports):
        ports = map(str, ports)
        print('Starting port forwarding...')
        process = sh.kubectl('port-forward', '--namespace', self.namespace,
                          '{}-titanium-{}'.format(self.release, pod), ' '.join(ports),
                          _bg=True)
        return process

    def get_logs(self, pod, container='mysql'):
        try:
            out = sh.kubectl.logs(
                '--namespace', NAMESPACE, '{}-titanium-{}'.format(self.release, pod),
                '-c', container
            )
            return out.stdout.decode('utf-8')
        except sh.ErrorReturnCode:
            return 'Fail to fetch logs.'

    def all_logs(self):
        for pod in range(self.no_pods):
            for container in ['init-mysql', 'clone-mysql', 'mysql', 'titanium']:
                print('\n=== Logs for: {}-titanium-{} - {} ==='.format(self.release, pod, container))
                print(self.get_logs(pod, container))

    @backoff.on_predicate(backoff.fibo, max_value=10)
    @backoff.on_exception(backoff.expo, ApiException, max_tries=8)
    def wait_for_pod(self, pod_ordinal, desired_state='Running'):
        pod = self.get_pod_status(pod_ordinal)
        if pod.status.phase == desired_state:
            return True
        return False

    @backoff.on_exception(backoff.expo, ApiException, max_tries=3)
    def get_pod_status(self, pod):
        return self.kubeV1.read_namespaced_pod_status(
            '{}-titanium-{}'.format(self.release, pod), self.namespace
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

    def print_all_logs(self):
        for release in self._releases:
            release.all_logs()

    def cleanup(self):
        for release in self._releases:
            release.delete()


class DBFixture:
    fixtures = []

    def __init__(self, release):
        self.release = release
        self.forward_process = None
        self.conn = None
        self.init = False
        self.fixtures.append(self)

    def __call__(self, release):
        self.disconnect()
        time.sleep(1)  # wait until process is killed
        return DBFixture(release)

    def connect_to_pod(self, pod, user=None, password=None):
        print('Connect to pod {} ...'.format(pod))
        if self.init:
            self.conn.close()
            self.forward_process.terminate()
            self.init = False

        self.forward_process = self.release.pod_forward_ports(pod, [3306])
        self._connect_to_mysql(user, password)

    @backoff.on_exception(backoff.expo, pymysql.err.OperationalError, max_tries=10)
    def _connect_to_mysql(self, user=None, password=None):
        print('Trying to connect to MYSQL...')
        self.conn = pymysql.connect(
            host='127.0.0.1', user=(user or 'root'),
            password=(password or self.release.mysql_password)
        )
        self.init = True

    def disconnect(self):
        if not self.init:
            return
        try:
            self.conn.close()
        except pymysql.err.Error:
            pass
        try:
            self.forward_process.terminate()
        except ProcessLookupError:
            pass

        self.init = False

    def create_db(self, name):
        self.query('CREATE DATABASE {};'.format(name))
        self.conn.select_db(name)

    def use_db(self, name):
        self.conn.select_db(name)

    def query(self, query, values=None, size=1):
        if values:
            with self.conn.cursor() as cursor:
                cursor.execute(query, values)
            self.conn.commit()
            return
        with self.conn.cursor() as cursor:
            cursor.execute(query, values)
            return cursor.fetchmany(size)

    @classmethod
    def cleanup(cls):
        for fixture in cls.fixtures:
            fixture.disconnect()


@pytest.fixture(autouse=True, scope='session')
def helm(request):
    if request.config.getoption('--deploy'):
        # trigger deploy
        deploy()

    helm_client = Helm()
    yield helm_client

    if request.session.testsfailed:
        helm_client.print_all_logs()
    helm_client.cleanup()


@pytest.fixture(scope='session')
def release(helm):
    release = helm.install()
    try:
        release.wait_for_pod(0)
        release.wait_for_pod(1)
    except ApiException:
        helm.cleanup()
        raise

    return release


@pytest.fixture()
def db():
    yield DBFixture
    DBFixture.cleanup()
