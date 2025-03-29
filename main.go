package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/hashicorp/vault/api"
	"github.com/spf13/cobra"
)

var (
	kvPath   string
	role     string
	kvEngine string
)

type Config struct {
	Secrets struct {
		Path   string `json:"path"`
		Role   string `json:"role"`
		Engine string `json:"engine"`
	} `json:"secrets"`
}

func loadConfig() (*Config, error) {
	// Read the config file
	data, err := os.ReadFile(".idot.json")
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read config file: %v", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %v", err)
	}

	return &config, nil
}

func getSecrets() (map[string]string, error) {
	// Load config file
	idotConfig, err := loadConfig()
	if err != nil {
		return nil, err
	}

	// Use config values if flags are not set
	if idotConfig != nil {
		if kvPath == "" {
			kvPath = idotConfig.Secrets.Path
		}
		if role == "" {
			role = idotConfig.Secrets.Role
		}
		if kvEngine == "" {
			kvEngine = idotConfig.Secrets.Engine
		}
	}

	// Check environment variables if flags and config are not set
	if kvPath == "" {
		kvPath = os.Getenv("VAULT_LOADER_PATH")
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

	// Check if path is provided
	if kvPath == "" {
		return nil, fmt.Errorf("path is required: provide it with --path flag, VAULT_LOADER_PATH environment variable, or in .idot.json config file")
	}

	// Initialize Vault client
	vaultConfig := api.DefaultConfig()
	client, err := api.NewClient(vaultConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Vault client: %v", err)
	}

	// Check for VAULT_TOKEN or VAULT_ID_TOKEN
	token := os.Getenv("VAULT_TOKEN")
	idToken := os.Getenv("VAULT_ID_TOKEN")

	if token == "" && idToken == "" {
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
			return nil, fmt.Errorf("--role is required when VAULT_ID_TOKEN is set")
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
	}

	// Set the token and get secrets
	client.SetToken(token)

	secret, err := client.KVv2(kvEngine).Get(context.Background(), kvPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read secrets: %v", err)
	}

	if secret == nil || secret.Data == nil {
		return nil, fmt.Errorf("no secrets found at path: %s", kvPath)
	}

	// Convert secrets to map[string]string
	secrets := make(map[string]string)
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

	return secrets, nil
}

func main() {
	var rootCmd = &cobra.Command{
		Use:   "vault-loader",
		Short: "Load secrets from Vault and export them as environment variables",
	}

	// Global flags
	rootCmd.PersistentFlags().StringVarP(&kvPath, "path", "p", "", "Path to the KV secrets (required)")
	rootCmd.PersistentFlags().StringVarP(&role, "role", "r", "", "Role to use when resolving ID token (required if VAULT_ID_TOKEN is set)")
	rootCmd.PersistentFlags().StringVarP(&kvEngine, "engine", "e", "kv", "Name of the KV secrets engine")

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
				// Escape single quotes in the value
				value = strings.ReplaceAll(value, "'", "'\\''")
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
				// Escape quotes and newlines in the value
				value = strings.ReplaceAll(value, "\"", "\\\"")
				value = strings.ReplaceAll(value, "\n", "\\n")
				fmt.Fprintf(file, "%s=\"%s\"\n", key, value)
			}

			return nil
		},
	}

	// Run command
	var runCmd = &cobra.Command{
		Use:   "run [command]",
		Short: "Run a command with the secrets as environment variables",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			secrets, err := getSecrets()
			if err != nil {
				return err
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

	rootCmd.AddCommand(exportCmd, envCmd, runCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
