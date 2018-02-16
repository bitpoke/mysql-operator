import pytest


def test_mysql_running_replication(db, release):
    db = db(release)
    db.connect_to_pod(0)
    db.create_db('test_1')
    db.query("""
    CREATE TABLE `test` (
        `id` int(11) NOT NULL AUTO_INCREMENT,
        `name` varchar(255) COLLATE utf8_bin NOT NULL,
        `value` varchar(255) COLLATE utf8_bin NOT NULL,
        PRIMARY KEY (`id`)
    );
    """)

    expected_values = ('n1', 'n2')
    db.query(
        "INSERT INTO `test` (name, value) VALUES (%s, %s)",
        expected_values
    )

    actual_values = db.query('SELECT name, value FROM test')
    assert actual_values[0] == expected_values

    # Test replication
    db.connect_to_pod(1)
    db.use_db('test_1')

    actual_values = db.query('SELECT name, value FROM test')
    assert actual_values[0] == expected_values


def test_backup_on_demand(helm, release, db):
    db = db(release)
    db.connect_to_pod(0)
    db.create_db('test_2')
    db.query("""
    CREATE TABLE `test` (
        `id` int(11) NOT NULL AUTO_INCREMENT,
        `name` varchar(255) COLLATE utf8_bin NOT NULL,
        `value` varchar(255) COLLATE utf8_bin NOT NULL,
        PRIMARY KEY (`id`)
    );
    """)
    expected_values = ('n1', 'n2')
    db.query(
        "INSERT INTO `test` (name, value) VALUES (%s, %s)",
        expected_values
    )

    out = release.execute(0, ['db', 'backup'])
    # out: Backup done. URI: gs:pl-test-mysql-backups/rrkjtce-titanium/2017-11-28T13:43-HUJICLA.xbackup.gz
    backup_uri = out.stdout.strip().split(b':', 1)[1].strip()
    backup_uri = backup_uri.decode('utf-8')

    assert backup_uri.startswith('gs:')
    assert 'pl-test-mysql-backups' in backup_uri
    db.cleanup()

    release2 = helm.install({
        'initBucketURI': backup_uri,
        'replicas': 1,
    })
    release2.wait_for_pod(0)

    db2 = db(release2)
    db2.connect_to_pod(0)
    db2.use_db('test_2')

    actual_values = db2.query('SELECT name, value FROM test')
    assert actual_values[0] == expected_values


def test_user_creation(helm, db):
    db_name, db_user, db_pass = 'xxTestxx', 'xxUserxx', 'xxPaSSxx'
    release = helm.install({
        'replicas': 1,
        'mysqlDatabase': db_name,
        'mysqlUser': db_user,
        'mysqlPassword': db_pass
    })
    release.wait_for_pod(0)

    db = db(release)
    db.connect_to_pod(0, db_user, db_pass)
    db.use_db(db_name)
    db.cleanup()


def test_gtid_mode_enabled(db, release):
    db = db(release)
    db.connect_to_pod(0)
    out = db.query("SHOW GLOBAL VARIABLES LIKE 'GTID_MODE'")
    assert out[0][1] == 'ON'
    db.cleanup()
