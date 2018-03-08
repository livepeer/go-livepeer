strmID=$(curl localhost:8935/streamID 2> /dev/null)
echo $strmID
curl "http://localhost:8935/transcode?strmID=$strmID"
sleep 2

manifestID=$(curl localhost:8935/manifestID 2> /dev/null)
manifest=$(curl localhost:8935/stream/$manifestID.m3u8 2> /dev/null)
sid1=$(echo $manifest| cut -d' ' -f 4)
sid2=$(echo $manifest| cut -d' ' -f 6)
sid3=$(echo $manifest| cut -d' ' -f 8)
ffplay "http://localhost:8935/stream/$sid1" &
ffplay "http://localhost:8935/stream/$sid2" &
ffplay "http://localhost:8935/stream/$sid3" &