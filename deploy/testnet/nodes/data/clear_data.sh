#!/bin/sh
echo "------------Clear Data for 3 Test Nodes------------"
BASEDIR=$(dirname "$0")

for i in 1 2 3
do
  echo "--- Clear data for node $i ..."
  sudi rm -rf "$BASEDIR"/node_"$i"/data/geth/chaindata
  sudi rm -rf "$BASEDIR"/node_"$i"/data/geth/lightchaindata
  sudi rm -rf "$BASEDIR"/node_"$i"/data/geth/nodes
  sudi rm -r "$BASEDIR"/node_"$i"/data/geth/LOCK
  sudi rm -r "$BASEDIR"/node_"$i"/data/geth/transactions.rlp
  sudi rm -r "$BASEDIR"/node_"$i"/data/geth.ipc
  sudi rm -r "$BASEDIR"/node_"$i"/log/*.log
done 