#!/bin/bash

LIVEPEER_PWD=$(pwd)

cd ../pipelines/apps/app
pnpm install

if [ ! -f .env ]; then
  echo "Creating .env file..."
  cp .env.example .env
fi

export PGPASSWORD='your-super-secret-and-long-postgres-password'

DB_PREPARED=$(psql -h 127.0.0.1 -p 5433 -U postgres -d postgres -tAc "SELECT 1 FROM information_schema.tables WHERE table_name='pipelines';")

if [ "$DB_PREPARED" != "1" ]; then
  echo "Table pipelines does not exist, setting up the database..."
  psql -h 127.0.0.1 -p 5433 -U postgres -d postgres -c "CREATE EXTENSION pg_stat_monitor;"
fi

pnpm db:push

MAIN_PIPELINE_EXISTS=$(psql -h 127.0.0.1 -p 5433 -U postgres -d postgres -tAc "SELECT 1 FROM pipelines where id = 'pip_DRQREDnSei4HQyC8';")
if [ "$MAIN_PIPELINE_EXISTS" != "1" ]; then
  echo "Main Dreamshaper pipeline does not exist, creating..."
  psql -h 127.0.0.1 -p 5433 -U postgres -d postgres -f "${LIVEPEER_PWD}/box/frontend.sql"
fi

pnpm dev
