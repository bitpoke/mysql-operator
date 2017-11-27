import time
import pytest


def test_mysql_running(db):
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
    db.insertQ(
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
