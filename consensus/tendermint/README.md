# Tendermint consensus for evrynet-node

## init
-- TOBE written

## metrics collections:

The specific metrics required for tunning Tendermint consensus is implemented and can be collected with flag 
```
--metrics --metrics.influxdb 
```

when running normal ./gev command. Note that the influxDB instance has to be preconfigured to store the metrics required.

To achieve the requied metrics, table manipulation is required as metrics are collected as raw counter over time. The available metrics are in the following measurements:
```
geth.eth/consensus/tendermint/in/packets.meter
geth.eth/consensus/tendermint/in/traffic.meter
geth.eth/consensus/tendermint/out/packets.meter
geth.eth/consensus/tendermint/out/traffic.meter
geth.eth/consensus/tendermint/proposalwait.timer
geth.eth/consensus/tendermint/rounds.meter
```

The other metrics (tx/blocks per second etc...) are available in normal Evrynet metrics



