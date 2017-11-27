# pylint: disable=no-member
import socket
import time
import json
import tempfile
import os
from random import choice
from string import ascii_lowercase
import threading

import pytest
import yaml
import sh
from kubernetes import client, config, watch
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
    },
    'scheduleBackup': None
}
config.load_kube_config()


def pytest_addoption(parser):
    parser.addoption("--deploy", action="store_true", default=False,
                     help="Build image and publish it to GS. (Default: false)")


def deploy():
    print(f'Start building image {TITANIUM_IMAGE}...')
    sh.docker.build('-t', TITANIUM_IMAGE, '.')
    print(f'Pusing image to GS...')
    sh.gcloud.docker('--', 'push', TITANIUM_IMAGE)


class Release:
    def __init__(self, release, namespace=NAMESPACE, values=None):
        self.release = release
        self.namespace = namespace
        self.values = {
            **INIT_CONFIG,
            **(values or {})
        }
        self.v1 = client.CoreV1Api()

    def apply(self):
        with tempfile.NamedTemporaryFile('w+', suffix='.yaml') as config_file:
            config_file.write(yaml.dump(self.values))
            config_file.flush()

            out = sh.helm.install('-f', config_file.name, '--name', self.release, '--debug',
                                  '--namespace', self.namespace, CHART_PATH)
            print(out)

    def delete(self):
        sh.helm.delete('--purge', self.release)

    def execute(self, pod, cmd):
        return sh.kubectl.exec('-it', f'{self.release}-titanium-{pod}', '--', cmd)

    def pod_forward_ports(self, pod, ports):
        ports = map(str, ports)
        print('Starting port forwarding...')
        process = sh.kubectl('port-forward', '--namespace', self.namespace,
                          f'{self.release}-titanium-{pod}', ' '.join(ports),
                          _bg=True)
        return process


    def wait_for_pod1(self, pod_ordinal, desired_state='Running'):
        def parse_output(data, _, process):
            print(f'EVENT: {data}')
            if data.strip() == desired_state:
                print('KILL PROCESS')
                process.terminate()

        print(f'Waiting for pod {pod_ordinal}...')
        try:
            sh.kubectl.get.pod(f'{self.release}-titanium-{pod_ordinal}', '--namespace', self.namespace,
                               '--watch=true', '--output=go-template={{ .status.phase }}\n', _out=parse_output,
                               _ok_code=[0,1])
        except sh.SignalException_SIGTERM:
            print('Killed')

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
        return self.v1.read_namespaced_pod_status(
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

        self.connect_to_pod(0)

    def connect_to_pod(self, pod):
        print(f'Connect to pod {pod} ...')
        if self.init:
            self.conn.close()
            self.p.terminate()

        self.p = self.release.pod_forward_ports(pod, [3306])
        self._connect_to_mysql()

    @backoff.on_exception(backoff.expo, pymysql.err.OperationalError, max_tries=10)
    def _connect_to_mysql(self):
        print('Trying to connect to MYSQL...')
        self.conn = pymysql.connect(
            host='127.0.0.1', user='root'
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
        self.conn.close()
        self.p.terminate()


@pytest.fixture(scope='session')
def db(helm):
    release = helm.install(values={
        'mysql': {
            'replicas': 2
        }
    })

    release.wait_for_pod(0)
    release.wait_for_pod(1)

    db_fixture = DBFixture(release)
    yield db_fixture
    db_fixture.cleanup()
