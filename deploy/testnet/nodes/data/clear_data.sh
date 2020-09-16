#!/bin/sh
echo "------------Clear Data for 3 Test Nodes------------"
BASEDIR=$(dirname "$0")

for i in 1 2 3
do
  echo "--- Clear data for node $i ..."
  sudi rm -rf "$BASEDIR"/node_"$i"/data/gev/chaindata
  sudi rm -rf "$BASEDIR"/node_"$i"/data/gev/lightchaindata
  sudi rm -rf "$BASEDIR"/node_"$i"/data/gev/nodes
  sudi rm -r "$BASEDIR"/node_"$i"/data/gev/LOCK
  sudi rm -r "$BASEDIR"/node_"$i"/data/gev/transactions.rlp
  sudi rm -r "$BASEDIR"/node_"$i"/data/gev.ipc
  sudi rm -r "$BASEDIR"/node_"$i"/log/*.log
done 