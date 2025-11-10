package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"time"
)

func main() {
	var (
		kubeconfig = flag.String("kubeconfig", os.Getenv("KUBECONFIG"), "Path to kubeconfig file")
		context    = flag.String("context", "", "Kubernetes context name")
		endpoint   = flag.String("endpoint", "/readyz", "Control plane endpoint to query")
		timeout    = flag.Duration("timeout", 10*time.Second, "Command timeout")
	)
	flag.Parse()

	if *kubeconfig == "" {
		fmt.Fprintln(os.Stderr, "kubeconfig must be provided via --kubeconfig or KUBECONFIG")
		os.Exit(2)
	}

	args := []string{"--kubeconfig", *kubeconfig}
	if *context != "" {
		args = append(args, "--context", *context)
	}
	args = append(args, "get", "--raw", *endpoint)

	cmd := exec.Command("kubectl", args...)
	cmd.Env = os.Environ()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	done := make(chan error, 1)
	go func() { done <- cmd.Run() }()

	select {
	case err := <-done:
		if err != nil {
			fmt.Fprintf(os.Stderr, "availability probe failed: %v\n", err)
			os.Exit(1)
		}
	case <-time.After(*timeout):
		_ = cmd.Process.Kill()
		fmt.Fprintf(os.Stderr, "availability probe timed out after %s\n", timeout.String())
		os.Exit(1)
	}

	fmt.Println("control plane readyz probe succeeded")
}
