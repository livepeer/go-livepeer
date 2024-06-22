# Script to create multiple transcoders on the ETH devnet.

CLI_PORT=7935
MEDIA_PORT=8935
RTMP_PORT=1935
BOND=50

if [ -z "$1" ]; then
    echo "Usage: create_multiple_transcoders.bash <num_transcoders>"
    exit 1
fi

num_transcoders=$1

# Run the go devtool transcoder script multiple times with different ports
echo "Creating $num_transcoders transcoders..."
for i in $(seq 1 $num_transcoders); do
    echo "Creating transcoder $i..."
    go run cmd/devtool/devtool.go --cliport $CLI_PORT --mediaport $MEDIA_PORT --rtmpport $RTMP_PORT -bond $BOND setup transcoder 
    CLI_PORT=$((CLI_PORT+10))
    MEDIA_PORT=$((MEDIA_PORT+10))
    RTMP_PORT=$((RTMP_PORT+10))
    BOND=$((BOND**10))
done

echo "Done creating $num_transcoders transcoders"
