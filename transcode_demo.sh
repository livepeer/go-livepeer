manifest=$(curl -s 'localhost:8935/stream/current.m3u8' 2> /dev/null)
sid1=$(echo $manifest| cut -d' ' -f 4)
sid2=$(echo $manifest| cut -d' ' -f 6)
sid3=$(echo $manifest| cut -d' ' -f 8)
ffplay "http://localhost:8935/stream/$sid1" &
ffplay "http://localhost:8935/stream/$sid2" &
ffplay "http://localhost:8935/stream/$sid3" &
