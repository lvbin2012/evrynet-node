# How to start 2 nodes in an Tendermint
1. Build and export to `PATH`
    ```shell
    $ go build ./cmd/gev
    $ go build ./cmd/bootnode
    $ export PATH=$(pwd):$PATH
    ```
2. Create a working directory for 2 validator nodes  
    ```shell
    $ mkdir node1 node2
    ```  

3. In each node’s working directory, create a data directory called `data`, and inside `data` create the `gev` directory
    ```shell
    $ mkdir -p node1/data/gev
    $ mkdir -p node2/data/gev
    ```

4. Generate node key and copy it into folder `node1`, `node2`  
    ```shell
    $ bootnode --genkey=nodekey1
    $ cp nodekey1 node1/nodekey
    
    $ bootnode --genkey=nodekey2
    $ cp nodekey2 node2/nodekey
    ```

5. Execute below command to display enode id of the new node  
    ```shell
    $ bootnode --nodekey=node1/nodekey --writeaddress > node1/enode
    $ cat node1/enode

    $ bootnode --nodekey=node2/nodekey --writeaddress > node2/enode
    $ cat node2/enode
    ```

6. In `node1` folder, we create 2 files `static-nodes.json`, `genesis.json` with the content as below:  
    `static-nodes.json`
    ```json
    [
        "enode://11111111@127.0.0.1:30300?discport=0", 
        "enode://22222222@127.0.0.1:30301?discport=0"
    ]
    ```
    - Replace `11111111` by content in node1/enode file was shown above.
    - Replace `22222222` by content in node2/enode file was shown above.
       
    `genesis.json`
    ```json
    {
        "config": {
            "chainId": 15,
            "tendermint": {
                "epoch": 3000,
                "policy": 0
            }
        },
        "nonce": "0x0000000000000001",
        "timestamp": "0x0",
        "parentHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
        "gasLimit": "0x8000000",
        "difficulty": "0x1",
        "mixhash": "0x0000000000000000000000000000000000000000000000000000000000000000",
        "coinbase": "0x3333333333333333333333333333333333333333",
        "alloc": {
            "0xb61F4c3E676cE9f4FbF7f5597A303eEeC3AE531B": {
                "balance": "0x1337000000000000000000"
            },
            "0xE8e86cB48b5Ae4143954A2e2c7314bD7628579bA": {
                "balance": "0x2337000000000000000000"
            }
        }
    }    
    ```   

7. Now we will generate initial accounts for any of the nodes in the required node’s working directory. The resulting public account address printed in the terminal should be recorded. Repeat as many times as necessary. A set of funded accounts may be required depending what you are trying to accomplish  
    ```sheel
    $ gev --datadir node1/data account new
    INFO [06-11|16:05:53.672] Maximum peer count                       ETH=25 LES=0 total=25
    Your new account is locked with a password. Please give a password. Do not forget this password.
    Passphrase: 
    Repeat passphrase: 
    Address: {b61F4c3E676cE9f4FbF7f5597A303eEeC3AE531B}

    $ gev --datadir node2/data account new
    INFO [06-11|16:06:34.529] Maximum peer count                       ETH=25 LES=0 total=25
    Your new account is locked with a password. Please give a password. Do not forget this password.
    Passphrase: 
    Repeat passphrase: 
    Address: {E8e86cB48b5Ae4143954A2e2c7314bD7628579bA}
    ```

8. To add accounts to the initial block, edit the `genesis.json` file in the lead node’s working directory and update the `alloc` field with the account(s) that were generated at previous step

9. Next we need to distribute the files created in part 4, which currently reside in the lead node’s working directory, to all other nodes. To do so, place `genesis.json` in the working directory of all nodes, place `static-nodes.json` in the data folder of each node and place X/nodekey in node (X)’s data/gev directory
    ```shell
    $ cp node1/genesis.json node2
    $ cp node1/static-nodes.json node1/data/
    $ cp node1/static-nodes.json node2/data/
    $ cp node1/nodekey node1/data/gev
    $ cp node2/nodekey node2/data/gev
    ```

10. Switch into working directory of lead node and initialize it. Repeat for every working directory X created in step 3. The resulting hash given by executing `gev init` must match for every node  
    ```shell
    $ cd node1
    $ gev --datadir data init genesis.json
    INFO [06-11|16:14:11.883] Maximum peer count                       ETH=25 LES=0 total=25
    ...
    INFO [06-11|16:14:11.898] Successfully wrote genesis state         database=lightchaindata
    $
    $ cd ../node2
    $ gev --datadir data init genesis.json
    INFO [06-11|16:14:24.814] Maximum peer count                       ETH=25 LES=0 total=25
    ...
    INFO [06-11|16:14:24.834] Successfully wrote genesis state         database=lightchaindata   
    ```

11. Start 2 nodes by this command.  
    - Node 1
    ```shell
    $ cd node1
    $ gev --datadir data --nodiscover --syncmode fast --mine --minerthreads 1 --networkid 15 --rpc --rpcaddr 0.0.0.0 --rpcport 22000 --rpcapi admin,db,eth,debug,miner,net,shh,txpool,personal,web3 --port 30300 --debug console
    ```
    - Node 2
    ```shell
    $ cd ../node2
    $ gev --datadir data --nodiscover --syncmode fast --mine --minerthreads 1 --networkid 15 --rpc --rpcaddr 0.0.0.0 --rpcport 22001 --rpcapi admin,db,eth,debug,miner,net,shh,txpool,personal,web3 --port 30301 --debug console
    ```
    
12. Or you can start all nodes by first creating a script and running it.
    ```shell
    $ nano startall.sh
    #!/bin/bash
    gev --datadir node1/data --nodiscover --syncmode fast --mine --minerthreads 1 --networkid 15 --rpc --rpcaddr 0.0.0.0 --rpcport 22000 --rpcapi admin,db,eth,debug,miner,net,shh,txpool,personal,web3 --port 30300 2>>node1/node.log &

    gev --datadir node2/data --nodiscover --syncmode fast --mine --minerthreads 1 --networkid 15 --rpc --rpcaddr 0.0.0.0 --rpcport 22001 --rpcapi admin,db,eth,debug,miner,net,shh,txpool,personal,web3 --port 30301 2>>node2/node.log &
    ```

    ```shell
    See if the any gev nodes are running.
    $ ps | grep gev
    
    Kill gev processes
    $ killall -INT gev
    
    $ chmod +x startall.sh
    $ ./startall.sh
    $ ps
    ```
