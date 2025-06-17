package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/hashicorp/vault/api"
	"github.com/spf13/cobra"
)

var (
	kvPaths      []string
	role         string
	kvEngine     string
	roleID       string
	secretID     string
	unquoted     bool
	ignoreIfFail bool
)

type Config struct {
	Secrets struct {
		Paths  []string `json:"paths"`
		Path   string   `json:"path"` // For backward compatibility
		Role   string   `json:"role"`
		Engine string   `json:"engine"`
	} `json:"secrets"`
}

var Version = "nightly"

func loadConfig() (*Config, error) {
	// Get current working directory
	currentDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current directory: %v", err)
	}

	// Search for config file recursively up the directory tree
	for {
		// Try to read the config file in current directory
		data, err := os.ReadFile(filepath.Join(currentDir, ".idot.json"))
		if err == nil {
			var config Config
			if err := json.Unmarshal(data, &config); err != nil {
				return nil, fmt.Errorf("failed to parse config file: %v", err)
			}
			return &config, nil
		}

		// If file doesn't exist, try parent directory
		if os.IsNotExist(err) {
			parentDir := filepath.Dir(currentDir)
			// Stop if we've reached the root directory
			if parentDir == currentDir {
				return nil, nil
			}
			currentDir = parentDir
			continue
		}

		// If there's any other error, return it
		return nil, fmt.Errorf("failed to read config file: %v", err)
	}
}

func getSecrets() (map[string]string, error) {
	// Load config file
	idotConfig, err := loadConfig()
	if err != nil {
		return nil, err
	}

	// Use config values if flags are not set
	if idotConfig != nil {
		if len(kvPaths) == 0 {
			if idotConfig.Secrets.Path != "" {
				kvPaths = []string{idotConfig.Secrets.Path}
			} else if len(idotConfig.Secrets.Paths) > 0 {
				kvPaths = idotConfig.Secrets.Paths
			}
		}
		if role == "" {
			role = idotConfig.Secrets.Role
		}
		if kvEngine == "" {
			kvEngine = idotConfig.Secrets.Engine
		}
	}

	// Check environment variables if flags and config are not set
	if len(kvPaths) == 0 {
		pathStr := os.Getenv("VAULT_LOADER_PATH")
		if pathStr != "" {
			kvPaths = strings.Split(pathStr, ",")
		}
	}
	if role == "" {
		role = os.Getenv("VAULT_LOADER_ROLE")
	}
	if kvEngine == "" {
		kvEngine = os.Getenv("VAULT_LOADER_ENGINE")
		if kvEngine == "" {
			kvEngine = "kv" // Default value
		}
	}
	if roleID == "" {
		roleID = os.Getenv("VAULT_ROLE_ID")
	}
	if secretID == "" {
		secretID = os.Getenv("VAULT_SECRET_ID")
	}

	// Check if paths are provided
	if len(kvPaths) == 0 {
		return nil, fmt.Errorf("path(s) are required: provide them with --path flag, VAULT_LOADER_PATH environment variable, or in .idot.json config file")
	}

	// Initialize Vault client
	vaultConfig := api.DefaultConfig()
	client, err := api.NewClient(vaultConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Vault client: %v", err)
	}

	// Check for VAULT_TOKEN, VAULT_ID_TOKEN, or AppRole credentials
	token := os.Getenv("VAULT_TOKEN")
	idToken := os.Getenv("VAULT_ID_TOKEN")

	if token == "" && idToken == "" && roleID == "" && secretID == "" {
		// Try to get token from vault CLI session
		cmd := exec.Command("vault", "token", "lookup", "-format=json")
		output, err := cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("failed to get token from vault CLI: %v", err)
		}

		var result struct {
			Data struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		if err := json.Unmarshal(output, &result); err != nil {
			return nil, fmt.Errorf("failed to parse vault CLI output: %v", err)
		}
		token = result.Data.ID
	} else if idToken != "" {
		if role == "" {
			// Calculate role name from path by replacing slashes with underscores
			role = strings.ReplaceAll(kvPaths[0], "/", "_")
			fmt.Fprintf(os.Stderr, "No role specified, using calculated role name: %s\n", role)
		}
		// Resolve ID token to access token
		client.SetToken(idToken)
		secret, err := client.Logical().Write("auth/jwt/login", map[string]interface{}{
			"role": role,
			"jwt":  idToken,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to resolve ID token: %v", err)
		}
		token = secret.Auth.ClientToken
	} else if roleID != "" && secretID != "" {
		// Authenticate using AppRole
		secret, err := client.Logical().Write("auth/approle/login", map[string]interface{}{
			"role_id":   roleID,
			"secret_id": secretID,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to authenticate with AppRole: %v", err)
		}
		token = secret.Auth.ClientToken
	}

	// Set the token and get secrets from all paths
	client.SetToken(token)
	secrets := make(map[string]string)

	for _, path := range kvPaths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}

		secret, err := client.KVv2(kvEngine).Get(context.Background(), path)
		if err != nil {
			return nil, fmt.Errorf("failed to read secrets from path %s: %v", path, err)
		}

		if secret == nil || secret.Data == nil {
			return nil, fmt.Errorf("no secrets found at path: %s", path)
		}

		// Convert secrets to map[string]string
		for key, value := range secret.Data {
			// Convert the value to string
			var strValue string
			switch v := value.(type) {
			case string:
				strValue = v
			case []byte:
				strValue = string(v)
			default:
				strValue = fmt.Sprintf("%v", v)
			}
			secrets[strings.ToUpper(key)] = strValue
		}
	}

	return secrets, nil
}

