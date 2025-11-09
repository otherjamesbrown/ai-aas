config {
  module = true
}

plugin "linode" {
  enabled = true
}

plugin "terraform" {
  enabled = true
}

rule "terraform_required_providers" {
  enabled = true
}

rule "terraform_required_version" {
  enabled = true
}
