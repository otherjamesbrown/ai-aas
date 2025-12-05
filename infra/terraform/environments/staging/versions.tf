# Staging environment Terraform configuration
# Uses local backend for state (override shared S3 backend)

terraform {
  required_version = ">= 1.6.0"

  # Local backend for staging - state stored in this directory
  backend "local" {
    path = "terraform.tfstate"
  }

  required_providers {
    linode = {
      source  = "linode/linode"
      version = "~> 2.12"
    }

    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 2.24"
    }

    helm = {
      source  = "hashicorp/helm"
      version = "~> 2.11"
    }

    null = {
      source  = "hashicorp/null"
      version = "~> 3.2"
    }
  }
}
