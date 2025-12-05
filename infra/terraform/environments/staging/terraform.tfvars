# Staging Environment Configuration
# Region: Singapore (sg-sin-2) with GPU support

environment = "staging"

# Override region to Singapore for GPU availability
region_overrides = {
  staging = "sg-sin-2"
}

# Additional tags for staging resources
tags = {
  cost-center = "staging"
  gpu-enabled = "true"
}
