# AI Subnet Setup Guide

Welcome to the [Livepeer Artificial Intelligence (AI) Subnet](https://explorer.livepeer.org/treasury/110409521297538895053642752647313688591695822800862508217133236436856613165807) 🤖! This comprehensive guide will assist you in setting up your Orchestrator or Gateway (formerly called Broadcaster) to utilize the AI Subnet of the [Livepeer protocol](https://livepeer.org/) for AI job processing.

> [!CAUTION]
> Please note that the _AI Subnet_ is currently in **alpha** and under active development. Running it on the same machine as your main Orchestrator/Gateway node is not recommended due to potential stability issues. Proceed with caution and understand the associated risks.

## Background

<!---TODO: Give some background on the *AI Subnet*. -->

## Terminology

Before proceeding, ensure you understand these key terms:

-   **AI Subnet**: The [client software](https://github.com/livepeer/go-livepeer/tree/ai-video) implementing the AI functionality of the Livepeer protocol that can run AI Orchestration and Gateway nodes both **off-chain** and **on-chain**.
-   **Mainnet AI Subnet**: The collective of Orchestrators and Gateways processing AI jobs on the Livepeer mainnet.
-   **Mainnet Transcoding Network**: The primary Livepeer network handling video transcoding jobs.

### Supported AI Models

During the **alpha** and **beta** phases, the _AI Subnet_ supports a limited number of AI models per inference pipeline. The currently supported models per pipeline are:

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

When the _Mainnet AI Subnet_ is fully operational we plan to support any ([diffusion](https://huggingface.co/docs/diffusers/en/index)) model that can be run in a docker container.

## Prerequisites

Before starting with either the binary or Docker installation for the _Mainnet AI Subnet_, ensure your system meets these requirements:

-   [An Nvidia GPU](https://developer.nvidia.com/cuda-gpus)
-   [The Nvidia driver](https://www.nvidia.com/Download/index.aspx)
-   [Docker](https://docs.docker.com/install/)
-   [Nvidia Container Toolkit](https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/install-guide.html)
-   [Cuda 12](https://developer.nvidia.com/cuda-downloads) (only required for the binary installation)

> [!NOTE]  
> Although this guide was tested on Linux, experienced users can adapt the instructions for Windows or macOS environments.

## Off-chain Setup

For testing and development purposes, it's a good practice first to run the Orchestrator and Gateway nodes **off-chain**. This allows you to quickly test the _AI Subnet_ and ensure that your Orchestrator and Gateway nodes function correctly before connecting them to the **on-chain** [Livepeer protocol](https://livepeer.org/).

### Orchestrator Setup

#### AI Models Configuration

Orchestrators on the _AI Subnet_ can select the [supported models](#supported-ai-models) they wish to advertise and process. To do this:

1. **AI Model Configuration**: Create an `aiModels.json` file in the `~/.lpData` directory to specify the AI models to support in the _AI Subnet_. Refer to the provided example below for proper formatting:

    ```json
    [
        {
            "pipeline": "text-to-image",
            "model_id": "ByteDance/SDXL-Lightning",
            "price_per_unit": 4768371,
            "warm": true
        },
        {
            "pipeline": "image-to-image",
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

    In this configuration:

    - `pipeline`: This mandatory field specifies the type of inference you want to run. The currently supported pipelines are `text-to-image`, `image-to-video`, and `image-to-image`.
    - `model_id`: This mandatory field is the [Hugging Face model ID](https://huggingface.co/docs/transformers/en/main_classes/model) of the model you want to use.
    - `price_per_unit`: This mandatory field is the price in [Wei](https://ethdocs.org/en/latest/ether.html) per unit of work.
    - `warm`: This optional field specifies if the model should be kept warm on the GPU. Keeping a model warm on the GPU reduces the time it takes to run the model as it is already loaded on the GPU. We only support one model per GPU in our current **alpha** phase. Therefore, if you have one GPU and one model warm, you cannot serve any other models.

2. **Install Hugging Face CLI**: Install the Hugging Face CLI by running the following command:

    ```bash
    pip install huggingface_hub[cli,hf_transfer]
    ```

3. **Create a Hugging Face Access Token**: Follow the instructions in the [Hugging Face documentation](https://huggingface.co/docs/hub/en/security-tokens) to create a Hugging Face access token and make it available under the `HG_TOKEN` environment variable. This token will download [token-gated models](https://huggingface.co/docs/transformers.js/en/guides/private) from the Hugging Face model hub. Alternatively, you can also install your Hugging Face access token on your machine using [login command](https://huggingface.co/docs/huggingface_hub/en/guides/cli#huggingface-cli-login) of the Hugging Face CLI.

    > [!IMPORTANT]
    > The `ld_checkpoints.sh` script contains the [SVD1.1](https://huggingface.co/stabilityai/stable-video-diffusion-img2vid-xt-1-1) model, which currently also requires you to agree to the model's license agreement. If you want to advertise this model on the _AI Subnet_, you need to go to the [model page](https://huggingface.co/stabilityai/stable-video-diffusion-img2vid-xt-1-1), log in, and accept their terms.

4. **Download AI Models**: Download the models listed in `aiModels.json` to the `~/.lpData/models` directory using the [ld_checkpoints.sh](https://github.com/livepeer/ai-worker/blob/main/runner/dl_checkpoints.sh) script from the [livepeer/ai-worker](https://github.com/livepeer/ai-worker/blob/main/runner/dl_checkpoints.sh) repository. To do this you can run the following in your terminal from the `.lpData` directory:

    ```bash
    curl -s https://raw.githubusercontent.com/livepeer/ai-worker/main/runner/dl_checkpoints.sh | bash -s -- --alpha
    ```

    > [!NOTE]
    > The `--alpha` flag is used to download only the models currently supported by the Livepeer.inc Gateway node on the _AI Subnet_. You can remove this flag if you want to download all models and advertise them for other Gateway nodes.

#### Orchestrator Binary Setup

To run the _AI Subnet_ Orchestrator **off-chain** using the [pre-build binaries](https://discord.com/channels/423160867534929930/577736983036559360), follow these steps:

1. **Download the Latest AI Subnet Binary**: Visit the [#🪛│builds Channel](https://discord.com/channels/423160867534929930/577736983036559360) on the [Livepeer community Discord](https://discord.com/channels/423160867534929930/577736983036559360). Find the latest message mentioning `Branch: ai-video` and your platform under `Platform:`. This message includes the newest _AI Subnet_ binaries for your system.
2. **Extract and Configure the Binary**: Once downloaded, extract the binary to a directory of your choice.
3. **Pull the Latest AI Runner docker image**: The Livepeer _AI Subnet_ uses a containerized workflow to run the AI models. You can download the latest AI Runner container by running the following command:

    ```bash
    docker pull livepeer/ai-runner:latest
    ```

4. **Start the Orchestrator**: Run the following command to initiate your _AI Subnet_ Orchestrator:

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

    While most of these flags are already used in the [Mainnet transcoding network](https://github.com/livepeer/go-livepeer) (documented in the [Livepeer documentation](https://docs.livepeer.org/references/go-livepeer/cli-reference)), the `-aiWorker`, `-aiModels`, and `-aiModelsDir` flags are new. They enable the _AI Subnet_ Orchestrator, define the location of your AI models configuration, and specify the directory where the models are stored on your machine, respectively. If the `aiModelsDir` flag is not set, the _AI Subnet_ Orchestrator will look for the models in the `~/.lpData/<NETWORK>/models` directory.

5. **Verify Setup**: Confirm that the _AI Subnet_ Orchestrator node is operating on port `8936`. To make the Gateway node accessible from the internet, unblock port `8936` on your machine and set up port forwarding on your router.

> [!NOTE]
> If no binaries are available for your system, you can build the [ai-video branch](https://github.com/livepeer/go-livepeer/tree/ai-video) of [go-livepeer](https://github.com/livepeer/go-livepeer) from source by following the instructions in the [Livepeer repository](https://docs.livepeer.org/orchestrators/guides/install-go-livepeer) or by reaching out to the Livepeer community on [Discord](https://discord.gg/livepeer).

#### Orchestrator Docker Setup

<!---TODO: Check HG_FACE needed and document model mount problems. -->

To run the _AI Subnet_ Orchestrator **off-chain** using Docker, follow these steps:

1. **Pull the AI Subnet Docker Image**: Pull the latest _AI Subnet_ Docker image from the [Livepeer Docker Hub](https://hub.docker.com/r/livepeer/go-livepeer-ai) using the following command:

    ```bash
    docker pull livepeer/go-livepeer:ai-video
    ```

2. **Pull the Latest AI Runner docker image**: The Livepeer _AI Subnet_ uses a [containerized workflow](https://www.ibm.com/topics/containerization) to run the AI models. You can download the latest [AI Runner](https://hub.docker.com/r/livepeer/ai-runner) image by running the following command:

    ```bash
    docker pull livepeer/ai-runner:latest
    ```

3. **Run the AI Subnet Docker Image**: Execute the following command to start your _AI Subnet_ Orchestrator:

    ```bash
    docker run \
        --name livepeer_ai_orchestrator \
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

    As outlined in the [Orchestrator Binary Setup](#orchestrator-binary-setup), the `-aiWorker`, `-aiModels`, and `-aiModelsDir` flags are unique to the _AI Subnet_ Orchestrator. The remaining flags are common to the [Mainnet transcoding network](https://github.com/livepeer/go-livepeer) as detailed in the [Livepeer documentation](https://docs.livepeer.org/references/go-livepeer/cli-reference). The AI-specific flags activate the _AI Subnet_ Orchestrator, specify the location of your AI models configuration file, and define the directory for model storage on your machine. If `aiModelsDir` is not set, the _AI Subnet_ Orchestrator defaults to the `~/.lpData/<NETWORK>/models` directory for model storage.

4. **Verify Setup**: Confirm that the _AI Subnet_ Orchestrator node is operating on port `8936`. To make the Gateway node accessible from the internet, unblock port `8936` on your machine and set up port forwarding on your router.

### Gateway Setup

#### Gateway Binary Setup

Gateway nodes on the _AI Subnet_ can be set up using the [pre-built binaries](https://discord.com/channels/423160867534929930/577736983036559360). Follow these steps to run the _AI Subnet_ Gateway node **off-chain**:

1. **Download and extract the latest AI Subnet Binary**: Follow steps 1 and 2 from the [Orchestrator Binary Setup](#orchestrator-binary-setup) to download and extract the latest _AI Subnet_ binary for your system.
2. **Start the Gateway**: Execute the following command to start your _AI Subnet_ Gateway node:

    ```bash
    ./livepeer \
        -datadir ~/.lpData2 \
        -broadcaster \
        -orchAddr <ORCH_LIST> \
        -httpAddr 0.0.0.0:8937 \
        -v 6 \
        -httpIngest
    ```

    The flags used here also apply to the [Mainnet transcoding network](https://github.com/livepeer/go-livepeer). To comprehensively understand these flags, consult the [Livepeer documentation](https://docs.livepeer.org/references/go-livepeer/cli-reference). Specifically, the `--orchAddr` and `--httpAddr` flags are crucial for routing the Gateway node to your local Orchestrator (i.e., `0.0.0.0:8936`) and facilitating **off-chain** communication between the Gateway and the Orchestrator.

3. **Verify Setup**: Confirm that the _AI Subnet_ Gateway node is operating on port `8937`. To make the Gateway node accessible from the internet, unblock port `8937` on your machine and set up port forwarding on your router.

#### Gateway Docker Setup

1. **Pull the AI Subnet Docker Image**: Follow step 1 from the [Orchestrator Docker Setup](#orchestrator-docker-setup) to pull the latest _AI Subnet_ Docker image from the [Livepeer Docker Hub](https://hub.docker.com/r/livepeer/go-livepeer-ai).
2. **Run the AI Subnet Docker Image**: Execute the following command to start your _AI Subnet_ Gateway node:

    ```bash
    docker run \
        --name livepeer_ai_gateway \
        -v ~/.lpData2/:/root/.lpData2 \
        -p 8937:8937 \
        --network host \
        livepeer/go-livepeer:ai-video \
        -datadir ~/.lpData2 \
        -broadcaster \
        -orchAddr <ORCH_LIST> \
        -httpAddr 0.0.0.0:8937 \
        -v 6 \
        -httpIngest
    ```

    As outlined in the [Gateway Binary Setup](#gateway-binary-setup), the flags are common to the [Mainnet transcoding network](https://github.com/livepeer/go-livepeer) and are documented in the [Livepeer documentation](https://docs.livepeer.org/references/go-livepeer/cli-reference). The `--orchAddr` and `--httpAddr` flags are essential for directing the Gateway node to your local Orchestrator and ensuring **off-chain** communication between the Gateway and the Orchestrator, respectively.

3. **Verify Setup**: Confirm that the _AI Subnet_ Gateway node is operating on port `8937`. To make the Gateway node accessible from the internet, unblock port `8937` on your machine and set up port forwarding on your router.

#### AI Job Submission

> [!IMPORTANT]
> If you're using the `warm` flag in your `aiModels.json` ensure you have the right pipeline running on your Orchestrator before submitting a job.

Submit an AI inference job for each of the supported pipelines to verify the correct functioning of your **off-chain** Gateway and Orchestrator nodes.

#### Text-to-Image Inference Job

To send a `text-to-image` inference job to the Gateway node and receive the result, follow these steps:

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

Congratulations! You've successfully set up your **off-chain** _AI Subnet_ Orchestrator and Gateway nodes to process `text-to-image` inference jobs. 🎉 You can repeat the process for the `image-to-video` and `image-to-image` pipelines described below to ensure the correct functioning of all the AI inference pipelines you did set up in your `aiModels.json`.

#### Image-to-Image Inference Job

<!-- TODO: Image-to-image steps not working.-->

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

#### Image-to-Video Inference Job

<!--TODO: Image-to-video steps not working.-->

To send an `image-to-video` inference job to the Gateway node and receive the result, follow these steps:

1. **Job Submission**: Submit a job using the `curl` command:

    ```bash
    curl -X POST localhost:8937/image-to-video -F image=@<PATH_TO_IMAGE> -F model_id=stabilityai/stable-video-diffusion-img2vid-xt-1-1
    ```

    > [!NOTE]
    > Substitute `<PATH_TO_IMAGE>` with the **local path** of the image you want to use for video generation. This command employs [curl](https://curl.se/docs/manpage.html)'s `-F` flag to upload the image file to the Gateway node. Refer to the [curl documentation](https://curl.se/docs/manpage.html) for more details.

    The output should look like:

    ```bash
    {"images":[{"seed":3865866304,"url":"/stream/8794a01b/1f9bc7f2.mp4"}]}
    ```

2. **Result Retrieval**: After job completion, you'll receive a JSON with a URL to download the result. Use `curl` to download:

    ```bash
    curl -O localhost:8936/stream/8794a01b/1f9bc7f2.mp4
    ```

## On-chain Configuration

After successful **off-chain** testing of your Orchestrator and Gateway nodes, you can proceed with the **on-chain** configuration. This involves connecting your nodes to the [Livepeer protocol](https://livepeer.org/) for AI job processing on the _AI Subnet_. The instructions below will guide you through the steps to configure your Orchestrator and Gateway nodes **on-chain**, without affecting your main nodes.

### On-chain AI Subnet Orchestrator Configuration

To redeem _Mainnet AI Subnet_ tickets **on-chain**, ensure your _Mainnet Transcoding Network_ Orchestrator is set up and ranked in the top 100. Refer to the [Livepeer Orchestrator Setup Documentation](https://docs.livepeer.org/orchestrators/guides/mainnet-transcoding-network) for setup steps. Once set up, configure your _Mainnet AI Subnet_ Orchestrator **on-chain**. Run a separate Orchestrator for the _AI Subnet_ to avoid affecting your main Orchestrator. This Orchestrator will handle AI jobs on the _Mainnet AI Subnet_. If your main Orchestrator is operational, there are two methods for **on-chain** AI ticket redemption:

-   **Method 1 (Recommended)**: Redeem the AI tickets **on-chain** on your _Mainnet AI Subnet_ Orchestrator, using the `-ethOrchAddr` to set your _Mainnet Transcoding Network_ Orchestrator as the tickets `recipient.
-   **Method 2**: Set up a ticket redemption service using the `-redeemer` flag, and have your _Mainnet AI Subnet_ Orchestrator send the tickets to this service using the `redeemerAddr` flag.

Detailed instructions for both methods are provided below.

#### Set Ticket Recipient

The first and **recommended method** is to use the `ethOrchAddr` flag to set the Ethereum address of your _Mainnet Transcoding Network_ Orchestrator as the recipient of the AI tickets. This ensures that the AI tickets are redeemed **on-chain** by your _AI Subnet_ Orchestrator, while the AI rewards are sent to your _Mainnet Transcoding Network_ Orchestrator. Follow these steps to set up your _Mainnet AI Subnet_ Orchestrator using this method:

1. Create a new Ethereum account for your _Mainnet AI Subnet_ Orchestrator. It is recommended that you use a separate account from your _Mainnet Transcoding Network_ Orchestrator for security reasons.
2. Fund the Ethereum account with enough ETH to cover the gas costs of redeeming the AI tickets **on-chain**.
3. Open port `8936` on your machine and set up port forwarding on your router to make the _Mainnet AI Subnet_ Orchestrator accessible from the internet.

After you have completed these steps, you can start your _Mainnet AI Subnet_ Orchestrator using the binary or Docker image.

#### Binary Startup Command

To start your _Mainnet AI Subnet_ Orchestrator using the [pre-built binaries](https://discord.com/channels/423160867534929930/577736983036559360), use the following command:

```bash
./livepeer \
    -network arbitrum-one-mainnet \
    -ethUrl https://arb1.arbitrum.io/rpc \
    -orchestrator \
    -transcoder \
    -serviceAddr <PUBLIC_ORCH_IP_OR_URL>:<PUBLIC_ORCH_PORT> \
    -v 6 \
    -nvidia "all" \
    -aiWorker \
    -aiModels ~/.lpData/aiModels.json \
    -aiModelsDir ~/.lpData/models \
    -pricePerUnit 70 \
    -ethAcctAddr <AI_SUBNET_ORCH_ETH_ADDRESS> \
    -ethOrchAddr <MAIN_ORCH_ETH_ADDRESS>
```

While most flags found in this command are similar to those used when running [Mainnet transcoding Orchestrator](https://docs.livepeer.org/references/go-livepeer/cli-reference), there are two AI-specific flags to note when setting up your _Mainnet AI Subnet_ Orchestrator **on-chain**:

-   `-ethAcctAddr`: This flag specifies the Ethereum address of your _Mainnet AI Subnet_ Orchestrator.
-   `-ethOrchAddr`: This flag specifies the Ethereum address of your _Mainnet Transcoding Network_ Orchestrator.

Setting these flags ensures that your _Mainnet AI Subnet_ Orchestrator is correctly configured to redeem AI tickets **on-chain**. 🎉

#### Docker Startup Command

To start your _Mainnet AI Subnet_ Orchestrator using Docker, follow these steps:

1. **Mount the AI Subnet Orchestrator Wallet**: Ensure your AI Subnet Orchestrator wallet is available in the `~/.lpData/arbitrum-one-mainnet/keystore` directory.
2. **Mount the AI Subnet Orchestrator Password File**: Ensure your AI Subnet Orchestrator password is available in the `~/.lpData/.eth_secret` file.
3. **Run the Docker Command**: Execute the following command to start your _Mainnet AI Subnet_ Orchestrator:

    ```bash
    docker run \
        --name livepeer_ai_orchestrator \
        -v ~/.lpData/:/root/.lpData/ \
        -v /var/run/docker.sock:/var/run/docker.sock \
        --network host \
        --gpus all \
        livepeer/go-livepeer:ai-video \
        -network arbitrum-one-mainnet \
        -ethUrl https://arb1.arbitrum.io/rpc \
        -orchestrator \
        -transcoder \
        -serviceAddr <PUBLIC_ORCH_IP_OR_URL>:<PUBLIC_ORCH_PORT> \
        -v 6 \
        -nvidia "all" \
        -aiWorker \
        -aiModels /root/.lpData/aiModels.json \
        -aiModelsDir ~/.lpData/models \
        -pricePerUnit 70 \
        -ethKeystorePath /root/.lpData/arbitrum-one-mainnet/keystore \
        -ethPassword /root/.lpData/.eth_secret \
        -ethAcctAddr <AI_SUBNET_ORCH_ETH_ADDRESS> \
        -ethOrchAddr <MAIN_ORCH_ETH_ADDRESS>
    ```

While most flags found in this command are similar to those used when running [Mainnet transcoding Orchestrator](https://docs.livepeer.org/references/go-livepeer/cli-reference), there are four AI-specific flags to note when setting up your Mainnet AI Subnet Orchestrator **on-chain** using Docker:

-   `-ethAcctAddr`: This flag specifies the Ethereum address of your _Mainnet AI Subnet_ Orchestrator.
-   `-ethOrchAddr`: This flag specifies the Ethereum address of your _Mainnet Transcoding Network_ Orchestrator.

Additionaly since the _AI Subnet_ software using [Docker-out-of-Docker](http://tdongsi.github.io/blog/2017/04/23/docker-out-of-docker/) to spin up the [AI Runner](https://github.com/livepeer/ai-worker) containers, two additional docker-specific flags are crucial:

-   `--network host`: Enables communication between the Docker daemon inside the container and the [AI Runner](https://github.com/livepeer/ai-worker) containers for AI inference jobs.
-   `--aiModelsDir`: Specifies the directory on your **host machine** where AI models are stored. The Docker daemon uses this path to mount the models in the AI Runner containers.

Correctly setting these flags configures your _Mainnet AI Subnet_ Orchestrator to successfully redeem AI tickets **on-chain**. 🎉

#### Use a ticket Redemption Service

> [!NOTE]
> Coming soon.

### On-chain Gateway Setup

> [!IMPORTANT]
> During the **alpha** phase, we're focusing our development efforts on the Livepeer.inc Gateway node for **on-chain** operations. While we plan to support additional **on-chain** Gateway nodes in the future, we currently don't offer setup documentation for them.

## Provide Metrics to the AI SPE Team

### Orchestrator Metrics

> [!IMPORTANT]
> Currently, only _AI Subnet_ Orchestrator nodes running inside a Docker container can send metrics. Binary installation does not support this feature.

To help the AI SPE team monitor the _AI Subnet_ performance, you can opt to send metrics from your _AI Subnet_ Orchestrator. These metrics aid in understanding network health and identifying potential issues. Follow these steps:

1. **Install Promtail**: Install [Promtail](https://grafana.com/docs/loki/latest/send-data/promtail/installation/) on your Orchestrator node:

    ```bash
    docker pull grafana/promtail:2.9.4
    ```

2. **Configure Promtail**: Create a Promtail folder at `/etc/promtail` and add a `ai_subnet_promtail.yml` configuration file in this directory:

    ```yaml
    # Replace placeholders with your details
    clients:
        - url: https://YOUR_NAME_HERE:YOUR_PASSWORD_HERE@dca-loki.livepeer.fun/loki/api/v1/push
          external_labels:
              node_name: YOUR_SERVER_NAME_HERE
              operator: YOUR_NAME_HERE

    scrape_configs:
        - job_name: docker_logs
          docker_sd_configs:
              - host: unix:///var/run/docker.sock
                refresh_interval: 5s
          relabel_configs:
              - source_labels: ["__meta_docker_container_name"]
                regex: "/(.*)"
                target_label: "container"
              - source_labels: ["__meta_docker_container_log_stream"]
                target_label: "logstream"
              - source_labels: ["__meta_docker_container_label_logging_jobname"]
                target_label: "job"
    ```

3. **Name Your Server**: Replace `YOUR_SERVER_NAME_HERE` with a unique server name.
4. **Obtain Credentials**: Contact the AI SPE team on the [ai-video channel](https://discord.com/channels/423160867534929930/1187806216185974934) of the Livepeer community on [Discord](https://discord.gg/livepeer) for `YOUR_NAME_HERE`, `YOUR_PASSWORD`, and `YOUR_SERVER_NAME_HERE` values. Update these in the Promtail configuration file.
5. **Start Promtail**: Run the following command:

    ```bash
    docker run \
        -d \
        --name=ai_subnet_promtail \
        -v /var/run/docker.sock:/var/run/docker.sock \
        -v /etc/promtail/:/etc/promtail/ \
        --network=host \
        grafana/promtail:2.9.4 \
        -config.file=/etc/promtail/ai_subnet_promtail.yml
    ```

Thank you for contributing to the AI SPE team's efforts to optimize the _AI Subnet_! 🚀

### Gateway Metrics

> [!IMPORTANT]
> During the alpha phase, we only support the Livepeer.inc Gateway node for on-chain Gateway operations to streamline our development process. We plan to extend support to other on-chain Gateway nodes in the future. Therefore, we currently do not provide documentation for sending Gateway node metrics.

## Issues

If you encounter any issues or have questions, feel free to reach out to us in the [ai-video channel](https://discord.com/channels/423160867534929930/1187806216185974934) of the Livepeer community on [Discord](https://discord.gg/livepeer).
