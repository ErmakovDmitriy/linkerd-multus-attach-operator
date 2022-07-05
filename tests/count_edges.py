#!/usr/bin/env python

import json
import sys

CONTROL_PLANE_NAMESPACES = [
    'linkerd-viz',
    'linkerd',
    'linkerd-cni',
]

LINKERD_PROMETHEUS = 'prometheus'

__MESHED_COUNT_KEY = 'meshed_count'
__NOT_MESHED_COUNT_KEY = 'not_meshed_count'


def count_edges(fio):
    edges = json.load(sys.stdin)

    meshed_count = 0
    not_meshed_count = 0

    for edge in edges:
        if (edge['src_namespace'] in CONTROL_PLANE_NAMESPACES or
                edge['src'].find(LINKERD_PROMETHEUS)) != -1:
            not_meshed_count += 1
        else:
            meshed_count += 1

    return {
        __MESHED_COUNT_KEY: meshed_count,
        __NOT_MESHED_COUNT_KEY: not_meshed_count,
    }


if __name__ == '__main__':
    if len(sys.argv) != 2:
        print('This script must be called with one of the three subcommands')
        print('count - prints number of meshed and not meshed edges')
        print('expect-meshed-only - prints number of '
              'meshed and not meshed edges'
              'and returns 1 if there are any not-meshed edges')
        print(
            'expect-not-meshed-only - prints number of '
            'meshed and not meshed edges'
            'and returns 1 if there are any meshed edges')

        sys.exit(2)

    command = sys.argv[1]
    fio = sys.stdin

    if command == 'count':
        count = count_edges(fio)

        print('Edges report:')
        print(count)
    elif command == 'expect-meshed-only':
        count = count_edges(fio)

        print('Edges report:')
        print(count)

        if (count[__MESHED_COUNT_KEY] == 0 and
                count[__NOT_MESHED_COUNT_KEY] == 0):
            print('Zeroes for both meshed and not meshed edges count')
            print('maybe not enough data')
            sys.exit(2)

        if count['not_meshed_count'] > 0:
            print('Fail: expect-meshed-only got not meshed edges')
            sys.exit(1)
        else:
            sys.exit(0)
    elif command == 'expect-not-meshed-only':
        count = count_edges(fio)

        print('Edges report:')
        print(count)

        if (count[__MESHED_COUNT_KEY] == 0 and
                count[__NOT_MESHED_COUNT_KEY] == 0):
            print('Zeroes for both meshed and not meshed edges count')
            print('maybe not enough data')
            sys.exit(2)

        if count['meshed_count'] > 0:
            print('Fail: expect-not-meshed-only got some not-meshed edges')
            sys.exit(1)
        else:
            sys.exit(0)
    else:
        print('Wrong command {}'.format(command))
        sys.exit(1)
