// Package engine provides CLI commands for inference engine management.
package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/api"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/engines"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/output"
)

// NewEngineCommand creates the engine parent command
func NewEngineCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "engine",
		Short: "Manage inference engines and configurations",
		Long: `Manage inference engines (vLLM, Triton, TGI) and their configurations.

Inference engines are the servers that run model inference. Each engine can have
multiple versions and configurations (resource profiles).

Examples:
  # List available engines
  ai-aas-cli engine list

  # Show engine details
  ai-aas-cli engine show vllm

  # Add a new engine version
  ai-aas-cli engine version add vllm 0.12.0 --image vllm/vllm-openai:v0.12.0

  # List engine configurations
  ai-aas-cli engine config list

  # Create a new configuration
  ai-aas-cli engine config create vllm/large-context \
    --engine vllm --gpu 1 --memory 32 --arg max-model-len=65536

See Also:
  ai-aas-cli model deploy create --engine-config <config>`,
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}

	// Add subcommands
	cmd.AddCommand(newEngineListCommand())
	cmd.AddCommand(newEngineShowCommand())
	cmd.AddCommand(newEngineVersionCommand())
	cmd.AddCommand(newEngineConfigCommand())

	return cmd
}

// newEngineListCommand creates the engine list subcommand
func newEngineListCommand() *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List available inference engines",
		Long: `List all available inference engines and their default versions.

Examples:
  # List all engines
  ai-aas-cli engine list

  # Output as JSON
  ai-aas-cli engine list --format json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get profile flag and load config with profile support
			profileName, _ := cmd.Flags().GetString("profile")
			cfg, _, err := config.GetEffectiveConfig(profileName)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			adminEndpoint := cfg.GetAdminEndpoint()

			if adminEndpoint == "" || adminEndpoint == "http://localhost:8080" {
				return fmt.Errorf("Admin API endpoint not configured. Run 'ai-aas-cli --init' first")
			}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			opts := []api.ClientOption{}
			if cfg.TLSInsecure {
				opts = append(opts, api.WithInsecureSkipVerify())
			}
			apiClient := api.NewClient(adminEndpoint, cfg.APIKey, opts...)
			engClient := engines.NewClient(apiClient)

			engineList, err := engClient.ListEngines(ctx)
			if err != nil {
				return fmt.Errorf("failed to list engines: %w", err)
			}

			if len(engineList) == 0 {
				fmt.Println("No engines found.")
				return nil
			}

			if format == "json" {
				return output.PrintJSON(engineList, true)
			}

			// Table output
			table := output.NewTableWriter()
			table.SetHeader([]string{"NAME", "DISPLAY NAME", "VERSION", "PROTOCOLS", "FORMATS"})

			for _, e := range engineList {
				protocols := strings.Join(e.ProtocolVersions, ", ")
				formats := strings.Join(e.SupportedModelFormats, ", ")
				if len(formats) > 30 {
					formats = formats[:27] + "..."
				}

				table.Append([]string{
					e.Name,
					e.DisplayName,
					e.DefaultVersion,
					protocols,
					formats,
				})
			}

			table.Render()
			fmt.Printf("\nTotal: %d engine(s)\n", len(engineList))

			return nil
		},
	}

	cmd.Flags().StringVarP(&format, "format", "f", "table", "output format (table, json)")

	return cmd
}

// newEngineShowCommand creates the engine show subcommand
func newEngineShowCommand() *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "show <engine-name>",
		Short: "Show engine details and versions",
		Long: `Display detailed information about an inference engine including all versions.

