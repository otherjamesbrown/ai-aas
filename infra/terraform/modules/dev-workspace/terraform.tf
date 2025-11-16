terraform {
  required_providers {
    linode = {
      source  = "linode/linode"
      version = "~> 2.12"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.5"
    }
  }
}

