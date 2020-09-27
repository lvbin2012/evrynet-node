#!/bin/sh
echo "------------Clear Data for 4 Test Nodes------------"
# Kill all apps are using port: 30301, 30302, 30303, 30304
sh ./stop_test_nodes.sh

# Init genesis block & Run test node
for i in 1 2 3 4
do
  echo "--- Clear data for node $i ..."
  rm -rf ./tests/test_nodes/node"$i"/data/gev/chaindata
  rm -rf ./tests/test_nodes/node"$i"/data/gev/lightchaindata
  rm -rf ./tests/test_nodes/node"$i"/data/gev/nodes
  rm -r ./tests/test_nodes/node"$i"/data/gev/LOCK
  rm -r ./tests/test_nodes/node"$i"/data/gev/transactions.rlp
  rm -r ./tests/test_nodes/node"$i"/data/gev.ipc
  rm -r ./node"$i".log
done 