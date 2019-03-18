#!/bin/bash

json_key=$JSON_KEY
echo "storing keys.... $json_key"

for chain in rinkeby mainnet devenv offchain
do
  echo $json_key | jq -r '.' > /root/.lpData/$chain/keystore/key.json
done

sleep 1

exec /usr/bin/livepeer "$@"
