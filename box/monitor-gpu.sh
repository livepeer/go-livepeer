#!/usr/bin/env bash

# header
echo "Timestamp,CPU,VRAM,GPU Fan"

while true; do
  # grab raw CSV line: util,used,total,fan
  raw=$(nvidia-smi \
    --query-gpu=utilization.gpu,memory.used,memory.total,fan.speed \
    --format=csv,noheader,nounits --id=0)

  # split and trim
  IFS=',' read -r gpu_util mem_used mem_total fan_speed <<< "$raw"
  gpu_util=${gpu_util//[[:space:]]/}
  mem_used=${mem_used//[[:space:]]/}
  mem_total=${mem_total//[[:space:]]/}
  fan_speed=${fan_speed//[[:space:]]/}

  # VRAM % calc
  vram_pct=$(awk -v u="$mem_used" -v t="$mem_total" \
    'BEGIN{ printf("%.2f", u*100/t) }')

  # timestamp
  ts=$(date +'%Y-%m-%dT%H:%M:%S')

  # output
  echo "${ts},${gpu_util},${vram_pct},${fan_speed}"
  sleep 0.2
done
