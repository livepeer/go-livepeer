#!/bin/bash
set -ex

echo "Starting 10 test builds with 1 hour intervals..."

for i in {1..10}; do
  echo ""
  echo "==================================="
  echo "Creating build $i/10"
  echo "==================================="

  git commit --allow-empty -m "Build $i/10"
  git push

  if [ $i -lt 10 ]; then
    echo "Sleeping for 1 hour before next build..."
    sleep 3600
  fi
done

echo ""
echo "==================================="
echo "All 10 builds triggered!"
echo "Check results at: https://github.com/livepeer/go-livepeer/actions"
echo "==================================="

