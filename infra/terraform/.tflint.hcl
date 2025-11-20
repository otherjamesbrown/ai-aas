config {
  call_module_type = "all"  # Replaced deprecated "module" attribute (removed in v0.54.0)
}

# Linode plugin disabled - no Linode resources exist in this Terraform configuration
# plugin "linode" {
#   enabled = true
# }

plugin "terraform" {
  enabled = true
}

rule "terraform_required_providers" {
  enabled = true
}

rule "terraform_required_version" {
  enabled = true
}
