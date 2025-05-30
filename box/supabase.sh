#!/bin/bash

# Download Docker scripts to run Supabase locally
DEST="./box/supabase"
if [ ! -d "$DEST" ]; then
  echo "Supabase directory: $DEST does not exist exists. Setting up..."
  cd ./box
  git clone --branch v1.24.09 --depth 1 https://github.com/supabase/supabase.git supabase-repo
  cp -r supabase-repo/docker ./supabase
  rm -rf supabase-repo
  cp docker-compose.supabase.yml supabase/docker-compose.yml
  cp supabase/.env.example supabase/.env
  cd ..
fi

cd ./box/supabase
docker compose up
