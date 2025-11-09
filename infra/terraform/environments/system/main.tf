locals {
  config     = local.env_config
  output_dir = "${path.module}/.generated"
  repo_url   = "https://github.com/otherjamesbrown/ai-aas"
  bootstrap_secrets = {
    registry-token = {
      data = {
        token = "PLACEHOLDER"
      }
      rotation_days = 30
    }
    grafana-admin = {
      data = {
        username = "admin"
        password = "CHANGE_ME"
      }
      rotation_days = 30
    }
  }
}

resource "null_resource" "prepare_output" {
  provisioner "local-exec" {
    command = "mkdir -p ${local.output_dir}"
  }
}

module "lke_cluster" {
  source        = "../../modules/lke-cluster"
  cluster_label = "ai-aas-${var.environment}"
  region        = local.config.region
  k8s_version   = local.config.k8s_version
  node_pools    = local.config.node_pools
  tags          = [for k, v in local.tags : "${k}:${v}"]
}

module "network" {
  source             = "../../modules/network"
  environment        = var.environment
  namespace          = var.environment
  output_dir         = local.output_dir
  ingress_whitelist  = local.config.ingress_whitelist
  allowed_egress_cidrs = ["0.0.0.0/0"]
  depends_on         = [null_resource.prepare_output]
}

module "secrets_bootstrap" {
  source           = "../../modules/secrets-bootstrap"
  environment      = var.environment
  output_dir       = local.output_dir
  bootstrap_secrets = local.bootstrap_secrets
  depends_on       = [null_resource.prepare_output]
}

module "observability" {
  source       = "../../modules/observability"
  environment  = var.environment
  output_dir   = local.output_dir
  retention_days = 30
  depends_on   = [null_resource.prepare_output]
}

module "data_services" {
  source      = "../../modules/data-services"
  environment = var.environment
  output_dir  = local.output_dir
  endpoints   = local.config.data_endpoints
  depends_on  = [null_resource.prepare_output]
}

module "argo_bootstrap" {
  source      = "../../modules/argo-bootstrap"
  environment = var.environment
  output_dir  = local.output_dir
  repo_url    = local.repo_url
  revision    = "main"
  depends_on  = [null_resource.prepare_output]
}
