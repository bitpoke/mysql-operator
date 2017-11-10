#!/usr/bin/env python

from setuptools import setup, find_packages


setup(
    name='Titanium',
    version='0.1',
    description='Mysql tools',
    author='Presslabs',
    author_email='flavius@presslabs.com',
    packages=find_packages(exclude=['tests', 'chart']),
    entry_points={
        'console_scripts': [
            'db = titanium.cli:main',
        ]
    }
)
