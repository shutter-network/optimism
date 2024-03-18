import argparse
import http.client
import logging
import os
import shutil
import socket
import subprocess
import time

from urllib.parse import urlparse
from devnet import Bunch, run_command, read_json, write_json
from devnet.shutter import Shutter


pjoin = os.path.join

parser = argparse.ArgumentParser(description='Bedrock devnet launcher')
parser.add_argument('--monorepo-dir', help='Directory of the monorepo', default=os.getcwd())
parser.add_argument('--allocs', help='Only create the allocs and exit', type=bool, action=argparse.BooleanOptionalAction)
parser.add_argument('--test', help='Tests the deployment, must already be deployed', type=bool, action=argparse.BooleanOptionalAction)
parser.add_argument('--l1-rpc', help='RPC url for L1', type=str, default="127.0.0.1")

log = logging.getLogger()


def main():
    args = parser.parse_args()

    monorepo_dir = os.path.abspath(args.monorepo_dir)
    devnet_dir = pjoin(monorepo_dir, '.devnet')
    contracts_bedrock_dir = pjoin(monorepo_dir, 'packages', 'contracts-bedrock')
    deployment_dir = pjoin(contracts_bedrock_dir, 'deployments', 'dev-sepolia')
    op_node_dir = pjoin(args.monorepo_dir, 'op-node')
    ops_bedrock_dir = pjoin(monorepo_dir, 'ops-bedrock')
    deploy_config_dir = pjoin(contracts_bedrock_dir, 'deploy-config')
    devnet_config_path = pjoin(deploy_config_dir, 'dev-sepolia.json')
    devnet_config_template_path = pjoin(deploy_config_dir, 'dev-sepolia-template.json')
    ops_chain_ops = pjoin(monorepo_dir, 'op-chain-ops')
    sdk_dir = pjoin(monorepo_dir, 'packages', 'sdk')
    shutter_contracts_dir = pjoin(monorepo_dir, "packages", "shop-contracts")

    paths = Bunch(
      mono_repo_dir=monorepo_dir,
      devnet_dir=devnet_dir,
      contracts_bedrock_dir=contracts_bedrock_dir,
      deployment_dir=deployment_dir,
      l1_deployments_path=pjoin(deployment_dir, '.deploy'),
      deploy_config_dir=deploy_config_dir,
      devnet_config_path=devnet_config_path,
      devnet_config_template_path=devnet_config_template_path,
      op_node_dir=op_node_dir,
      ops_bedrock_dir=ops_bedrock_dir,
      ops_chain_ops=ops_chain_ops,
      sdk_dir=sdk_dir,
      genesis_l2_path=pjoin(devnet_dir, 'genesis-l2.json'),
      addresses_json_path=pjoin(devnet_dir, 'addresses.json'),
      rollup_config_path=pjoin(devnet_dir, 'rollup.json'),
      shutter_contracts_dir=shutter_contracts_dir,
    )

    os.makedirs(devnet_dir, exist_ok=True)

    git_commit = subprocess.run(['git', 'rev-parse', 'HEAD'], capture_output=True, text=True).stdout.strip()
    git_date = subprocess.run(['git', 'show', '-s', "--format=%ct"], capture_output=True, text=True).stdout.strip()

    # CI loads the images from workspace, and does not otherwise know the images are good as-is
    if os.getenv('DEVNET_NO_BUILD') == "true":
        log.info('Skipping docker images build')
    else:
        log.info(f'Building docker images for git commit {git_commit} ({git_date})')
        run_command(['docker', 'compose', 'build', '--progress', 'plain',
                     '--build-arg', f'GIT_COMMIT={git_commit}', '--build-arg', f'GIT_DATE={git_date}'],
                    cwd=paths.ops_bedrock_dir, env={
            'PWD': paths.ops_bedrock_dir,
            'DOCKER_BUILDKIT': '1',  # (should be available by default in later versions, but explicitly enable it anyway)
            'COMPOSE_DOCKER_CLI_BUILD': '1',  # use the docker cache
            # XXX: make dependent on shutter flag
            'COMPOSE_PROFILES': 'shutter'  # use the shutter compose profile to build
          })

    sht = Shutter(paths)

    if os.path.exists(paths.devnet_config_path):
        log.info("Config already exists")
    else:
        init_config(paths, args.l1_rpc)

    if os.path.exists(paths.deployment_dir):
        log.info("L1 deployment already exists")
    else:
        deploy_l1(paths, args.l1_rpc)

    if os.path.exists(paths.genesis_l2_path):
        log.info('L2 genesis and rollup configs already generated.')
    else:
        generate_l2_genesis(paths, args.l1_rpc, sht)

    up(paths, sht)
    log.info('Rollup ready')


