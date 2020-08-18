#!/bin/sh
set -euo pipefail

basedir=/root/nodedata
etherbase=`cat ${basedir}/accounts/etherbase`
addresses=`cat ${basedir}/accounts/addresses`
privatekeys=${basedir}/accounts/privatekeys

rm -rf ${basedir}/data
mkdir -p ${basedir}/data/geth && cp $basedir/nodekey $basedir/data/geth/

echo "===========init node"
gev --datadir ${basedir}/data init ${basedir}/one_node_genesis.json

echo "===========import account"
while IFS= read -r privatekey
do
  keypath=${basedir}/accounts/privatekey
  echo ${privatekey} > ${keypath}
  gev account import --datadir ${basedir}/data ${keypath} --password ${basedir}/accounts/password
done < ${privatekeys}

echo '===========starting node'
gev --datadir ${basedir}/data --nodiscover --tendermint.blockperiod 1 --syncmode full --mine --networkid 15 \
    --rpc --rpcaddr 0.0.0.0 --rpcport 8545 --port 30303 \
    --etherbase ${etherbase} \
    --unlock ${addresses} --password ${basedir}/accounts/password \
    --rpcapi admin,db,eth,debug,miner,net,shh,txpool,personal,web3 --allow-insecure-unlock