Examples:
  # Show vLLM engine details
  ai-aas-cli engine show vllm

  # Output as JSON
  ai-aas-cli engine show vllm --format json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			engineName := args[0]

			// Get profile flag and load config with profile support
			profileName, _ := cmd.Flags().GetString("profile")
			cfg, _, err := config.GetEffectiveConfig(profileName)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			adminEndpoint := cfg.GetAdminEndpoint()

			if adminEndpoint == "" || adminEndpoint == "http://localhost:8080" {
				return fmt.Errorf("Admin API endpoint not configured. Run 'ai-aas-cli --init' first")
			}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			opts := []api.ClientOption{}
			if cfg.TLSInsecure {
				opts = append(opts, api.WithInsecureSkipVerify())
			}
			apiClient := api.NewClient(adminEndpoint, cfg.APIKey, opts...)
			engClient := engines.NewClient(apiClient)

			engine, err := engClient.GetEngine(ctx, engineName)
			if err != nil {
				return fmt.Errorf("engine not found: %s", engineName)
			}

			versions, err := engClient.ListVersions(ctx, engineName)
			if err != nil {
				return fmt.Errorf("failed to get versions: %w", err)
			}

			if format == "json" {
				data := map[string]interface{}{
					"engine":   engine,
					"versions": versions,
				}
				return output.PrintJSON(data, true)
			}

			// Display engine info
			fmt.Printf("Engine: %s\n", engine.Name)
			fmt.Println("────────────────────────────────────────────────────")

			fmt.Printf("\nDetails\n")
			fmt.Printf("  Display Name:    %s\n", engine.DisplayName)
			if engine.Description != "" {
				fmt.Printf("  Description:     %s\n", engine.Description)
			}
			fmt.Printf("  Default Version: %s\n", engine.DefaultVersion)
			fmt.Printf("  Protocols:       %s\n", strings.Join(engine.ProtocolVersions, ", "))
			fmt.Printf("  Model Formats:   %s\n", strings.Join(engine.SupportedModelFormats, ", "))

			fmt.Printf("\nVersions\n")
			if len(versions) == 0 {
				fmt.Printf("  No versions available\n")
			} else {
				for _, v := range versions {
					defaultMark := ""
					if v.IsDefault {
						defaultMark = " (default)"
					}
					deprecatedMark := ""
					if v.IsDeprecated {
						deprecatedMark = " [deprecated]"
					}
					fmt.Printf("  %s%s%s\n", v.Version, defaultMark, deprecatedMark)
					fmt.Printf("    Image: %s\n", v.ContainerImage)
					if v.MinGPUMemoryGB != nil {
						fmt.Printf("    Min GPU Memory: %d GB\n", *v.MinGPUMemoryGB)
					}
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&format, "format", "f", "table", "output format (table, json)")

	return cmd
}

// newEngineVersionCommand creates the engine version parent command
func newEngineVersionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Manage engine versions",
		Long: `Manage versions for inference engines.

Examples:
  # List versions for an engine
  ai-aas-cli engine show vllm

  # Add a new version
  ai-aas-cli engine version add vllm 0.12.0 --image vllm/vllm-openai:v0.12.0

  # Set default version
  ai-aas-cli engine version set-default vllm 0.12.0`,
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}

	cmd.AddCommand(newVersionAddCommand())
	cmd.AddCommand(newVersionSetDefaultCommand())
	cmd.AddCommand(newVersionDeleteCommand())

	return cmd
}