def init_config(paths, l1_rpc):
    deploy_config = read_json(paths.devnet_config_template_path)
    latest_hash = subprocess.run([
      'cast', 'block', '-r', l1_rpc, "latest", "-f", "hash"
    ], env={
      **os.environ
    }, capture_output=True).stdout.decode("utf-8").strip('\n')

    deploy_config['l1StartingBlockTag'] = latest_hash
    deploy_config['p2pSequencerAddress'] = os.getenv("P2P_SEQUENCER_PUBLIC_KEY")
    write_json(paths.devnet_config_path, deploy_config)


def deploy_l1(paths, l1_rpc):
    log.info('Deploying contracts on L1')

    wait_for_rpc_server(l1_rpc)
    deployer_private_key = os.getenv("PRIVATE_KEY")

    fqn = 'scripts/Deploy.s.sol:Deploy'
    run_command([
      'forge', 'script', fqn, '--private-key', deployer_private_key,
      '--rpc-url', l1_rpc, '--broadcast', '--slow'
    ], env={}, cwd=paths.contracts_bedrock_dir)

    shutil.copy(paths.l1_deployments_path, paths.addresses_json_path)

    log.info('Syncing contracts.')
    run_command([
      'forge', 'script', fqn, '--sig', 'sync()',
      '--rpc-url', l1_rpc, '--slow'
    ], env={}, cwd=paths.contracts_bedrock_dir)


def generate_l2_genesis(paths, l1_rpc, sht):
    log.info('Generating L2 genesis and rollup configs.')
    cmd = [
      'go', 'run', 'cmd/main.go', 'genesis', 'l2',
      '--l1-rpc', l1_rpc,
      '--deploy-config', paths.devnet_config_path,
      '--deployment-dir', paths.deployment_dir,
      '--outfile.l2', paths.genesis_l2_path,
      '--outfile.rollup', paths.rollup_config_path
    ]
    if sht.enabled:
        cmd += [
          "--shutter-deploy-config",
          sht.devnet_config,
        ]
    run_command(cmd, cwd=paths.op_node_dir)


def up(paths, sht):
    rollup_config = read_json(paths.rollup_config_path)
    addresses = read_json(paths.addresses_json_path)

    log.info('Bringing up L2.')
    run_command(['docker', 'compose', 'up', '-d', 'l2'], cwd=paths.ops_bedrock_dir, env={
      'PWD': paths.ops_bedrock_dir
    })
    wait_up("127.0.0.1", 9545)
    wait_for_rpc_server('http://127.0.0.1:9545')

    l2_output_oracle = addresses['L2OutputOracleProxy']
    log.info(f'Using L2OutputOracle {l2_output_oracle}')
    batch_inbox_address = rollup_config['batch_inbox_address']
    log.info(f'Using batch inbox {batch_inbox_address}')

    log.info('Bringing up `op-node`, `op-proposer` and `op-batcher`.')
    run_command(['docker', 'compose', 'up', '-d', 'op-node', 'op-proposer', 'op-batcher'], cwd=paths.ops_bedrock_dir,
                env={
                  'PWD': paths.ops_bedrock_dir,
                  'L2OO_ADDRESS': l2_output_oracle,
                  'SEQUENCER_BATCH_INBOX_ADDRESS': batch_inbox_address
                })

    log.info('Bringing up `artifact-server`')
    run_command(['docker', 'compose', 'up', '-d', 'artifact-server'], cwd=paths.ops_bedrock_dir, env={
      'PWD': paths.ops_bedrock_dir
    })

    if sht.enabled:
        sht.run_all()


def wait_up(ip, port, retries=10, wait_secs=1):
    for i in range(0, retries):
        log.info(f'Trying {ip}:{port}')
        s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        try:
            s.connect((ip, int(port)))
            s.shutdown(2)
            log.info(f'Connected {ip}:{port}')
            return True
        except Exception:
            time.sleep(wait_secs)

    raise Exception(f'Timed out waiting for port {port}.')


def wait_for_rpc_server(url):
    log.info(f'Waiting for RPC server at {url}')

    headers = {'Content-type': 'application/json'}
    body = '{"id":1, "jsonrpc":"2.0", "method": "eth_chainId", "params":[]}'

    parsed = urlparse(url)

    while True:
        try:
            if parsed.scheme == "https":
                conn = http.client.HTTPSConnection(parsed.netloc)
            else:
                conn = http.client.HTTPConnection(parsed.netloc)
            conn.request('POST', parsed.path if parsed.path else '/', body, headers)
            response = conn.getresponse()
            if response.status < 300:
                log.info(f'RPC server at {url} ready')
                return
        except Exception as e:
            log.info(f'Waiting for RPC server at {url}')
            time.sleep(1)
        finally:
            if conn:
                conn.close()


if __name__ == '__main__':
    main()
