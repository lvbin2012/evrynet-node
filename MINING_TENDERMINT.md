# How to start 1 nodes with miner in Tendermint

1. Build and export to `PATH`
    ```shell
    $ go build ./cmd/gev
    $ go build ./cmd/bootnode
    $ go build ./cmd/puppeth
    $ export PATH=$(pwd):$PATH
    ```

2. Create a working directory for 1 validator node
    ```shell
    $ mkdir validator-node
    ```  

3. In working directory create a folder containing nodekey
    ```shell    
    $ mkdir -p validator-node/gev
    ```  

4. Generate node key and copy it into folder `validator-node/gev`
    ```shell
    $ bootnode --genkey=nodekey
    $ cp nodekey validator-node/gev
    ```

5. Generate tendermint genesis.json from pupeth
     ```shell
    $ ./puppeth    
    ```

Following steps to generate tendermint genesis.json. In step ask for first validator, enter address from nodekey, which is generated above

6. Init chaindata with tendermint genesis.json
    ```shell
    $ ./gev init genesis.json --datadir validator-node
    ```

7. Now we will start node and mine
    ```shell    
    $ ./gev --datadir validator-node --rpc --rpcaddr "127.0.0.1" --rpcport "8545" --rpc --rpccorsdomain "*" console 
    $ miner.start()
    ```
    A node will generate block and seal every 1 second (default config). To config for this blockperiod, when start node, you can config flag, for example: ```--tendermint.blockperiod 10``` (10 seconds)