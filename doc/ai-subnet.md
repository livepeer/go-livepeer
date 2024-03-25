# AI Subnet Guide

Welcome to the [Livepeer Artificial Intelligence (AI) Subnet](https://explorer.livepeer.org/treasury/110409521297538895053642752647313688591695822800862508217133236436856613165807) 🤖! This guide will walk you through setting up your Orchestrator to utilize the AI Subnet of the [Livepeer protocol](https://livepeer.org/) and perform AI jobs.

> [!WARNING]
> The AI Subnet is currently in alpha and under active development. Running it on the same machine as your main Orchestrator is not recommended due to potential stability issues. Proceed with caution and understand the associated risks.

## Prerequisites

Before you begin, ensure that you have the following prerequisites installed on your machine:

-   [A Linux-based operating system](https://www.ubuntu.com/download)
-   [Docker](https://docs.docker.com/install/)
-   [Nvidia Container Toolkit](https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/latest/install-guide.html)

> [!NOTE]  
> While this guide is tailored for Linux, experienced Docker users can adapt the instructions for Windows or macOS environments.

## Setup Instructions

Follow these step-by-step instructions to configure your Livepeer Orchestrator for the AI Subnet:

1. **Install Docker and Nvidia Container Toolkit**: Install Docker and the Nvidia Container Toolkit on your machine by following the respective installation guides.

2. **Clone the `ai-video` Branch**: Clone the `ai-video` branch from the [go-livepeer](https://github.com/livepeer/go-livepeer/tree/ai-video) repository.

3. **Build the Subnet Docker Image**: Navigate to the root of the cloned repository and build the Subnet Docker image using the following command:

    ```bash
    docker build -f ./docker/Dockerfile -t livepeer/go-livepeer-ai:latest .
    ```

4. **Configure AI Models**: Create an `aiModels.json` file in the `~/.lpData` directory to specify the AI models to support in the AI Subnet. Refer to the provided example for details on formatting.

5. **Download AI Models**: Download the models listed in `aiModels.json` to the `~/.lpData/models` directory using the [ld_checkpoints.sh](https://github.com/livepeer/ai-worker/blob/main/runner/dl_checkpoints.sh) script from the [livepeer/ai-worker](https://github.com/livepeer/ai-worker/blob/main/runner/dl_checkpoints.sh) repository.

6. **Run the Subnet Docker Image**: Execute the following command to run start your AI Subnet Orchestrator:

    ```bash
    docker run -v ~/.lpData/:/root/.lpData -v /var/run/docker.sock:/var/run/docker.sock --network host --gpus all livepeer/go-livepeer-ai:latest -orchestrator -transcoder -aiWorker -serviceAddr 0.0.0.0:8936 -v 6 -nvidia "all" -aiModels /root/.lpData/aiModels.json
    ```

7. **Verify Setup**: Ensure that the AI Subnet Orchestrator runs on port 8936. Open port 8936 on your machine and forward it to the internet for external access.

That's it! You've successfully configured your Livepeer Orchestrator to perform AI jobs on the AI Subnet. If you encounter any issues or have questions, feel free to reach out to us in the [ai-video channel](https://discord.com/channels/423160867534929930/1187806216185974934) of the Livepeer community on [Discord](https://discord.gg/livepeer).
