# AI Subnet Setup Guide

Welcome to the [Livepeer Artificial Intelligence (AI) Subnet](https://explorer.livepeer.org/treasury/110409521297538895053642752647313688591695822800862508217133236436856613165807) 🤖! This comprehensive guide will assist you in setting up your Orchestrator or Gateway (formerly called Broadcaster) to utilize the AI Subnet of the [Livepeer protocol](https://livepeer.org/) for AI job processing.

> [!ATTENTION]
> Please note that the AI Subnet is currently in **alpha** and under active development. Running it on the same machine as your main Orchestrator/Gateway node is not recommended due to potential stability issues. Proceed with caution and understand the associated risks.

## Background

<!---TODO: Give some background on the AI subnet. -->

### Supported AI Models

During the **alpha** and **beta** phases, the AI Subnet supports a limited number of AI models per inference pipeline. The currently supported models per pipeline are:

**Text-to-Image**:

-   [sd-turbo](https://huggingface.co/stabilityai/sd-turbo)
-   [sdxl-turbo](https://huggingface.co/stabilityai/sdxl-turbo)
-   [stable-diffusion-v1-5](https://huggingface.co/runwayml/stable-diffusion-v1-5)
-   [stable-diffusion-xl-base-1.0](https://huggingface.co/stabilityai/stable-diffusion-xl-base-1.0)
-   [openjourney-v4](https://huggingface.co/prompthero/openjourney-v4)
-   [ByteDance/SDXL-Lightning](https://huggingface.co/ByteDance/SDXL-Lightning)

**Image-to-Image**:

-   [sd-turbo](https://huggingface.co/stabilityai/sd-turbo)
-   [sdxl-turbo](https://huggingface.co/stabilityai/sdxl-turbo)
-   [ByteDance/SDXL-Lightning](https://huggingface.co/ByteDance/SDXL-Lightning)

**Image-to-Video**:

-   [stable-video-diffusion-img2vid-xt](https://huggingface.co/stabilityai/stable-video-diffusion-img2vid-xt)
-   [stabilityai/stable-video-diffusion-img2vid-xt-1-1](https://huggingface.co/stabilityai/stable-video-diffusion-img2vid-xt-1-1)

When the AI subnet is fully operational we plan to support any ([diffusion](https://huggingface.co/docs/diffusers/en/index)) model that can be run in a docker container.

## Prerequisites

Before starting with either the binary or Docker installation for the AI subnet, ensure your system meets these requirements:

-   [An Nvidia GPU](https://developer.nvidia.com/cuda-gpus)
-   [The Nvidia driver](https://www.nvidia.com/Download/index.aspx)
-   [Docker](https://docs.docker.com/install/)
-   [Nvidia Container Toolkit](https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/install-guide.html)
-   [Cuda 12](https://developer.nvidia.com/cuda-downloads) (only required for the binary installation)

> [!NOTE]  
> Although this guide was tested on Linux, experienced users can adapt the instructions for Windows or macOS environments.

## Off-chain Setup

For testing and development purposes, it's a good practice to first run the Orchestrator and Gateway nodes **off-chain**. This allows you to quickly test the AI Subnet and ensure that your Orchestrator and Gateway are functioning correctly before connecting them to the on-chain [Livepeer protocol](https://livepeer.org/).

### Orchestrator Setup

#### AI Models Configuration

Orchestrators on the AI subnet can select the [supported models](#supported-ai-models) they wish to advertise and process. To do this:

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
            "pipeline": "text-to-image",
            "model_id": "ByteDance/SDXL-Lightning",
            "price_per_unit": 4768371
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

3. **Create a Hugging Face Access Token**: Follow the instructions in the [Hugging Face documentation](https://huggingface.co/docs/hub/en/security-tokens) to create a Hugging Face access token and make it available under the `HG_TOKEN` environment variable. This token will download [token-gated models](https://huggingface.co/docs/transformers.js/en/guides/private) from the Hugging Face model hub. Alternatively, you can also install your Hugging Face access token on your machine using [login command](https://huggingface.co/docs/huggingface_hub/en/guides/cli#huggingface-cli-login) of the Hugging Face CLI.

    > [!IMPORTANT]
    > The `ld_checkpoints.sh` script contains the [SVD1.1](https://huggingface.co/stabilityai/stable-video-diffusion-img2vid-xt-1-1) model, which currently also requires you to agree to the model's license agreement. If you want to advertise this model on the AI Subnet, you need to go to the [model page](https://huggingface.co/stabilityai/stable-video-diffusion-img2vid-xt-1-1), log in, and accept their terms.

4. **Download AI Models**: Download the models listed in `aiModels.json` to the `~/.lpData/models` directory using the [ld_checkpoints.sh](https://github.com/livepeer/ai-worker/blob/main/runner/dl_checkpoints.sh) script from the [livepeer/ai-worker](https://github.com/livepeer/ai-worker/blob/main/runner/dl_checkpoints.sh) repository. You can run the following in your terminal to do this:

    ```bash
    curl -s https://raw.githubusercontent.com/livepeer/ai-worker/main/runner/dl_checkpoints.sh | bash -s -- --alpha
    ```

    > [!NOTE]
    > The `--alpha` flag is used to download only the models currently supported by the Livepeer.inc Gateway node on the AI Subnet. You can remove this flag if you want to download all models and advertise them for other Gateway nodes.

#### Orchestrator Binary Setup

To run the AI Subnet Orchestrator **off-chain** using the [pre-build binaries](https://discord.com/channels/423160867534929930/577736983036559360), follow these steps:

1. **Download the Latest AI Subnet Binary**: Visit the [#🪛│builds Channel](https://discord.com/channels/423160867534929930/577736983036559360) on the [Livepeer community Discord](https://discord.com/channels/423160867534929930/577736983036559360). Find the latest message mentioning `Branch: ai-video` and your platform under `Platform:`. This message includes the newest AI Subnet binaries for your system.
2. **Extract and Configure the Binary**: Once downloaded, extract the binary to a directory of your choice.
3. **Pull the Latest AI Runner docker image**: The Livepeer AI Subnet uses a containerized workflow to run the AI models. You can download the latest AI Runner container by running the following command:

    ```bash
    docker pull livepeer/ai-runner:latest
    ```

4. **Start the Orchestrator**: Run the following command to initiate your AI Subnet Orchestrator:

    ```bash
    ./livepeer \
        -orchestrator \
        -transcoder \
        -serviceAddr 0.0.0.0:8936 \
        -v 6 \
        -nvidia "all" \
        -aiWorker \
        -aiModels ~/.lpData/aiModels.json \
        -aiModelsDir ~/.lpData/models
    ```

    While most of these flags are already used in the [Mainnet transcoding network](https://github.com/livepeer/go-livepeer) (documented in the [Livepeer documentation](https://docs.livepeer.org/references/go-livepeer/cli-reference)), the `-aiWorker`, `-aiModels`, and `-aiModelsDir` flags are new. They enable the AI Subnet Orchestrator, define the location of your AI models configuration, and specify the directory where the models are stored on your machine, respectively. If the `aiModelsDir` flag is not set, the AI Subnet Orchestrator will look for the models in the `~/.lpData/<NETWORK>/models` directory.

5. **Verify Setup**: Ensure the AI Subnet Orchestrator runs on port `8936`. Open port `8936` on your machine and forward it to the internet for external access.

> [!NOTE]
> If no binaries are available for your system, you can build the [ai-video branch](https://github.com/livepeer/go-livepeer/tree/ai-video) of [go-livepeer](https://github.com/livepeer/go-livepeer) from source by following the instructions in the [Livepeer repository](https://docs.livepeer.org/orchestrators/guides/install-go-livepeer) or by reaching out to the Livepeer community on [Discord](https://discord.gg/livepeer).

#### Orchestrator Docker Setup

<!---TODO: Check HG_FACE needed and document model mount problems. -->

To run the AI Subnet Orchestrator **off-chain** using Docker, follow these steps:

1. **Pull the AI Subnet Docker Image**: Pull the latest AI subnet Docker image from the [Livepeer Docker Hub](https://hub.docker.com/r/livepeer/go-livepeer-ai) using the following command:

    ```bash
    docker pull livepeer/go-livepeer:ai-video
    ```

2. **Pull the Latest AI Runner docker image**: The Livepeer AI Subnet uses a [containerized workflow](https://www.ibm.com/topics/containerization) to run the AI models. You can download the latest [AI Runner](https://hub.docker.com/r/livepeer/ai-runner) image by running the following command:

    ```bash
    docker pull livepeer/ai-runner:latest
    ```

3. **Run the AI Subnet Docker Image**: Execute the following command to start your AI Subnet Orchestrator:

    ```bash
    docker run \
        -v ~/.lpData/:/root/.lpData/ \
        -v /var/run/docker.sock:/var/run/docker.sock \
        --network host \
        --gpus all \
        livepeer/go-livepeer:ai-video \
        -orchestrator \
        -transcoder \
        -serviceAddr 0.0.0.0:8936 \
        -v 6 \
        -nvidia "all" \
        -aiWorker \
        -aiModels /root/.lpData/aiModels.json \
        -aiModelsDir ~/.lpData/models
    ```

    As outlined in the [Orchestrator Binary Setup](#orchestrator-binary-setup), the `-aiWorker`, `-aiModels`, and `-aiModelsDir` flags are unique to the AI Subnet Orchestrator. The remaining flags are common to the [Mainnet transcoding network](https://github.com/livepeer/go-livepeer) as detailed in the [Livepeer documentation](https://docs.livepeer.org/references/go-livepeer/cli-reference). The AI-specific flags activate the AI Subnet Orchestrator, specify the location of your AI models configuration, and define the directory for model storage on your machine. If `aiModelsDir` is not set, the AI Subnet Orchestrator defaults to the `~/.lpData/<NETWORK>/models` directory for model storage.

4. **Verify Setup**: Ensure that the AI Subnet Orchestrator runs on port `8936`. Open port `8936` on your machine and forward it to the internet for external access.

### Gateway Setup

#### Gateway Binary Setup

Gateway nodes on the AI subnet can be set up using the [pre-built binaries](https://discord.com/channels/423160867534929930/577736983036559360). Follow these steps to run the AI Subnet Gateway node **off-chain**:

1. **Download and extract the latest AI Subnet Binary**: Follow steps 1 and 2 from the [Orchestrator Binary Setup](#orchestrator-binary-setup) to download and extract the latest AI Subnet binary for your system.
2. **Start the Gateway**: Execute the following command to start your AI Subnet Gateway node:

    ```bash
    ./livepeer \
        -datadir ~/.lpData2 \
        -broadcaster \
        -orchAddr <ORCH_LIST> \
        -httpAddr 0.0.0.0:8937 \
        -v 6 \
        -httpIngest
    ```

    The flags used here are also applicable to the [Mainnet transcoding network](https://github.com/livepeer/go-livepeer). For a comprehensive understanding of these flags, consult the [Livepeer documentation](https://docs.livepeer.org/references/go-livepeer/cli-reference). Specifically, the `--orchAddr` and `--httpAddr` flags are crucial for routing the Gateway node to your local Orchestrator (i.e., `0.0.0.0:8936`) and facilitating off-chain communication between the Gateway and the Orchestrator.

3. **Verify Setup**: Ensure that the AI Subnet Gateway node runs on port `8937`. Open port `8937` on your machine and forward it to the internet for external access.

#### Gateway Docker Setup

1. **Pull the AI Subnet Docker Image**: Follow step 1 from the [Orchestrator Docker Setup](#orchestrator-docker-setup) to pull the latest AI Subnet Docker image from the [Livepeer Docker Hub](https://hub.docker.com/r/livepeer/go-livepeer-ai).
2. **Run the AI Subnet Docker Image**: Execute the following command to start your AI Subnet Gateway node:

    ```bash
    docker run -v ~/.lpData2/:/root/.lpData2 -p 8937:8937 --network host livepeer/go-livepeer:ai-video -datadir ~/.lpData2 -broadcaster -orchAddr <ORCH_LIST> -httpAddr 0.0.0.0:8937 -v 6 -httpIngest
    ```

    As outlined in the [Gateway Binary Setup](#gateway-binary-setup) the flags are common to the [Mainnet transcoding network](https://github.com/livepeer/go-livepeer) and are documented in the [Livepeer documentation](https://docs.livepeer.org/references/go-livepeer/cli-reference). The `--orchAddr` and `--httpAddr` flags are essential for directing the Gateway node to your local Orchestrator and ensuring **off-chain** communication between the Gateway and the Orchestrator, respectively.

3. **Verify Setup**: Ensure that the AI Subnet Gateway node runs on port `8937`. Open port `8937` on your machine and forward it to the internet for external access.

#### AI Job Submission

To verify the correct functioning of your **off-chain** Gateway and Orchestrator nodes, submit an AI inference job for each of the supported pipelines.

**Text-to-Image Inference Job**:

To send an `text-to-image` inference job to the Gateway node and receive the result, follow these steps:

1. **Job Submission**: Submit a job using the `curl` command:

    ```bash
    curl -X POST 0.0.0.0:8937/text-to-image -d '{"prompt":"a dog","model_id":"ByteDance/SDXL-Lightning"}'
    ```

    The output should look like:

    ```bash
    {"images":[{"seed":280278971,"url":"/stream/34937c31/dc88c7c9.png"}]}
    ```

2. **Result Retrieval**: After job completion, you'll receive a JSON with a URL to download the result. Use `curl` to download:

    ```bash
    curl -O 0.0.0.0:8937/stream/34937c31/dc88c7c9.png
    ```

Congratulations! You've successfully set up your **off-chain** AI Subnet Orchestrator and Gateway nodes to process `text-to-image` inference jobs 🎉. You can repeat the process for the `image-to-video` and `image-to-image` pipelines described below to ensure the correct functioning of all the AI inference pipelines you did setup in your `aiModels.json`.

**Image-to-Image Inference Job**:

To send an `image-to-image` inference job to the Gateway node and receive the result, follow these steps:

1. **Job Submission**: Submit a job using the `curl` command:

    ```bash
    curl -X POST 0.0.0.0:8937/image-to-image -F image=@<PATH_TO_IMAGE> -F prompt="a dog" -F model_id="ByteDance/SDXL-Lightning"
    ```

    > [!NOTE]
    > Substitute `<PATH_TO_IMAGE>` with the **local path** of the image you want to use for video generation. This command employs [curl](https://curl.se/docs/manpage.html)'s `-F` flag to upload the image file to the Gateway node. Refer to the [curl documentation](https://curl.se/docs/manpage.html) for more details.

2. **Result Retrieval**: After job completion, you'll receive a JSON with a URL to download the result. Use `curl` to download:

    ```bash
    curl -O 0.0.0.0:8937/stream/dffff04c/6f247287.png
    ```

**Image-to-Video Inference Job**:

To send an `image-to-video` inference job to the Gateway node and receive the result, follow these steps:

1. **Job Submission**: Submit a job using the `curl` command:

    ```bash
    curl -X POST localhost:8936/image-to-video -F image=@<PATH_TO_IMAGE> -F model_id=stabilityai/stable-video-diffusion-img2vid-xt-1-
    ```

    > [!NOTE]
    > Substitute `<PATH_TO_IMAGE>` with the **local path** of the image you want to use for video generation. This command employs [curl](https://curl.se/docs/manpage.html)'s `-F` flag to upload the image file to the Gateway node. Refer to the [curl documentation](https://curl.se/docs/manpage.html) for more details.

2. **Result Retrieval**: After job completion, you'll receive a JSON with a URL to download the result. Use `curl` to download:

    ```bash
    curl -O 0.0.0.0:8937/stream/dffff04c/6f247287.png
    ```

% TODO: Done till here.

## On-chain Setup

#### Put Orchestrator On-chain

There are two ways to put your AI subnet orchestrator **on-chain** to receive jobs. First you can create a new ETH wallet for your AI subnet orchestrator, deposit some eth for gas fees and set the `ethOrchAddr` flag and set it to the Ethereum address of your orchestrator Alternatively, you can run a ticket redemption service on your main orchestrator to redeem tickets on behalf of your AI subnet orchestrator.

## Issues

If you encounter any issues or have questions, feel free to reach out to us in the [ai-video channel](https://discord.com/channels/423160867534929930/1187806216185974934) of the Livepeer community on [Discord](https://discord.gg/livepeer).
