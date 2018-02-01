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


TITANIUM_IMAGE = 'gcr.io/pl-infra/titanium-toolbox:{}'.format(
    os.getenv('TITANIUM_IMAGE_TAG', 'latest'))
NAMESPACE = 'titanium-testing'
OPERATOR_EXEC = '../bin/linux/operator'
BUCKET_SECRET_NAME = 'backups-secret-for-gs'
POD_NAME_TEMPLATE = '{}-mysql-{}'

def MYSQL_CLUSTER_CONFIG(name, **spec):
    return {
        'apiVersion': 'titanium.presslabs.net/v1alpha1',
        'kind': 'MysqlCluster',
        'metadata': {
            'name': name,
            'namespace': NAMESPACE,
        },
        'spec': {
            'replicas': 2,
            'mysqlRootPassword': 'supersecret',
            'backupBucketURI': 'gs:pl-test-mysql-backups/{}/'.format(name),
            'backupBucketSecretName': BUCKET_SECRET_NAME,
            'initBucketSecretName': BUCKET_SECRET_NAME,
            'podSpec':{
                'titaniumImage': TITANIUM_IMAGE,
            },
            'volumeSpec': {
                'persistenceDisabled': True, # for speeding up the test
            },
            **spec
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
    def __init__(self, release, values=None):
        self.name = release
        values = values or {}

        self.values = MYSQL_CLUSTER_CONFIG(release, **values)

        print('Tools image: ', self.values['spec']['podSpec']['titaniumImage'])

        self.mysql_password = self.values['spec'].get('mysqlRootPassword', '')
        self.kubeV1 = client.CoreV1Api()
        self.no_pods = self.values['spec']['replicas']

    def apply(self):
        with tempfile.NamedTemporaryFile('w+', suffix='.yaml') as config_file:
            config_file.write(yaml.dump(self.values))
            config_file.flush()

            out = sh.kubectl.apply('-f', config_file.name)
            print('\n'.join(out.stdout.decode('utf-8').splitlines()[-25:]))

    def delete(self):
        sh.kubectl.delete.mysql(self.name, '--namespace', NAMESPACE)

    def execute(self, pod, cmd, container='titanium'):
        try:
            return sh.kubectl.exec('--namespace', NAMESPACE, '-it',
                                POD_NAME_TEMPLATE.format(self.name, pod),
                                '-c', container, '--', cmd, _tty_out=False, _tty_in=False)
        except sh.ErrorReturnCode as e:
            print('More info: ', e.stderr)
            raise

    def pod_forward_ports(self, pod, ports):
        ports = map(str, ports)
        print('Starting port forwarding...')
        process = sh.kubectl('port-forward', '--namespace', NAMESPACE,
                          POD_NAME_TEMPLATE.format(self.name, pod), ' '.join(ports),
                          _bg=True)
        return process

    def get_logs(self, pod, container='mysql'):
        try:
            out = sh.kubectl.logs(
                '--namespace', NAMESPACE, POD_NAME_TEMPLATE.format(self.name, pod),
                '-c', container
            )
            return out.stdout.decode('utf-8')
        except sh.ErrorReturnCode:
            return 'Fail to fetch logs.'

    def all_logs(self):
        for pod in range(self.no_pods):
            for container in ['init-mysql', 'clone-mysql', 'mysql', 'titanium']:
                pod_name = POD_NAME_TEMPLATE.format(self.name, pod)
                print('\n=== Logs for: {} - {} ==='.format(
                    pod_name, container))
                print(self.get_logs(pod, container))

    @backoff.on_predicate(backoff.fibo, max_value=5)
    @backoff.on_exception(backoff.expo, ApiException, max_tries=7)
    def wait_for_pod(self, pod_ordinal, desired_state='Running'):
        print('Wait for pod({})...'.format(pod_ordinal))
        pod = self.get_pod_status(pod_ordinal)
        if pod.status.phase == desired_state:
            return True
        return False

    @backoff.on_exception(backoff.expo, ApiException, max_tries=3)
    def get_pod_status(self, pod):
        print('Get status pod({})'.format(pod))
        return self.kubeV1.read_namespaced_pod_status(
            POD_NAME_TEMPLATE.format(self.name, pod), NAMESPACE
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
        self._port = 2000 + (abs(hash(self.release.name)) % (10**4))
        self.connected_with_pod = -1

    def get_port(self):
        return self._port + self.connected_with_pod

    def __call__(self, release):
        self.disconnect()
        time.sleep(2)  # wait until process is killed
        return DBFixture(release)

    def connect_to_pod(self, pod, user=None, password=None):
        print('Connect to pod {} ...'.format(pod))
        self.disconnect()
        self.connected_with_pod = pod
        try:
            self.forward_process = self.release.pod_forward_ports(pod, ['{}:3306'.format(self.get_port())])
            self._connect_to_mysql(user, password)
            self.init = True
        except pymysql.err.OperationalError:
            self._stop_port_forward()
            raise

    @backoff.on_exception(backoff.expo, pymysql.err.OperationalError, max_tries=9)
    def _connect_to_mysql(self, user=None, password=None):
        print('Trying to connect to MYSQL port {}...'.format(self.get_port()))
        self.conn = pymysql.connect(
            host='127.0.0.1', port=self.get_port(), user=(user or 'root'),
            password=(password or self.release.mysql_password)
        )

    def disconnect(self):
        if not self.init:
            return

        self._close_connection()
        self._stop_port_forward()

        self.connected_with_pod = -1
        self.init = False

    def _stop_port_forward(self):
        print('Port forward stoped ({}:3306)'.format(self.get_port()))
        try:
            self.forward_process.terminate()
        except ProcessLookupError:
            pass

    def _close_connection(self):
        print('Close connection to pod {}'.format(self.connected_with_pod))

        try:
            self.conn.close()
        except pymysql.err.Error:
            pass

    def create_db(self, name):
        self.query('CREATE DATABASE {};'.format(name))
        self.conn.select_db(name)

    @backoff.on_exception(backoff.expo, pymysql.err.InternalError, max_tries=4)
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
