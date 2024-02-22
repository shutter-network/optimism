import logging
import os
import time
from pathlib import Path

import devnet

log = logging.getLogger()


class Shutter:
    INITDB = "01-init-db.sh"
    INITCHAIN = "02-init-chain.sh"
    BOOTSTRAP = "03-bootstrap.sh"
    RUN = "04-run.sh"

    DEVNET_CONFIG = "deploy-config/devnetL2.json"

    def __init__(self, paths) -> None:
        self.ops_dir = Path(paths.ops_bedrock_dir)
        self.shop_contract_dir = Path(paths.shutter_contracts_dir)
        self.devnet_config = self.shop_contract_dir / self.DEVNET_CONFIG

    def enabled(self):
        return os.getenv("DEVNET_SHUTTER_ENABLED") == "true"

    def run_all(self):
        # those all raise
        #  a CalledProcessError
        # when error code is non-zero
        self.init_database()
        self.init_shuttermint()
        self.run()
        self.init_l2_contracts()
        # FIXME: this does not seem to work yet.
        # sometimes we get an OOB error
        time.sleep(10)
        self.bootstrap_shuttermint()
        self.run()

    def init_database(self):
        devnet.run_command(
            ["bash", self.INITDB],
            cwd=self.ops_dir,
            env={
                "PWD": self.ops_dir,
            },
        )

    def init_shuttermint(self):
        devnet.run_command(
            ["bash", self.INITCHAIN],
            cwd=self.ops_dir,
            env={
                "PWD": self.ops_dir,
            },
        )

    def init_l2_contracts(self):
        devnet.run_command(
            [
                "forge",
                "script",
                "-f",
                # TODO: make param or read from somewhere?
                "http://localhost:9545",
                "-s",
                "run()",
                # "--slow",
                # "--no-cache",
                "--broadcast",
                "script/Deploy.s.sol",
            ],
            cwd=self.shop_contract_dir,
            # TODO: read from the config?:
            env={
                "PWD": self.ops_dir,
                "INBOX_ADDRESS": "0x4200000000000000000000000000000000000066",
                "KEYPERSETMANAGER_ADDRESS": "0x4200000000000000000000000000000000000067",
                "KEYBROADCAST_ADDRESS": "0x4200000000000000000000000000000000000068",
                "SEQUENCER_ADDRESS": "0x8000000000000000000000000000000000000001",
                "PRIVATE_KEY": "0x83b6122c38b58e37ce42adafd43e7b402e19f4413ce6de9dc9219f50d71c3768",
                "KEYPER_ADDRESSES": "0xed6c85f92A9d8fB07b2773a14F7cD9040a1b3a57,0x1Cd9F3B8091C28e443f475FDf8bAc97C8727d537,0x933cA0DBF893aaCd2a818ec3791fEC11FDf1aeF2,0x2a0D87eA3a9E0ca33Ddd4a62C33878b58152effE",
                # should be 1 minimum
                "ACTIVATION_DELTA": "50",
                "THRESHOLD": "3",
            },
        )

    def bootstrap_shuttermint(self):
        devnet.run_command(
            ["bash", self.BOOTSTRAP],
            cwd=self.ops_dir,
            env={
                "PWD": self.ops_dir,
            },
        )

    def run(self):
        devnet.run_command(
            ["bash", self.RUN],
            cwd=self.ops_dir,
            env={
                "PWD": self.ops_dir,
            },
        )

    def up(self):
        log.info("Bringing up shutter services")
        devnet.run_command(
            [
                "docker",
                "compose",
                "up",
                "-d",
                "shutter-node",
            ],
            cwd=self.ops_dir,
            env={
                "PWD": self.ops_dir,
                "COMPOSE_PROFILES": "shutter",
            },
        )