// newVersionAddCommand creates the version add subcommand
func newVersionAddCommand() *cobra.Command {
	var (
		image        string
		setDefault   bool
		minGPUMemory int
		releaseNotes string
	)

	cmd := &cobra.Command{
		Use:   "add <engine> <version>",
		Short: "Add a new engine version",
		Long: `Add a new version for an inference engine.

Examples:
  # Add a new vLLM version
  ai-aas-cli engine version add vllm 0.12.0 --image vllm/vllm-openai:v0.12.0

  # Add and set as default
  ai-aas-cli engine version add vllm 0.12.0 --image vllm/vllm-openai:v0.12.0 --set-default

  # Add with GPU memory requirement
  ai-aas-cli engine version add vllm 0.12.0 --image vllm/vllm-openai:v0.12.0 --min-gpu-memory 24`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			engineName := args[0]
			version := args[1]

			if image == "" {
				return fmt.Errorf("--image flag is required")
			}

			// Get profile flag and load config with profile support
			profileName, _ := cmd.Flags().GetString("profile")
			cfg, _, err := config.GetEffectiveConfig(profileName)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			adminEndpoint := cfg.GetAdminEndpoint()

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			opts := []api.ClientOption{}
			if cfg.TLSInsecure {
				opts = append(opts, api.WithInsecureSkipVerify())
			}
			apiClient := api.NewClient(adminEndpoint, cfg.APIKey, opts...)
			engClient := engines.NewClient(apiClient)

			req := engines.AddVersionRequest{
				Version:        version,
				ContainerImage: image,
				SetDefault:     setDefault,
				ReleaseNotes:   releaseNotes,
			}
			if minGPUMemory > 0 {
				req.MinGPUMemoryGB = &minGPUMemory
			}

			v, err := engClient.AddVersion(ctx, engineName, req)
			if err != nil {
				return fmt.Errorf("failed to add version: %w", err)
			}

			fmt.Printf("✓ Added version %s for engine %s\n", v.Version, engineName)
			fmt.Printf("  Image: %s\n", v.ContainerImage)
			if v.IsDefault {
				fmt.Printf("  Set as default version\n")
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&image, "image", "", "container image (required)")
	cmd.Flags().BoolVar(&setDefault, "set-default", false, "set this version as the default")
	cmd.Flags().IntVar(&minGPUMemory, "min-gpu-memory", 0, "minimum GPU memory required (GB)")
	cmd.Flags().StringVar(&releaseNotes, "release-notes", "", "release notes for this version")

	cmd.MarkFlagRequired("image")

	return cmd
}

// newVersionSetDefaultCommand creates the version set-default subcommand
func newVersionSetDefaultCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set-default <engine> <version>",
		Short: "Set the default version for an engine",
		Long: `Set the default version for an inference engine.

The default version is used when no specific version is specified in a config.

Examples:
  # Set vLLM default to 0.12.0
  ai-aas-cli engine version set-default vllm 0.12.0`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			engineName := args[0]
			version := args[1]

			// Get profile flag and load config with profile support
			profileName, _ := cmd.Flags().GetString("profile")
			cfg, _, err := config.GetEffectiveConfig(profileName)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			adminEndpoint := cfg.GetAdminEndpoint()

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			opts := []api.ClientOption{}
			if cfg.TLSInsecure {
				opts = append(opts, api.WithInsecureSkipVerify())
			}
			apiClient := api.NewClient(adminEndpoint, cfg.APIKey, opts...)
			engClient := engines.NewClient(apiClient)

			if err := engClient.SetDefaultVersion(ctx, engineName, version); err != nil {
				return fmt.Errorf("failed to set default version: %w", err)
			}

			fmt.Printf("✓ Set default version for %s to %s\n", engineName, version)

			return nil
		},
	}

	return cmd
}

// newVersionDeleteCommand creates the version delete subcommand
func newVersionDeleteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <engine> <version>",
		Short: "Delete an engine version",
		Long: `Delete a version from an inference engine.

Examples:
  # Delete vLLM version 0.6.2
  ai-aas-cli engine version delete vllm 0.6.2`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			engineName := args[0]
			version := args[1]

			// Get profile flag and load config with profile support
			profileName, _ := cmd.Flags().GetString("profile")
			cfg, _, err := config.GetEffectiveConfig(profileName)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			adminEndpoint := cfg.GetAdminEndpoint()

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			opts := []api.ClientOption{}
			if cfg.TLSInsecure {
				opts = append(opts, api.WithInsecureSkipVerify())
			}
			apiClient := api.NewClient(adminEndpoint, cfg.APIKey, opts...)
			engClient := engines.NewClient(apiClient)

			if err := engClient.DeleteVersion(ctx, engineName, version); err != nil {
				return fmt.Errorf("failed to delete version: %w", err)
			}

			fmt.Printf("✓ Deleted version %s from engine %s\n", version, engineName)

			return nil
		},
	}

	return cmd
}

