#!/usr/bin/env python3

import os
import logging
import sys
from huggingface_hub import snapshot_download
import boto3
from botocore.exceptions import ClientError

# Set up logging
logging.basicConfig(level=logging.INFO, stream=sys.stdout, format='%(asctime)s - %(levelname)s - %(message)s')
logger = logging.getLogger(__name__)

def main():
    logger.info("Starting model download and upload process...")

    # 1. Get environment variables
    model_id = os.environ.get("MODEL_ID")
    model_revision = os.environ.get("MODEL_REVISION", "main") # Default to 'main'
    s3_endpoint = os.environ.get("S3_ENDPOINT")
    s3_bucket = os.environ.get("S3_BUCKET")
    aws_access_key_id = os.environ.get("AWS_ACCESS_KEY_ID")
    aws_secret_access_key = os.environ.get("AWS_SECRET_ACCESS_KEY")
    huggingface_token = os.environ.get("HUGGINGFACE_TOKEN")

    # Validate required environment variables
    if not all([model_id, s3_endpoint, s3_bucket, aws_access_key_id, aws_secret_access_key]):
        logger.error("Missing required environment variables. Ensure MODEL_ID, S3_ENDPOINT, S3_BUCKET, AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY are set.")
        sys.exit(1)

    logger.info(f"Configuration: ModelID={model_id}, Revision={model_revision}, S3_BUCKET={s3_bucket}, S3_ENDPOINT={s3_endpoint}")

    # 2. Define local download path
    local_dir = f"/tmp/model/{model_id.replace('/', '_')}/{model_revision}"
    os.makedirs(local_dir, exist_ok=True)
    logger.info(f"Local download directory: {local_dir}")

    # 3. Download model from HuggingFace
    try:
        logger.info(f"Downloading model '{model_id}' (revision: '{model_revision}') from HuggingFace...")
        snapshot_download(
            repo_id=model_id,
            revision=model_revision,
            local_dir=local_dir,
            token=huggingface_token,
            resume_download=True, # Resume if interrupted
            local_dir_use_symlinks=False, # Avoid symlinks for easier S3 upload
        )
        logger.info("Model download complete.")
    except Exception as e:
        logger.error(f"Error downloading model from HuggingFace: {e}")
        sys.exit(1)

    # 4. Upload model to S3
    s3_client = boto3.client(
        "s3",
        endpoint_url=s3_endpoint,
        aws_access_key_id=aws_access_key_id,
        aws_secret_access_key=aws_secret_access_key,
    )

    s3_prefix = f"models/{model_id}/{model_revision}/"
    logger.info(f"Uploading model artifacts to S3 bucket '{s3_bucket}' with prefix '{s3_prefix}'...")

    try:
        for root, _, files in os.walk(local_dir):
            for file_name in files:
                local_path = os.path.join(root, file_name)
                # Calculate relative path to form S3 key
                relative_path = os.path.relpath(local_path, local_dir)
                s3_key = os.path.join(s3_prefix, relative_path).replace(os.sep, '/') # Ensure forward slashes for S3

                logger.debug(f"Uploading {local_path} to s3://{s3_bucket}/{s3_key}")
                s3_client.upload_file(local_path, s3_bucket, s3_key)
        logger.info("Model upload to S3 complete.")
    except ClientError as e:
        logger.error(f"S3 client error during upload: {e}")
        sys.exit(1)
    except Exception as e:
        logger.error(f"Error during S3 upload: {e}")
        sys.exit(1)

    logger.info("Model download and upload process finished successfully.")
    sys.exit(0)

if __name__ == "__main__":
    main()
