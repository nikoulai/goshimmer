#!/bin/bash

# Create a function to join an array of strings by a given character
function join { local IFS="$1"; shift; echo "$*"; }

export DOCKER_BUILDKIT=1
export COMPOSE_DOCKER_CLI_BUILD=1
echo "Build GoShimmer"
# Allow docker compose to build and cache an image
docker-compose build

# check exit code of builder
if [ $? -ne 0 ]
then
  echo "Building failed. Please fix and try again!"
  exit 1
fi

echo "Run GoShimmer solo-network"

docker-compose up

echo "Clean up docker resources"
docker-compose down -v