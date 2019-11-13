#!/usr/bin/env bash

url='localhost:8935'
manifestID='current'

print_usage() {
  printf "Usage: use -u to specify the url (default localhost:8935), use -m to specify manifestID (default current.m3u8)"
}

while getopts ":u:m:h" opt; do
  case ${opt} in
    u )
      url=$OPTARG
      echo "url: $url"
      ;;
    m )
      manifestID=$OPTARG
      echo "manifestID: $manifestID"
      ;;
    h )
      print_usage
      ;;
    \? )
      echo "Invalid option: $OPTARG" 1>&2
      ;;
    : )
      echo "Invalid option: $OPTARG requires an argument" 1>&2
      ;;
  esac
done

sURI="$url/stream/$manifestID.m3u8"
manifest=$(curl -s $sURI 2> /dev/null)
if [ -z "$manifest" ]
then
    echo "Empty response from $url/stream/$manifestID.m3u8. Make sure the -currentManifest flag is on, or use -h to get usage options."
    exit
fi

sid1=$(echo $manifest| cut -d' ' -f 4)
sid2=$(echo $manifest| cut -d' ' -f 6)
sid3=$(echo $manifest| cut -d' ' -f 8)
ffplay "http://$url/stream/$sid1" &
ffplay "http://$url/stream/$sid2" &
ffplay "http://$url/stream/$sid3" &
