package helpers

import "fmt"

// KubeConfigFor returns the kubeconfig path for a given environment.
// The implementation will be filled in during later phases once clusters are provisioned.
func KubeConfigFor(env string) string {
	return fmt.Sprintf("~/.kube/%s-config", env)
}