// newEngineConfigCommand creates the engine config parent command
func newEngineConfigCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage engine configurations",
		Long: `Manage inference engine configurations (resource profiles).

Configurations define resource requirements and engine-specific arguments
for deploying models. Use configurations with model deployments via
--engine-config flag.

Examples:
  # List all configurations
  ai-aas-cli engine config list

  # Show config details
  ai-aas-cli engine config show vllm/default

  # Create a new configuration
  ai-aas-cli engine config create vllm/large-context \
    --engine vllm --gpu 1 --memory 32 --arg max-model-len=65536`,
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}

	cmd.AddCommand(newConfigListCommand())
	cmd.AddCommand(newConfigShowCommand())
	cmd.AddCommand(newConfigCreateCommand())
	cmd.AddCommand(newConfigDeleteCommand())

	return cmd
}

// newConfigListCommand creates the config list subcommand
func newConfigListCommand() *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List engine configurations",
		Long: `List all engine configurations (resource profiles).

Examples:
  # List all configurations
  ai-aas-cli engine config list

  # Output as JSON
  ai-aas-cli engine config list --format json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get profile flag and load config with profile support
			profileName, _ := cmd.Flags().GetString("profile")
			cfg, _, err := config.GetEffectiveConfig(profileName)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			adminEndpoint := cfg.GetAdminEndpoint()

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			opts := []api.ClientOption{}
			if cfg.TLSInsecure {
				opts = append(opts, api.WithInsecureSkipVerify())
			}
			apiClient := api.NewClient(adminEndpoint, cfg.APIKey, opts...)
			engClient := engines.NewClient(apiClient)

			configs, err := engClient.ListConfigs(ctx)
			if err != nil {
				return fmt.Errorf("failed to list configs: %w", err)
			}

			if len(configs) == 0 {
				fmt.Println("No configurations found.")
				return nil
			}

			if format == "json" {
				return output.PrintJSON(configs, true)
			}

			// Table output
			table := output.NewTableWriter()
			table.SetHeader([]string{"NAME", "ENGINE", "GPU", "MEMORY", "DEFAULT"})

			for _, c := range configs {
				isDefault := "No"
				if c.IsDefault {
					isDefault = "Yes"
				}

				table.Append([]string{
					c.Name,
					c.EngineName,
					fmt.Sprintf("%d", c.GPUCount),
					fmt.Sprintf("%d GB", c.MemoryGB),
					isDefault,
				})
			}

			table.Render()
			fmt.Printf("\nTotal: %d configuration(s)\n", len(configs))

			return nil
		},
	}

	cmd.Flags().StringVarP(&format, "format", "f", "table", "output format (table, json)")

	return cmd
}

// newConfigShowCommand creates the config show subcommand
func newConfigShowCommand() *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "show <config-name>",
		Short: "Show configuration details",
		Long: `Display detailed information about an engine configuration.

