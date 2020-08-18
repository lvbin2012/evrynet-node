#!/bin/bash

# Input params
localVolumes=
until [[ $localVolumes ]]; do read -rp "- Path of Sharing Volume: " localVolumes; done
genesisPath=
until [[ $genesisPath ]]; do read -rp "- Path of Genesis File: " genesisPath; done
rpccorsdomain=
until [[ $rpccorsdomain ]]; do read -rp "- RPC Cors Domain: " rpccorsdomain; done
version=
until [[ $version ]]; do read -rp "- Tag Version/Branch Name you want to deploy: " version; done
env=
until [[ $env ]]; do read -rp "- Environment of Image: " env; done

# Replace / with -
newVersion=${version//\//-}

BASEDIR=$(dirname "$0")
BOOTNODE_REPOSITORY="kybernetwork/evrynet-bootnode"
BOOTNODE_TAG_ENV="$BOOTNODE_REPOSITORY:$newVersion-$env"

NODE_REPOSITORY="kybernetwork/evrynet-node"
NODE_TAG_ENV="$NODE_REPOSITORY:$newVersion-$env"

echo -e "\n=> The image $BOOTNODE_TAG_ENV will be used to deploy Bootnode!"
echo -e "=> The image $NODE_TAG_ENV will be used to deploy Node!\n"


# Check status of bootnode image
if [[ "$(sudo docker images -q "$BOOTNODE_TAG_ENV" 2>/dev/null)" == "" ]]; then
  pullBootnodeImage=
  until [[ $pullBootnodeImage ]]; do read -rp "** Image $BOOTNODE_TAG_ENV doesn't exist on your local! Do you want to pull this image? " pullBootnodeImage; done

  if [[ "$pullBootnodeImage" == "y" ]]; then
    yes | sudo docker pull "$BOOTNODE_TAG_ENV"
    # Check status of bootnode image
    if [[ "$(sudo docker images -q "$BOOTNODE_TAG_ENV" 2>/dev/null)" == "" ]]; then
      echo "=> Can not pull Image $BOOTNODE_TAG_ENV . Make sure this image has existed on your hub!"
      exit 1
    fi
  else
    echo "=> You must build bootnode image on you local. Read README.md to know the way to build!"
    exit 1
  fi
fi

# Check status of node image
if [[ "$(sudo docker images -q "$NODE_TAG_ENV" 2>/dev/null)" == "" ]]; then
  pullNodeImage=
  until [[ $pullNodeImage ]]; do read -rp "** Image $NODE_TAG_ENV doesn't exist on your local! Do you want to pull this image? " pullNodeImage; done

  if [[ "$pullNodeImage" == "y" ]]; then
    yes | sudo docker pull "$NODE_TAG_ENV"
    # Check status of node image
    if [[ "$(sudo docker images -q "$NODE_TAG_ENV" 2>/dev/null)" == "" ]]; then
      echo "=> Can not pull Image $NODE_TAG_ENV . Make sure this image has existed on your hub!"
      exit 1
    fi
  else
    echo "=> You must build node image on you local. Read README.md to know the way to build!"
    exit 1
  fi
fi

echo -e "\n--- Stopping nodes 1,2,3 ..."
for i in 1 2 3; do
  yes | sudo docker exec -it gev-node-"$i" /bin/sh ./stop_node.sh
  sleep 3
done

# Stop bootnode & nodes container
sudo docker stop gev-bootnode gev-node-1 gev-node-2 gev-node-3
# Remove bootnode nodes container
sudo docker rm -f gev-bootnode gev-node-1 gev-node-2 gev-node-3

# Clear network bridge
yes | sudo docker network prune

echo -e "\n--- Starting bootnode ..."
yes | sudo imageTag="$BOOTNODE_TAG_ENV" docker-compose -f "$BASEDIR"/docker-compose.yml up -d gev-bootnode

bootnodeIP=$(sudo docker inspect -f "{{ .NetworkSettings.IPAddress }}" gev-bootnode)
if [[ "$bootnodeIP" == "" ]]; then
  echo "=> Bootnode ID is empty!"
  exit 1
fi

echo -e "\n--- Starting nodes ..."
yes | sudo bootnodeIP="$bootnodeIP" genesisPath="$genesisPath" imageTag="$NODE_TAG_ENV" shareVolumes="$localVolumes" rpccorsdomain="$rpccorsdomain" \
  docker-compose -f "$BASEDIR"/docker-compose.yml up -d gev-node-1 gev-node-2 gev-node-3
