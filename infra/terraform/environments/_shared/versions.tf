terraform {
  required_version = "~> 1.6.0"

  backend "s3" {}

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
  }
}