Examples:
  # Show config details
  ai-aas-cli engine config show vllm/default

  # Output as JSON
  ai-aas-cli engine config show vllm/default --format json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			configName := args[0]

			// Get profile flag and load config with profile support
			profileName, _ := cmd.Flags().GetString("profile")
			cfg, _, err := config.GetEffectiveConfig(profileName)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			adminEndpoint := cfg.GetAdminEndpoint()

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			opts := []api.ClientOption{}
			if cfg.TLSInsecure {
				opts = append(opts, api.WithInsecureSkipVerify())
			}
			apiClient := api.NewClient(adminEndpoint, cfg.APIKey, opts...)
			engClient := engines.NewClient(apiClient)

			config, err := engClient.GetConfig(ctx, configName)
			if err != nil {
				return fmt.Errorf("config not found: %s", configName)
			}

			if format == "json" {
				return output.PrintJSON(config, true)
			}

			// Display config info
			fmt.Printf("Config: %s\n", config.Name)
			fmt.Println("────────────────────────────────────────────────────")

			fmt.Printf("\nEngine\n")
			fmt.Printf("  Name:     %s\n", config.EngineName)
			if config.Version != "" {
				fmt.Printf("  Version:  %s\n", config.Version)
			} else {
				fmt.Printf("  Version:  (default)\n")
			}

			if config.Description != "" {
				fmt.Printf("\nDescription\n")
				fmt.Printf("  %s\n", config.Description)
			}

			fmt.Printf("\nResources\n")
			fmt.Printf("  GPU Count:   %d\n", config.GPUCount)
			if config.GPUType != "" {
				fmt.Printf("  GPU Type:    %s\n", config.GPUType)
			}
			fmt.Printf("  Memory:      %d GB\n", config.MemoryGB)
			fmt.Printf("  CPU Cores:   %d\n", config.CPUCores)

			fmt.Printf("\nScaling\n")
			fmt.Printf("  Min Replicas: %d\n", config.MinReplicas)
			fmt.Printf("  Max Replicas: %d\n", config.MaxReplicas)
			fmt.Printf("  Startup Timeout: %d seconds\n", config.StartupTimeoutSeconds)

			// Engine args
			if len(config.EngineArgs) > 0 && string(config.EngineArgs) != "{}" {
				fmt.Printf("\nEngine Arguments\n")
				var args map[string]interface{}
				if err := json.Unmarshal(config.EngineArgs, &args); err == nil {
					for k, v := range args {
						fmt.Printf("  %s: %v\n", k, v)
					}
				}
			}

			if config.IsDefault {
				fmt.Printf("\n[Default config for %s]\n", config.EngineName)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&format, "format", "f", "table", "output format (table, json)")

	return cmd
}

