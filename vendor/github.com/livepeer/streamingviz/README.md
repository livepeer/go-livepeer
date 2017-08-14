# Livepeer Network Visualization

Right now the Livepeer nodes report their status to a central visualization server by default. We'll remove this after the initial build, but for now, during debugging it is useful to see which nodes connect to one another and what roles they are playing for a given stream.

## Start the Visualizaiton Server

From the `streamingviz` root directory, run

    go run ./server/server.go

Start the livepeer nodes with the `--viz` flag to allow them to send their info the viz server. The `--vizhost <host>` flag will specify where the server is running (defaults to `http://localhost:8585`).

    livepeer --viz --vizhost http://gateway1-toynet.livepeer.org:8585

## Access the visualization

If the visualization server is running, you can access the visualizaiton at [http://localhost:8585?streamid=\<streamid\>] for any given stream id. Accessing it without the argument will show the entire network, but not any stream data about who is broadcasting or consuming.

Nodes will report their peer list to the server every 20 seconds. Visualization will refresh to update itself every 30 seconds.

## TODO

1. Make sure that the http requests aren't blocking
2. Add `LogDone()` events when nodes are done broadcasting, consuming, or relaying.
3. Add a dropdown of known streams to the visualization so we can inspect them all without copying and pasting the streamID.
4. Add a timeout to remove nodes that haven't checked in in awhile.

