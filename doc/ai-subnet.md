# AI Subnet Setup Guide

Welcome to the [Livepeer Artificial Intelligence (AI) Subnet](https://explorer.livepeer.org/treasury/110409521297538895053642752647313688591695822800862508217133236436856613165807) 🤖! This comprehensive guide will assist you in setting up your Orchestrator/Broadcaster to utilize the AI Subnet of the Livepeer protocol for AI job processing.

> [!WARNING]
> Please note that the AI Subnet is currently in alpha and under active development. Running it on the same machine as your main Orchestrator/Broadcaster is not recommended due to potential stability issues. Proceed with caution and understand the associated risks.

## Prerequisites

Before starting with either the binary or Docker installation, ensure your system meets these requirements:

-   [An Nvidia GPU](https://developer.nvidia.com/cuda-gpus)
-   [The Nvidia driver](https://www.nvidia.com/Download/index.aspx)
-   [Docker](https://docs.docker.com/install/)
-   [Nvidia Container Toolkit](https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/install-guide.html)
-   [Cuda 12](https://developer.nvidia.com/cuda-downloads) - Only required for the binary installation.

> [!NOTE]  
> Although this guide was tested on Linux, experienced users can adapt the instructions for Windows or macOS environments.

### Orchestrator Setup

#### AI Models Configuration

Orchestrators on the AI subnet can chose which models to support on their machines. To do this:

1. **AI Model Configuration**: Create an `aiModels.json` file in the `~/.lpData` directory to specify the AI models to support in the AI Subnet. Refer to the provided example below for proper formatting:

    ```json
    [
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

2. **Install Hugging Face CLI**: Install the Hugging Face CLI by running the following command:

    ```bash
    pip install huggingface_hub[cli,hf_transfer]
    ```

3. **Create a Hugging Face Access Token**: Create a Hugging Face access token by fallowing the instructions in the [Hugging Face documentation](https://huggingface.co/docs/hf-cli/auth/) and make this token available under the `HG_TOKEN` environment variable. This token will be used to download token-gated models from the Hugging Face model hub.

   > [!IMPORTANT]
   > The `ld_checkpoints.sh` script contains the [SVD1.1](https://huggingface.co/stabilityai/stable-video-diffusion-img2vid-xt-1-1) model which currently also requires you to agree to the model's license agreement. If you want to advertice this models on the AI Subnet you need to go to the [model page](https://huggingface.co/stabilityai/stable-video-diffusion-img2vid-xt-1-1) login and accept their terms.

4. **Download AI Models**: Download the models listed in `aiModels.json` to the `~/.lpData/models` directory using the [ld_checkpoints.sh](https://github.com/livepeer/ai-worker/blob/main/runner/dl_checkpoints.sh) script from the [livepeer/ai-worker](https://github.com/livepeer/ai-worker/blob/main/runner/dl_checkpoints.sh) repository. You can run the following in your terminal to do this:

    ```bash
    curl -s https://raw.githubusercontent.com/livepeer/ai-worker/main/runner/dl_checkpoints.sh | bash -s --alpha
    ```

    > [!IMPORTANT]
    > The `--alpha` flag is used to only download the models that are currently supported by the Livepeer.inc gateway node on the AI Subnet. If you want to download all models and advertice them for other gateway nodes, you can remove this flag.

#### Orchestrator Binary Setup

#### Orchestrator Offchain Binary Setup

To run the AI Subnet Orchestrator off-chain using the pre-build binaries, follow these steps:

1. **Obtain the Latest AI Subnet Binary**: Navigate to the [#🪛│builds Channel](https://discord.com/channels/423160867534929930/577736983036559360) in the [Livepeer community Discord](https://discord.com/channels/423160867534929930/577736983036559360). Look for the most recent message that mentions `Branch: ai-video`. This message will contain the latest AI Subnet binaries that are compatible with your system.
2. **Extract and Configure the Binary**: Once downloaded, extract the binary to a directory of your choice.
3. **Pull the Latest AI Runner docker image**: The Livepeer AI Subnet uses a containerized workflow to run the AI models. You can download the latest AI Runner container by running the following command:

    ```bash
    docker pull livepeer/ai-runner:latest
    ```

4. **Start the Orchestrator**: Execute the following command to start your AI Subnet Orchestrator:

    ```bash
    ./livepeer -orchestrator -transcoder -aiWorker -serviceAddr 0.0.0.0:8936 -v 6 -nvidia "all" -aiModels ~/.lpData/aiModels.json
    ```

5. **Verify Setup**: Ensure that the AI Subnet Orchestrator runs on port 8936. Open port 8936 on your machine and forward it to the internet for external access.

> [!NOTE]
> If no binaries are available for your system, you can build the [ai-video branch](https://github.com/livepeer/go-livepeer/tree/ai-video) of [go-livepeer](https://github.com/livepeer/go-livepeer) from source by following the instructions in the [Livepeer repository](https://docs.livepeer.org/orchestrators/guides/install-go-livepeer) or by reaching out to the Livepeer community on [Discord](https://discord.gg/livepeer).

#### Orchestrator Docker Setup

##### Offchain Orchestrator Docker Setup

To run the AI Subnet Orchestrator using Docker, follow these steps:

1. **Pull the Subnet Docker Image**: Pull the latest Subnet Docker image from the [Livepeer Docker Hub](https://hub.docker.com/r/livepeer/go-livepeer-ai) using the following command:

    ```bash
    docker pull livepeer/go-livepeer:ai-video
    ```

2. **Pull the Latest AI Runner docker image**: The Livepeer AI Subnet uses a containerized workflow to run the AI models. You can download the latest AI Runner container by running the following command:

    ```bash
    docker pull livepeer/ai-runner:latest
    ```

3. **Run the Subnet Docker Image**: Execute the following command to start your AI Subnet Orchestrator:

    ```bash
    docker run -v ~/.lpData/:/root/.lpData -v /var/run/docker.sock:/var/run/docker.sock --network host --gpus all livepeer/go-livepeer:ai-video -orchestrator -transcoder -aiWorker -serviceAddr 0.0.0.0:8936 -v 6 -nvidia "all" -aiModels /root/.lpData/aiModels.json
    ```

4. **Verify Setup**: Ensure that the AI Subnet Orchestrator runs on port 8936. Open port 8936 on your machine and forward it to the internet for external access.

### Broadcaster Setup

#### Broadcaster Binary Setup

##### Offchain Broadcaster Binary Setup

1. **Start the Broadcaster**: Execute the following command to start your AI Subnet Broadcaster:

    ```bash
    ./livepeer -datadir ~/.lpData2 -broadcaster -orchAddr <ORCH_LIST> -httpAddr 0.0.0.0:8937 -v 6 -httpIngest
    ```

2. **Verify Setup**: Ensure that the AI Subnet Broadcaster runs on port 8937. Open port 8937 on your machine and forward it to the internet for external access.

#### Broadcaster Docker Setup

##### Offchain Broadcaster Docker Setup

1. **Start the Broadcaster**: Execute the following command to start your AI Subnet Broadcaster:

    ```bash
    docker run -v ~/.lpData2/:/root/.lpData2 -p 8937:8937 livepeer/go-livepeer:ai-video -datadir ~/.lpData2 -broadcaster -orchAddr <ORCH_LIST> -httpAddr 0.0.0.0:8937 -v 6 -httpIngest
    ```

2. **Verify Setup**: Ensure that the AI Subnet Broadcaster runs on port 8937. Open port 8937 on your machine and forward it to the internet for external access.

## Issues

If you encounter any issues or have questions, feel free to reach out to us in the [ai-video channel](https://discord.com/channels/423160867534929930/1187806216185974934) of the Livepeer community on [Discord](https://discord.gg/livepeer).