// newConfigCreateCommand creates the config create subcommand
func newConfigCreateCommand() *cobra.Command {
	var (
		engineName            string
		version               string
		description           string
		gpuCount              int
		gpuType               string
		memoryGB              int
		cpuCores              int
		engineArgs            []string
		minReplicas           int
		maxReplicas           int
		startupTimeoutSeconds int
		isDefault             bool
	)

	cmd := &cobra.Command{
		Use:   "create <config-name>",
		Short: "Create a new engine configuration",
		Long: `Create a new engine configuration (resource profile).

Configuration names should follow the format "engine/profile" (e.g., vllm/large-context).

Examples:
  # Create a basic config
  ai-aas-cli engine config create vllm/large-context \
    --engine vllm --gpu 1 --memory 32

  # Create with engine arguments
  ai-aas-cli engine config create vllm/large-context \
    --engine vllm --gpu 1 --memory 32 \
    --arg max-model-len=65536 --arg gpu-memory-utilization=0.95

  # Create multi-GPU config
  ai-aas-cli engine config create vllm/multi-gpu \
    --engine vllm --gpu 2 --memory 96 \
    --arg tensor-parallel-size=2`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			configName := args[0]

			if engineName == "" {
				return fmt.Errorf("--engine flag is required")
			}

			// Get profile flag and load config with profile support
			profileName, _ := cmd.Flags().GetString("profile")
			cfg, _, err := config.GetEffectiveConfig(profileName)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			adminEndpoint := cfg.GetAdminEndpoint()

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			opts := []api.ClientOption{}
			if cfg.TLSInsecure {
				opts = append(opts, api.WithInsecureSkipVerify())
			}
			apiClient := api.NewClient(adminEndpoint, cfg.APIKey, opts...)
			engClient := engines.NewClient(apiClient)

			// Parse engine args
			parsedArgs := make(map[string]interface{})
			for _, arg := range engineArgs {
				parts := strings.SplitN(arg, "=", 2)
				if len(parts) != 2 {
					return fmt.Errorf("invalid engine arg format: %s (expected key=value)", arg)
				}
				// Try to parse as number
				if num, err := parseNumber(parts[1]); err == nil {
					parsedArgs[parts[0]] = num
				} else {
					parsedArgs[parts[0]] = parts[1]
				}
			}

			req := engines.CreateConfigRequest{
				Name:                  configName,
				EngineName:            engineName,
				Version:               version,
				Description:           description,
				GPUCount:              gpuCount,
				GPUType:               gpuType,
				MemoryGB:              memoryGB,
				CPUCores:              cpuCores,
				EngineArgs:            parsedArgs,
				MinReplicas:           minReplicas,
				MaxReplicas:           maxReplicas,
				StartupTimeoutSeconds: startupTimeoutSeconds,
				IsDefault:             isDefault,
			}

			created, err := engClient.CreateConfig(ctx, req)
			if err != nil {
				return fmt.Errorf("failed to create config: %w", err)
			}

			fmt.Printf("✓ Created configuration: %s\n", created.Name)
			fmt.Printf("  Engine:  %s\n", created.EngineName)
			fmt.Printf("  GPU:     %d\n", created.GPUCount)
			fmt.Printf("  Memory:  %d GB\n", created.MemoryGB)

			return nil
		},
	}

	cmd.Flags().StringVar(&engineName, "engine", "", "inference engine name (required)")
	cmd.Flags().StringVar(&version, "version", "", "engine version (uses default if not specified)")
	cmd.Flags().StringVar(&description, "description", "", "configuration description")
	cmd.Flags().IntVar(&gpuCount, "gpu", 1, "number of GPUs")
	cmd.Flags().StringVar(&gpuType, "gpu-type", "", "GPU type (e.g., rtx-4090, a100-40gb)")
	cmd.Flags().IntVar(&memoryGB, "memory", 16, "memory in GB")
	cmd.Flags().IntVar(&cpuCores, "cpu", 4, "number of CPU cores")
	cmd.Flags().StringArrayVar(&engineArgs, "arg", nil, "engine argument (key=value, can be repeated)")
	cmd.Flags().IntVar(&minReplicas, "min-replicas", 1, "minimum replicas")
	cmd.Flags().IntVar(&maxReplicas, "max-replicas", 1, "maximum replicas")
	cmd.Flags().IntVar(&startupTimeoutSeconds, "startup-timeout", 900, "startup timeout in seconds")
	cmd.Flags().BoolVar(&isDefault, "default", false, "set as default config for this engine")

	cmd.MarkFlagRequired("engine")

	return cmd
}

// newConfigDeleteCommand creates the config delete subcommand
func newConfigDeleteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <config-name>",
		Short: "Delete an engine configuration",
		Long: `Delete an engine configuration.

Cannot delete a configuration that is in use by deployments.

Examples:
  # Delete a configuration
  ai-aas-cli engine config delete vllm/large-context`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			configName := args[0]

			// Get profile flag and load config with profile support
			profileName, _ := cmd.Flags().GetString("profile")
			cfg, _, err := config.GetEffectiveConfig(profileName)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			adminEndpoint := cfg.GetAdminEndpoint()

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			opts := []api.ClientOption{}
			if cfg.TLSInsecure {
				opts = append(opts, api.WithInsecureSkipVerify())
			}
			apiClient := api.NewClient(adminEndpoint, cfg.APIKey, opts...)
			engClient := engines.NewClient(apiClient)

			if err := engClient.DeleteConfig(ctx, configName); err != nil {
				return fmt.Errorf("failed to delete config: %w", err)
			}

			fmt.Printf("✓ Deleted configuration: %s\n", configName)

			return nil
		},
	}

	return cmd
}

// parseNumber tries to parse a string as a number
func parseNumber(s string) (interface{}, error) {
	// Try int first
	var i int
	if _, err := fmt.Sscanf(s, "%d", &i); err == nil {
		return i, nil
	}
	// Try float
	var f float64
	if _, err := fmt.Sscanf(s, "%f", &f); err == nil {
		return f, nil
	}
	return nil, fmt.Errorf("not a number")
}