func main() {
	var rootCmd = &cobra.Command{
		Use:   "vault-loader",
		Short: "Load secrets from Vault and export them as environment variables",
		Version: Version,
	}

	// Global flags
	rootCmd.PersistentFlags().StringSliceVarP(&kvPaths, "path", "p", []string{}, "Comma-separated paths to the KV secrets (required)")
	rootCmd.PersistentFlags().StringVarP(&role, "role", "r", "", "Role to use when resolving ID token (required if VAULT_ID_TOKEN is set)")
	rootCmd.PersistentFlags().StringVarP(&kvEngine, "engine", "e", "kv", "Name of the KV secrets engine")
	rootCmd.PersistentFlags().StringVarP(&roleID, "role-id", "", "", "Role ID for AppRole authentication")
	rootCmd.PersistentFlags().StringVarP(&secretID, "secret-id", "", "", "Secret ID for AppRole authentication")

	// Export command
	var exportCmd = &cobra.Command{
		Use:   "export",
		Short: "Export secrets as bash commands",
		RunE: func(cmd *cobra.Command, args []string) error {
			secrets, err := getSecrets()
			if err != nil {
				return err
			}

			// Output bash commands to set environment variables
			for key, value := range secrets {
				// Escape special characters in the value
				value = strings.ReplaceAll(value, "'", "'\\''")
				value = strings.ReplaceAll(value, "\n", "\\n")
				value = strings.ReplaceAll(value, "\r", "\\r")
				value = strings.ReplaceAll(value, "\t", "\\t")
				fmt.Printf("export %s='%s'\n", key, value)
			}

			return nil
		},
	}

	// Env command
	var envCmd = &cobra.Command{
		Use:   "env",
		Short: "Create a .env file with the secrets",
		RunE: func(cmd *cobra.Command, args []string) error {
			secrets, err := getSecrets()
			if err != nil {
				return err
			}

			// Create .env file
			file, err := os.Create(".env")
			if err != nil {
				return fmt.Errorf("failed to create .env file: %v", err)
			}
			defer file.Close()

			// Write secrets to file
			for key, value := range secrets {
				if !unquoted {
					// Escape quotes and newlines in the value
					value = strings.ReplaceAll(value, "\"", "\\\"")
					value = strings.ReplaceAll(value, "\n", "\\n")
					fmt.Fprintf(file, "%s=\"%s\"\n", key, value)
				} else {
					fmt.Fprintf(file, "%s=%s\n", key, value)
				}
			}

			return nil
		},
	}

	// Add the unquoted flag to env command
	envCmd.Flags().BoolVarP(&unquoted, "unquoted", "u", false, "Do not wrap values in quotes")

	// Run command
	var runCmd = &cobra.Command{
		Use:   "run [command]",
		Short: "Run a command with the secrets as environment variables",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var secrets map[string]string

			secrets, err := getSecrets()
			if err != nil {
				if ignoreIfFail {
					fmt.Fprintf(os.Stderr, "skipped secret loading from hc-vault\n")
					secrets = make(map[string]string)
				} else {
					return err
				}
			}

			// Create command with environment variables
			command := exec.Command(args[0], args[1:]...)
			command.Env = os.Environ()

			// Add secrets to environment
			for key, value := range secrets {
				command.Env = append(command.Env, fmt.Sprintf("%s=%s", key, value))
			}

			// Run the command
			command.Stdin = os.Stdin
			command.Stdout = os.Stdout
			command.Stderr = os.Stderr

			return command.Run()
		},
	}

	// Add the ignore-if-fail flag to run command
	runCmd.Flags().BoolVar(&ignoreIfFail, "ignore-if-fail", false, "Continue running command even if secret loading fails")

	rootCmd.AddCommand(exportCmd, envCmd, runCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
