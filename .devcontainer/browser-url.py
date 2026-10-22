#!/usr/bin/env python3
"""Find the host URL via Docker labels, independent of Compose project naming."""
import argparse
import json
import os
from pathlib import Path
import subprocess
import sys


def docker(*args):
    result = subprocess.run(['docker', *args], capture_output=True, text=True)
    if result.returncode:
        raise RuntimeError(result.stderr.strip() or 'Docker command failed')
    return result.stdout


def one_container(*filters):
    args = ['ps', '--format', '{{.ID}}']
    for value in filters:
        args.extend(['--filter', value])
    ids = docker(*args).split()
    if len(ids) != 1:
        raise RuntimeError('Expected one running project container; run task devcontainer:up, or rebuild to apply the new UI relay')
    return ids[0]


def browser_url(workspace):
    workspace = os.path.abspath(workspace)
    container = one_container('label=devcontainer.local_folder=' + workspace,
                              'label=devcontainer.config_file=' + str(Path(workspace) / '.devcontainer/devcontainer.json'))
    info = json.loads(docker('inspect', container))[0]
    project = info['Config']['Labels'].get('com.docker.compose.project')
    if not project:
        raise RuntimeError('Workspace has no Compose project label')
    relay = one_container('label=com.docker.compose.project=' + project,
                          'label=com.docker.compose.service=dev-ui')
    bindings = json.loads(docker('inspect', relay))[0]['NetworkSettings']['Ports'].get('8080/tcp') or []
    for binding in bindings:
        if binding.get('HostIp') == '127.0.0.1':
            port = binding.get('HostPort', '')
            if port.isdigit() and 0 < int(port) < 65536:
                return 'http://127.0.0.1:' + port
    raise RuntimeError('UI relay has no published localhost port; run task devcontainer:rebuild')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--workspace-folder', default='.')
    args = parser.parse_args()
    try:
        print(browser_url(args.workspace_folder))
    except (OSError, RuntimeError, ValueError, KeyError, IndexError) as error:
        print('SeamGraph URL: ' + str(error), file=sys.stderr)
        return 2
    return 0


if __name__ == '__main__':
    sys.exit(main())
