/*
Package `worker` hosts the AI worker logic for managing or using live runner
containers that serve live-video-to-video on the Livepeer AI subnet. It includes:

- Golang API Bindings (./runner.gen.go):

Generated from ./openapi.yaml, the live runner API spec maintained in this
repository. To re-generate them run: `make ai_worker_codegen`

- Worker (./worker.go):

Routes live-video-to-video requests to a runner container.

- Docker Manager (./docker.go):

Manages runner containers. For a state diagram showing the lifecycle of a container, see the /doc/worker.md file.

- Serverless Worker (./serverless_worker.go):

Talks to a remote scope runner over a websocket instead of managing containers.
*/
package worker
