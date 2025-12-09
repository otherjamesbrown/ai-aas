# Use a Python slim image as the base
FROM python:3.10-slim-bookworm

# Set working directory
WORKDIR /app

# Install necessary packages
# We use a non-root user later, so install system packages first
RUN apt-get update && apt-get install -y --no-install-recommends \
    git \
    && rm -rf /var/lib/apt/lists/*

# Install Python dependencies
COPY operators/ai-model-operator/hack/model-downloader/requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# Copy the downloader script
COPY operators/ai-model-operator/hack/model-downloader/download.py .

# Create a non-root user and switch to it
RUN adduser --system --group appuser
USER appuser

# Set the entrypoint for the container
ENTRYPOINT ["python3", "/app/download.py"]
