locals {
  default_tags = {
    project     = var.project
    environment = var.environment
    managed_by  = "terraform"
  }

  tags = merge(local.default_tags, var.tags)

  environment_regions = {
    development = lookup(var.region_overrides, "development", var.default_region)
    staging     = lookup(var.region_overrides, "staging", var.default_region)
    production  = lookup(var.region_overrides, "production", var.default_region)
    system      = lookup(var.region_overrides, "system", var.default_region)
  }

  environment_defaults = {
    development = {
      region      = local.environment_regions.development
      k8s_version = "1.34"
      node_pools = [
        {
          type  = "g6-standard-4"
          count = 2
          autoscaler = {
            min = 2
            max = 4
          }
          labels = {
            role = "general"
          }
          taints = []
        },
        {
          type  = "g1-gpu-rtx6000"
          count = 1
          autoscaler = {
            min = 1
            max = 1
          }
          # Note: labels and taints are not supported by the Linode Terraform provider
          # They must be applied manually after node pool creation using:
          # ./scripts/infra/apply-gpu-node-labels.sh
          labels = {}
          taints = []
        }
      ]
      ingress_whitelist = ["0.0.0.0/0"]
      data_endpoints = {
        postgres = {
          host     = "postgres.dev.internal"
          port     = 5432
          protocol = "tcp"
        }
        redis = {
          host     = "redis.dev.internal"
          port     = 6379
          protocol = "tcp"
        }
        rabbitmq = {
          host     = "rabbitmq.dev.internal"
          port     = 5672
          protocol = "tcp"
        }
      }
    }

    staging = {
      region      = local.environment_regions.staging
      k8s_version = "1.34"
      node_pools = [
        {
          type  = "g6-standard-6"
          count = 3
          autoscaler = {
            min = 3
            max = 6
          }
          labels = {
            role = "general"
          }
          taints = []
        },
        {
          # RTX4000 Ada Medium - 20GB VRAM, 32GB RAM, for 7B models and embeddings
          # Note: RTX6000 (g1-gpu-*) is NOT supported by LKE
          type  = "g2-gpu-rtx4000a1-m"
          count = 1
          autoscaler = {
            min = 1
            max = 2
          }
          # Note: labels and taints applied via scripts/infra/apply-gpu-node-labels.sh
          # Labels: nvidia.com/gpu.product=RTX4000-Ada, ai-aas.io/gpu-class=rtx4000-medium
          labels = {}
          taints = []
        },
        {
          # RTX4000 Ada Large - 20GB VRAM, 64GB RAM, for larger models needing more CPU memory
          type  = "g2-gpu-rtx4000a1-l"
          count = 1
          autoscaler = {
            min = 1
            max = 1
          }
          # Note: labels and taints applied via scripts/infra/apply-gpu-node-labels.sh
          # Labels: nvidia.com/gpu.product=RTX4000-Ada, ai-aas.io/gpu-class=rtx4000-large
          labels = {}
          taints = []
        }
      ]
      ingress_whitelist = ["0.0.0.0/0"]
      data_endpoints = {
        postgres = {
          host     = "postgres.stg.internal"
          port     = 5432
          protocol = "tcp"
        }
        redis = {
          host     = "redis.stg.internal"
          port     = 6379
          protocol = "tcp"
        }
        rabbitmq = {
          host     = "rabbitmq.stg.internal"
          port     = 5672
          protocol = "tcp"
        }
      }
    }

    production = {
      region      = local.environment_regions.production
      k8s_version = "1.34"
      node_pools = [
        {
          type  = "g6-standard-8"
          count = 3
          autoscaler = {
            min = 3
            max = 6
          }
          labels = {
            role = "general"
          }
          taints = []
        },
        {
          type  = "g7-highmem-16"
          count = 1
          autoscaler = {
            min = 1
            max = 2
          }
          labels = {
            role = "highmem"
          }
          taints = []
        }
      ]
      ingress_whitelist = ["0.0.0.0/0"]
      data_endpoints = {
        postgres = {
          host     = "postgres.prod.internal"
          port     = 5432
          protocol = "tcp"
        }
        redis = {
          host     = "redis.prod.internal"
          port     = 6379
          protocol = "tcp"
        }
        rabbitmq = {
          host     = "rabbitmq.prod.internal"
          port     = 5672
          protocol = "tcp"
        }
      }
    }

    system = {
      region      = local.environment_regions.system
      k8s_version = "1.34"
      node_pools = [
        {
          type  = "g6-standard-6"
          count = 3
          autoscaler = {
            min = 3
            max = 6
          }
          labels = {
            role = "ops"
          }
          taints = []
        },
        {
          type  = "g1-gpu-rtx6000"
          count = 2
          autoscaler = {
            min = 2
            max = 4
          }
          labels = {
            role = "gpu"
          }
          taints = [
            {
              key    = "workload"
              value  = "gpu"
              effect = "NoSchedule"
            }
          ]
        }
      ]
      ingress_whitelist = ["0.0.0.0/0"]
      data_endpoints = {
        postgres = {
          host     = "postgres.sys.internal"
          port     = 5432
          protocol = "tcp"
        }
        redis = {
          host     = "redis.sys.internal"
          port     = 6379
          protocol = "tcp"
        }
        rabbitmq = {
          host     = "rabbitmq.sys.internal"
          port     = 5672
          protocol = "tcp"
        }
      }
    }
  }

  env_config = lookup(local.environment_defaults, var.environment, local.environment_defaults["development"])
}
