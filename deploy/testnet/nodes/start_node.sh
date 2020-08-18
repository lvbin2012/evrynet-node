#!/bin/bash

#Define stop process
stopProcess() {
    echo "- Container stopped, performing stop node at PID $gev_pid ..."
    kill -TERM "$gev_pid" 2>/dev/null
}

#Trap TERM INT
trap 'stopProcess' TERM INT

#Execute a command
echo "- Create genesis block for node $NODE_ID!"
./gev --datadir ./data init ./genesis.json

# shellcheck disable=SC2039
if [[ $HAS_METRIC ]]; then
  if [[ ! $METRICS_ENDPOINT ]]; then
    METRICS_ENDPOINT=http://52.220.52.16:8086
  fi
  if [[ ! $METRICS_USER ]]; then
    # shellcheck disable=SC2209
    METRICS_USER=test
  fi
  if [[ ! $METRICS_PASS ]]; then
    # shellcheck disable=SC2209
    METRICS_PASS=test
  fi

  echo "Start node $NODE_ID with metric!"
  ./gev --datadir ./data --identity "$NODE_ID" --verbosity 4 --tendermint.blockperiod 1 --syncmode full --networkid 15 --mine \
    --rpc --rpcaddr 0.0.0.0 --rpcvhosts "*" --rpcport 8545 --port 30303 \
    --miner.gaslimit 94000000 \
    --rpcapi admin,db,eth,evr,debug,miner,net,shh,txpool,personal,web3,tendermint \
    --bootnodes "enode://$BOOTNODE_ID@$BOOTNODE_IP:30300" \
    --allow-insecure-unlock \
    --pprof --pprofaddr 0.0.0.0 --pprofport 6060 \
    --nodekeyhex "$NODEKEYHEX"\
    --metrics --metrics.influxdb --metrics.influxdb.endpoint "$METRICS_ENDPOINT" --metrics.influxdb.username $METRICS_USER --metrics.influxdb.password $METRICS_PASS 2>>./log/node.log &

  gev_pid=$!
  echo "- PID of node NODE_ID is $gev_pid"
else
  echo "Start node $NODE_ID!"
  ./gev --datadir ./data --identity "$NODE_ID" --verbosity 4 --tendermint.blockperiod 1 --syncmode full --networkid 15 --mine \
    --rpc --rpcaddr 0.0.0.0 --rpccorsdomain http://"$RPC_CORSDOMAIN":8080 --rpcvhosts "*" --rpcport 8545 --port 30303 \
    --miner.gaslimit 94000000 \
    --bootnodes "enode://$BOOTNODE_ID@$BOOTNODE_IP:30300" \
    --allow-insecure-unlock \
    --pprof --pprofaddr 0.0.0.0 --pprofport 6060 \
    --nodekeyhex "$NODEKEYHEX"\
    --rpcapi admin,db,eth,evr,debug,miner,net,shh,txpool,personal,web3,tendermint 2>>./log/node.log &

  gev_pid=$!
  echo "- PID of node NODE_ID is $gev_pid"
fi

#Wait
wait $gev_pid
trap - TERM INT
wait $gev_pid