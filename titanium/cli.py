import logging
import argparse
import pkgutil
import importlib

import titanium.commands as cmds_module


def get_commands(parser):
    subparser = parser.add_subparsers(title='custome commands')
    for _, name, _ in pkgutil.iter_modules(cmds_module.__path__):
        if name.endswith('base'):
            continue
        try:
            module_name = f"{cmds_module.__name__}.{name}"
            Command = importlib.import_module(module_name).Command
        except ImportError as e:
            logging.error('Error at importing command %s, err: %s', name, e)
            raise
        parser = subparser.add_parser(name, help=Command.__doc__)
        parser.set_defaults(func=Command(parser))


def main():
    "Main CLI endtrypoint"
    parser = argparse.ArgumentParser(
            description='Mysql presslabs tools.'
        )

    get_commands(parser)
    args = parser.parse_args()
    if getattr(args, 'func', None):
        args.func(args)
    else:
        print('See help.')
