locals {
  tags = distinct(concat(var.tags, [var.cluster_label]))
}

resource "linode_lke_cluster" "this" {
  label       = var.cluster_label
  region      = var.region
  k8s_version = var.k8s_version
  tags        = local.tags

  dynamic "pool" {
    for_each = var.node_pools
    content {
      type  = pool.value.type
      count = pool.value.count

      autoscaler {
        min = pool.value.autoscaler.min
        max = pool.value.autoscaler.max
      }

      dynamic "labels" {
        for_each = pool.value.labels
        content {
          key   = labels.key
          value = labels.value
        }
      }

      dynamic "taints" {
        for_each = pool.value.taints
        content {
          key    = taints.value.key
          value  = taints.value.value
          effect = taints.value.effect
        }
      }
    }
  }
}

data "linode_lke_cluster_kubeconfig" "this" {
  cluster_id = linode_lke_cluster.this.id
}

data "linode_lke_cluster_dashboard" "this" {
  cluster_id = linode_lke_cluster.this.id
}
