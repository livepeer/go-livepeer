FROM python:3.11-slim

WORKDIR /app

# System dependencies for av (ffmpeg)
RUN apt-get update && apt-get install -y --no-install-recommends \
    libavcodec-dev libavformat-dev libavutil-dev libswscale-dev libswresample-dev \
    pkg-config gcc g++ && \
    rm -rf /var/lib/apt/lists/*

# Install SDK from source
COPY livepeer-gateway/ /sdk/
RUN pip install --no-cache-dir /sdk/

# Install missing transitive dep + FastAPI + media deps
RUN pip install --no-cache-dir protobuf fastapi uvicorn[standard] Pillow numpy

COPY app.py .

EXPOSE 8080

CMD ["sh", "-c", "uvicorn app:app --host 0.0.0.0 --port ${PORT:-8080}"]
