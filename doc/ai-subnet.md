# AI Subnet Setup Guide

Welcome to the [Livepeer Artificial Intelligence (AI) Subnet](https://explorer.livepeer.org/treasury/110409521297538895053642752647313688591695822800862508217133236436856613165807) 🤖! This comprehensive guide will assist you in setting up your Orchestrator/Broadcaster to utilize the AI Subnet of the Livepeer protocol for AI job processing.

> [!WARNING]
> Please note that the AI Subnet is currently in alpha and under active development. Running it on the same machine as your main Orchestrator/Broadcaster is not recommended due to potential stability issues. Proceed with caution and understand the associated risks.

## Binary Installation

### Prerequisites

Before getting started, ensure that your machine meets the following prerequisites:

-   [Cuda 12](https://developer.nvidia.com/cuda-downloads)

### Setup Instructions

1. **Obtain the Latest AI Subnet Binary**: Navigate to the [#🪛│builds Channel](https://discord.com/channels/423160867534929930/577736983036559360) in the [Livepeer community Discord](https://discord.com/channels/423160867534929930/577736983036559360). Look for the most recent message that mentions `Branch: ai-video`. This message will contain the latest AI Subnet binaries that are compatible with your system.
2. **Extract and Configure the Binary**: Once downloaded, extract the binary to a directory of your choice.

### Orchestrator Configuration

1. **AI Model Configuration**: Create an `aiModels.json` file in the `~/.lpData` directory to specify the AI models to support in the AI Subnet. Refer to the provided example below for proper formatting:

    ```json
    [
        {
            "pipeline": "text-to-image",
            "model_id": "stabilityai/sdxl-turbo",
            "price_per_unit": 4768371
        },
        {
            "pipeline": "text-to-image",
            "model_id": "ByteDance/SDXL-Lightning",
            "price_per_unit": 4768371,
            "warm": true
        },
        {
            "pipeline": "image-to-video",
            "model_id": "stabilityai/stable-video-diffusion-img2vid-xt-1-1",
            "price_per_unit": 3390842
        }
    ]
    ```

2. **Download AI Models**: Download the models listed in `aiModels.json` to the `~/.lpData/models` directory using the [ld_checkpoints.sh](https://github.com/livepeer/ai-worker/blob/main/runner/dl_checkpoints.sh) script from the [livepeer/ai-worker](https://github.com/livepeer/ai-worker/blob/main/runner/dl_checkpoints.sh) repository.
3. **Start the Orchestrator**: Execute the following command to start your AI Subnet Orchestrator:

    ```bash
    ./livepeer -orchestrator -transcoder -aiWorker -serviceAddr 0.0.0.0:8936 -v 6 -nvidia "all" -aiModels ~/.lpData/aiModels.json
    ```

4. **Verify Setup**: Ensure that the AI Subnet Orchestrator runs on port 8936. Open port 8936 on your machine and forward it to the internet for external access.

### Broadcaster Configuration

1. **Start the Broadcaster**: Execute the following command to start your AI Subnet Broadcaster:

    ```bash
    ./livepeer -datadir ~/.lpData2 -broadcaster -orchAddr <ORCH_LIST> -httpAddr 0.0.0.0:8937 -v 6 -httpIngest
    ```

2. **Verify Setup**: Ensure that the AI Subnet Broadcaster runs on port 8937. Open port 8937 on your machine and forward it to the internet for external access.

## Docker Installation

### Prerequisites

Before proceeding, ensure that your machine satisfies the following prerequisites:

-   [A Linux-based operating system](https://www.ubuntu.com/download)
-   [Docker](https://docs.docker.com/install/)
-   [Nvidia Container Toolkit](https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/latest/install-guide.html)

> [!NOTE]  
> Although this guide focuses on Linux, experienced Docker users can adapt the instructions for Windows or macOS environments.

### Setup Instructions

1. **Install Docker and Nvidia Container Toolkit**: Install Docker and the Nvidia Container Toolkit on your machine by following the respective installation guides.
2. **Pull the Subnet Docker Image**: Pull the latest Subnet Docker image from the [Livepeer Docker Hub](https://hub.docker.com/r/livepeer/go-livepeer-ai) using the following command:

    ```bash
    docker pull livepeer/go-livepeer:ai-video
    ```

### Orchestrator Configuration

1. **AI Model Configuration**: Create an `aiModels.json` file in the `~/.lpData` directory to specify the AI models to support in the AI Subnet. Refer to the provided example above for proper formatting.
2. **Download AI Models**: Download the models listed in `aiModels.json` to the `~/.lpData/models` directory using the [ld_checkpoints.sh](https://github.com/livepeer/ai-worker/blob/main/runner/dl_checkpoints.sh) script from the [livepeer/ai-worker](https://github.com/livepeer/ai-worker/blob/main/runner/dl_checkpoints.sh) repository.
3. **Run the Subnet Docker Image**: Execute the following command to start your AI Subnet Orchestrator:

    ```bash
    docker run -v ~/.lpData/:/root/.lpData -v /var/run/docker.sock:/var/run/docker.sock --network host --gpus all livepeer/go-livepeer:ai-video -orchestrator -transcoder -aiWorker -serviceAddr 0.0.0.0:8936 -v 6 -nvidia "all" -aiModels /root/.lpData/aiModels.json
    ```

4. **Verify Setup**: Ensure that the AI Subnet Orchestrator runs on port 8936. Open port 8936 on your machine and forward it to the internet for external access.

### Broadcaster Configuration

1. **Start the Broadcaster**: Execute the following command to start your AI Subnet Broadcaster:

    ```bash
    docker run -v ~/.lpData2/:/root/.lpData2 -p 8937:8937 livepeer/go-livepeer:ai-video -datadir ~/.lpData2 -broadcaster -orchAddr <ORCH_LIST> -httpAddr 0.0.0.0:8937 -v 6 -httpIngest
    ```

2. **Verify Setup**: Ensure that the AI Subnet Broadcaster runs on port 8937. Open port 8937 on your machine and forward it to the internet for external access.

## Issues

If you encounter any issues or have questions, feel free to reach out to us in the [ai-video channel](https://discord.com/channels/423160867534929930/1187806216185974934) of the Livepeer community on [Discord](https://discord.gg/livepeer).
