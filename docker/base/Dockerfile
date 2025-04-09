FROM nvidia/cuda:12.8.1-devel-ubuntu22.04

ARG MODEL_TYPE="text-generation"
ARG VERSION

ARG PYTHON_VERSION=3.12

# Set the working directory
WORKDIR /workspace
ENV DEBIAN_FRONTEND=noninteractive
ENV PATH="/root/.local/bin:$PATH"

# Install Python
RUN echo 'tzdata tzdata/Areas select America' | debconf-set-selections && \
    echo 'tzdata tzdata/Zones/America select Los_Angeles' | debconf-set-selections && \
    apt-get -qq update && \
    apt-get -qq install -y --no-install-recommends \
    software-properties-common && \
    add-apt-repository -y ppa:deadsnakes/ppa && \
    apt-get -qq update && \
    apt-get -qq install -y python${PYTHON_VERSION} python${PYTHON_VERSION}-dev python${PYTHON_VERSION}-venv && \
    update-alternatives --install /usr/bin/python3 python3 /usr/bin/python${PYTHON_VERSION} 1 && \
    update-alternatives --set python3 /usr/bin/python${PYTHON_VERSION}

# Install other dependencies
RUN apt-get -qq update && \
    apt-get -qq install -y --no-install-recommends \
    curl \
    gcc \
    libibverbs-dev && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

COPY kaito/presets/workspace/dependencies/requirements.txt /workspace/requirements.txt

RUN --mount=type=cache,target=/root/.cache/uv \
    curl -LsSf https://astral.sh/uv/install.sh | sh && \
    uv pip install --no-cache --system -r /workspace/requirements.txt

# 1. Huggingface transformers
COPY kaito/presets/workspace/inference/${MODEL_TYPE}/inference_api.py \
    kaito/presets/workspace/tuning/${MODEL_TYPE}/cli.py \
    kaito/presets/workspace/tuning/${MODEL_TYPE}/fine_tuning.py \
    kaito/presets/workspace/tuning/${MODEL_TYPE}/parser.py \
    kaito/presets/workspace/tuning/${MODEL_TYPE}/dataset.py \
    kaito/presets/workspace/tuning/${MODEL_TYPE}/metrics/metrics_server.py \
    /workspace/tfs/

# 2. vLLM
COPY kaito/presets/workspace/inference/vllm/inference_api.py /workspace/vllm/inference_api.py

# Chat template
ADD kaito/presets/workspace/inference/chat_templates /workspace/chat_templates

# Convenient script to run multi-node serving with vLLM
RUN curl -LsS https://raw.githubusercontent.com/vllm-project/vllm/refs/heads/v0.8.3/examples/online_serving/multi-node-serving.sh -o /workspace/vllm/multi-node-serving.sh && \
    chmod +x /workspace/vllm/multi-node-serving.sh

RUN echo $VERSION > /workspace/version.txt
