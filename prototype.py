#!/usr/bin/env python
from __future__ import unicode_literals, print_function

import os
import re
import sys
import subprocess
import tempfile


def main(environ=os.environ, errout=sys.stderr, okout=sys.stdout):
    return _check_examples(
        environ.get('EXAMPLE_SOURCE_MD', 'README.md'),
        int(environ.get('EXAMPLE_SOURCE_COUNT', 4)),
        errout,
        okout
    )


def _check_examples(example_source_md, example_source_count, errout, okout):
    print('---> CHECKING {}'.format(example_source_md), file=okout)

    retcodes = []

    with open(example_source_md) as source_md:
        i = 0

        for chunk in source_md.read().split('```'):
            source = re.sub('^go', '', chunk.strip()).strip()
            if not source.startswith('package main'):
                continue

            i += 1
            exit_code = _run_example(source)
            retcodes.append(exit_code)

            if exit_code == 0:
                print('---> PASS ({})'.format(i), file=okout)
                continue

            print('!!!> FAIL ({}):\n!!! {}'.format(
                i, _numbered(source).replace('\n', '\n!!! ')), file=errout)

    if len(retcodes) != example_source_count:
        print('!!!> EXPECTED SOURCE COUNT {} != {}'.format(
            example_source_count, len(retcodes)), file=errout)
        return 86

    agg_ret = max(retcodes) if len(retcodes) > 0 else 0

    if agg_ret == 0:
        print('---> OK!', file=okout)

    return agg_ret


def _numbered(source):
    return '\n'.join([
        '{:3}: {}'.format(i + 1, line)
        for i, line in
        enumerate(source.split('\n'))
    ])


def _run_example(source, gotool='go'):
    with tempfile.NamedTemporaryFile(suffix='.go') as temp_go:
        temp_go.write(source)
        temp_go.flush()

        return subprocess.call(
            [gotool, 'run', temp_go.name],
            stdout=subprocess.PIPE
        )


if __name__ == '__main__':
    sys.exit(main())
