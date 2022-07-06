#!/usr/bin/env python

'''
This script parses output from linkerd viz edges and counts number of meshed
edges.
The implementation is far from good because it makes a lot of assumptions about
how the meshed/not meshed edges will be printed in the linkerd cli output.

Edges which have their source in linkerd's control namespaces
are excluded because there are always connections between linkerd-proxy and its
control plane.
'''

import json
import sys

CONTROL_PLANE_NAMESPACES = [
    'linkerd-viz',
    'linkerd',
    'linkerd-cni',
]

LINKERD_PROMETHEUS = 'prometheus'


def count_meshed_edges(fio) -> int:
    edges = json.load(sys.stdin)

    meshed_count = 0

    for edge in edges:
        if (edge['src_namespace'] in CONTROL_PLANE_NAMESPACES or
                edge['src'].find(LINKERD_PROMETHEUS)) != -1:
            continue

        meshed_count += 1

    return meshed_count


if __name__ == '__main__':
    if len(sys.argv) != 2:
        print('This script must be called with one of the three subcommands')
        print('count - prints number of meshed and not meshed edges')
        print('expect-meshed - prints number of '
              'meshed and not meshed edges'
              'and returns 1 if there are not any meshed edges')
        print(
            'expect-not-meshed-only - prints number of '
            'meshed and not meshed edges'
            'and returns 1 if there are any meshed edges')

        sys.exit(2)

    command = sys.argv[1]
    fio = sys.stdin

    if command == 'count':
        count = count_meshed_edges(fio)

        print('Meshed edges count: {}'.format(count))
    elif command == 'expect-meshed':
        count = count_meshed_edges(fio)

        print('Meshed edges count: {}'.format(count))

        if count == 0:
            print('Fail: expect-meshed got only not-meshed edges')
            sys.exit(1)
        else:
            print('Pass: got meshed edges: {}'.format(count))
            sys.exit(0)
    elif command == 'expect-not-meshed-only':
        count = count_meshed_edges(fio)

        print('Meshed edges count: {}'.format(count))

        if count > 0:
            print('Fail: expect-not-meshed-only got some meshed edges')
            sys.exit(1)
        else:
            print('Pass: all parsed edges are not meshed')
            sys.exit(0)
    else:
        print('Wrong command {}'.format(command))
        sys.exit(1)
