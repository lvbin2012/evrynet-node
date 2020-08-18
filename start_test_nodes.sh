#!/bin/sh
echo "------------Create Genesis Block------------"
# Kill all apps are using port: 30301, 30302, 30303, 30304
sh ./stop_test_nodes.sh

# Init genesis block & Run test node
for i in 1 2 3 4
do
  echo "--- Create genesis block for node $i ..."
  ./gev --datadir ./tests/test_nodes/node"$i"/data init ./tests/test_nodes/genesis.json

  echo "--- Start test node $i ..."
  ./gev --datadir ./tests/test_nodes/node"$i"/data --nodiscover --tendermint.blockperiod 1 --syncmode full --networkid 15 --mine \
    --rpc --rpcaddr 0.0.0.0 --rpcport 2200"$i" --port 3030"$i" \
    --pprof --pprofport 606"$i" \
    --rpcapi admin,db,eth,debug,miner,net,shh,txpool,personal,web3,tendermint --allow-insecure-unlock 2>>node"$i".log &
done