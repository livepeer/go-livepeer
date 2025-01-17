/*
Package `worker` hosts the main AI worker logic for managing or using runner
containers for processing inference requests on the Livepeer AI subnet. The
package allows interacting with the [AI runner containers], and it includes:
	- **Golang API Bindings** (./runner.gen.go): Generated from the AI runner's
		OpenAPI spec. To re-generate them run: `make ai_worker_codegen`
	- **Worker** (./worker.go): Listens for inference requests from the Livepeer
		AI subnet and routes them to the AI runner.
	- **Docker Manager** (./docker.go): Manages AI runner containers.

[AI runner containers]: https://github.com/livepeer/ai-runner
*/
package worker
