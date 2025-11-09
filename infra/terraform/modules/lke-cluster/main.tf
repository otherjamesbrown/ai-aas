resource "linode_lke_cluster" "this" {
  label       = var.cluster_label
  region      = var.region
  k8s_version = var.k8s_version
  tags        = var.tags

  dynamic "pool" {
    for_each = var.node_pools
    content {
      type  = pool.value.type
      count = pool.value.count

      autoscaler {
        min = pool.value.autoscaler.min
        max = pool.value.autoscaler.max
      }
    }
  }
}
