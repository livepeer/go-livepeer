# HTTP endpoint

The Livepeer node exposes a HTTP interface for monitoring and managing the node. This is how the `livepeer_cli` tool interfaces with a running node.
By default, the CLI listens to localhost:7935. This can be adjusted with the -cliAddr `<interface>:<port>` flag.

## Available endpoints:



`/getLogLevel` returns current verbosity level in the body of response

`/setLogLevel` sets verbosity current level. Level to set should be provided in body of the request, encoded as `application/x-www-form-urlencoded`. Parameter should be named `loglevel`.
It can be used from command like this:

`curl -F loglevel=6 http://localhost:7935/setLogLevel`

Log level should be integer from 0 to 6, where 6 means most verbose logging.
