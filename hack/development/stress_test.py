#!/usr/bin/python3

# Description:
# A simple python script that generates lots of queries to stress the MySQL server.
#
# Notes:
# pip install mysql-connector-python
#
# Examples:
# ./stress_test.py mysql://root:not-so-secure@127.0.0.1:33066/test --times=100

import time
import sys
import argparse
from urllib.parse import urlparse

import mysql.connector


class Base:
    def __init__(self, db, args):
        self.db = db
        self.args = args

    def __call__(self):
        self.init_sql()

        rows = self.args.rows
        total_time = time.time()
        for i in range(int(self.args.times)):
            start_time = time.time()
            self.test(i)
            print ('{}: round: {} rows: {} to {} time: {}'.format(self.__class__.__name__,
                i, i*rows, (i+1)*rows, time.time()-start_time))

    def init_sql(self):
        pass

    def test(self, epoch):
        pass


class InsertStress(Base):
    def __init__(self, *args, **kwargs):
        super().__init__(*args, **kwargs)
        self.table_name = 'test'

    def init_sql(self):
        cursor = self.db.cursor()
        cursor.execute('CREATE TABLE IF NOT EXISTS {} '
                       '(f1 VARCHAR(30), f2 VARCHAR(30), f3 VARCHAR(30))'.format(self.table_name))
        self.db.commit()

    def test(self, epoch):
        sql = "INSERT INTO {} (f1, f2, f3) VALUES (%s, %s, %s)".format(self.table_name)
        val = []
        for i in range(self.args.rows):
            val.append(("l_"+str(i), "f_"+str(i), "a_"+str(i)))


        mycursor = self.db.cursor()
        mycursor.executemany(sql, val)
        self.db.commit()


def mysql_connection(dsn):
    dsn = urlparse(dsn)
    db_name = dsn.path.strip('/')
    if len(db_name)==0:
        raise RuntimeError('Database name not corectly specifyied: {}'.format(db_name))

    try:
        db = mysql.connector.connect(
            host=dsn.hostname,
            port=dsn.port,
            user=dsn.username,
            passwd=dsn.password,
            database=db_name,
        )
    except mysql.connector.errors.ProgrammingError as e:
        if 'Unknown database' in e.msg:
            db = mysql.connector.connect(
                host=dsn.hostname,
                port=dsn.port,
                user=dsn.username,
                passwd=dsn.password,
            )
            cursor = db.cursor()
            cursor.execute('CREATE DATABASE IF NOT EXISTS {}'.format(db_name))
            cursor.execute('USE {}'.format(db_name))
            db.commit()
        else:
            raise

    return db


def parse_args():
    parser = argparse.ArgumentParser(description='Stress test for mysql.')
    parser.add_argument('DSN', type=str,
                        help='Data source name that contains user and password.')
    parser.add_argument('--rows', type=int, default=100000, help='The number of rows to execute')
    parser.add_argument('--times', type=int, default=100,
                        help='The number of times to execute the queries')

    return parser.parse_args()


def main():
    args = parse_args()
    db = mysql_connection(args.DSN)

    # run insert stress tests
    InsertStress(db, args)()


if __name__ == "__main__":
    main()
