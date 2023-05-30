if [ -d "./tran_result" ]; then
  echo "Need to remove tran_result first"
  exit -1
fi

mkdir -p tran_result
lastsum=''

for i in {1..2}; do
  echo "Transcoding for out$i"
  ffmpeg -i seg$i.ts -c:v libx264 -s 426:240 -r 30 -mpegts_copyts 1 -minrate 700k -maxrate 700k -bufsize 700k ./tran_result/out$i.ts
  sum=($(md5sum ./tran_result/out$i.ts))
  if [[ "$lastsum" != '' && "$sum" != "$lastsum" ]]; then
    printf "\n\nDifferent MD5 - $lastsum != $sum(out$i)\n"
    exit -1
  fi
  lastsum="$sum"
done

echo "All Equal!"
md5sum ./tran_result/out*
