# Evrynet developer notes

## How to run test (this is for evrynet team only)

1. Prepare your dev env: `./prepare_dev_env.sh`
2. Run your node: `./run_one_node.sh`

## How to start 4 nodes using Tendermint consensus
1. Run `chmod +x ./start_test_nodes.sh` to make sure you have permission to run this file.
2. From the root folder of project, you can start 4 nodes by `./start_test_nodes.sh`
3. To stop, you run `./stop_test_nodes.sh`
4. To make sure 4 nodes already started. You can use console to show all peers in network by this command `./gev attach http://127.0.0.1:22001`, then run `admin.peers` you will see the result:
    ```yaml
    [{
        caps: ["eth/62", "eth/63"],
        enode: "enode://a9e3035cb6933a754455cfe4111e44d6f5711b484317122b0f0d38d4dc1938319c84b839aeedec35a9c9df1a9a54a365bd3380b470210cd9f45441f25a05c919@127.0.0.1:56064",
        id: "4b69bed7d5626bde523d80bc3ca5f11792bad2aa50816726b441fa306ddeab2f",
        name: "Gev/v1.9.0-unstable/darwin-amd64/go1.12.4",
        network: {
          inbound: true,
          localAddress: "127.0.0.1:30301",
          remoteAddress: "127.0.0.1:56064",
          static: false,
          trusted: false
        },
        protocols: {
          eth: {
            difficulty: 11,
            head: "0xd308251660c147776a8772ef7e8dd550b5fd5c485583d21b6b35708e7aa2eedb",
            version: 63
          }
        }
    }, {
        caps: ["eth/62", "eth/63"],
        enode: "enode://65c13901b52771dd0a8c80d47118df32e5a5db44c93744ce47e64731c0fb68ab90635bf7b673a0e112ae3727e719caaee6923805d2b8ac767e2142dd00c2220b@127.0.0.1:56054",
        id: "5a2ab61fb351edd410e11dfb954e4bf2c68f13d97c45db0e02645d145db6911f",
        name: "Gev/v1.9.0-unstable/darwin-amd64/go1.12.4",
        network: {
          inbound: true,
          localAddress: "127.0.0.1:30301",
          remoteAddress: "127.0.0.1:56054",
          static: false,
          trusted: false
        },
        protocols: {
          eth: {
            difficulty: 11,
            head: "0x855300485c7552c5ca4bad034b0533b10b1fae49f103c7c7631ada531201262a",
            version: 63
          }
        }
    }, {
        caps: ["eth/62", "eth/63"],
        enode: "enode://c696dc88658f5c32f51a9656e047b5dfab4f8247751eebde022fadee402e0e181085bcf79e3fc1dbe356538c9ea1a903dd3321566b4238374ad7250a421d908e@127.0.0.1:56058",
        id: "b5de3d9cea0a814642e90adf45f8b547a7f16730c0c8961a21b56c31d84ddb49",
        name: "Gev/v1.9.0-unstable/darwin-amd64/go1.12.4",
        network: {
          inbound: true,
          localAddress: "127.0.0.1:30301",
          remoteAddress: "127.0.0.1:56058",
          static: false,
          trusted: false
        },
        protocols: {
          eth: {
            difficulty: 10,
            head: "0x7fed2199ac26bf6129b2074a357019384b16377ffcba0bcd36b9e123d9e97206",
            version: 63
          }
        }
    }]
    ```
