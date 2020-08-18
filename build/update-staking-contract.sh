#!/usr/bin/env bash
set -euo pipefail

pushd ./consensus/staking_contracts
git submodule update --remote staking-contract
popd