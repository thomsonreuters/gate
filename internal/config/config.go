// Copyright 2026 Thomson Reuters
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package config provides application configuration loading, validation, and access.
// Configuration is loaded from file (YAML) and environment variables via Viper,
// then validated and stored for concurrency-safe access via GetCurrent.
package config

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/spf13/viper"
	"github.com/thomsonreuters/gate/internal/constants"
	"github.com/thomsonreuters/gate/internal/logger"
)

const (
	// ConfigRootLinux is the default config directory root on Linux.
	ConfigRootLinux = "/etc/"
	// ConfigRootWindows is the default config directory root on Windows.
	ConfigRootWindows = "C:\\ProgramData\\"
	// ConfigRootDarwin is the default config directory root on macOS.
	ConfigRootDarwin = "/Library/Application Support/"
	// ConfigFileName is the base name of the config file (without extension).
	ConfigFileName = "config"
	// ConfigFileExtension is the config file extension used by Viper.
	ConfigFileExtension = "yaml"
	// KeyAWSRegion is the Viper key for the AWS region setting.
	KeyAWSRegion = "aws_region"
)

// Config holds the full application configuration. All nested sections are validated on Load.
type Config struct {
	Logger     LoggerConfig      `mapstructure:"logger"`
	Audit      AuditConfig       `mapstructure:"audit"`
	Selector   SelectorConfig    `mapstructure:"selector"`
	Policy     PolicyConfig      `mapstructure:"policy"`
	Server     ServerConfig      `mapstructure:"server"`
	OIDC       OIDCConfig        `mapstructure:"oidc"`
	Origin     OriginConfig      `mapstructure:"origin"`
	FIPS       FIPSConfig        `mapstructure:"fips"`
	GitHubApps []GitHubAppConfig `mapstructure:"github_apps"`

	AWSRegion string `mapstructure:"aws_region"`
}

// Validate validates all configuration sections and returns the first error encountered.
func (c *Config) Validate() error {
	var validators = []func() error{
		c.Logger.Validate,
		c.Audit.Validate,
		c.Selector.Validate,
		c.Policy.Validate,
		c.Server.Validate,
		c.OIDC.Validate,
		c.Origin.Validate,
		c.FIPS.Validate,
		func() error { return ValidateGitHubApps(c.GitHubApps) },
	}

	for _, validator := range validators {
		if err := validator(); err != nil {
			return err
		}
	}

	return nil
}

// getDirectory returns the OS-specific default config directory for the application.
func (c *Config) getDirectory() string {
	var directory string
	switch runtime.GOOS {
	case "linux":
		directory = ConfigRootLinux
	case "windows":
		directory = ConfigRootWindows
	case "darwin":
		directory = ConfigRootDarwin
	}
	return filepath.Join(directory, constants.ProgramIdentifier)
}

// getViper builds a Viper instance with config file search paths, env binding, and defaults.
func (c *Config) getViper(path string) *viper.Viper {
	v := viper.New()
	v.SetConfigName(ConfigFileName)
	v.SetConfigType(ConfigFileExtension)

	if path != "" {
		v.SetConfigFile(path)
	} else {
		v.AddConfigPath(".")
		v.AddConfigPath(c.getDirectory())
	}

	v.SetEnvPrefix(strings.ToUpper(constants.ProgramIdentifier))
	v.SetEnvKeyReplacer(strings.NewReplacer(`.`, `_`, `-`, `_`))
	v.AutomaticEnv()

	setDefaults(v)
	bindEnv(v)

	return v
}

var (
	current   *Config
	currentMu sync.RWMutex
)

// Load loads configuration from file (if present) and environment
// variables, validates, and sets it as current.
func Load(ctx context.Context, configPath string) (*Config, error) {
	cfg := &Config{}
	v := cfg.getViper(configPath)

	if err := v.ReadInConfig(); err != nil {
		var notFoundErr viper.ConfigFileNotFoundError
		if errors.As(err, &notFoundErr) {
			slog.WarnContext(ctx, "No config file found, relying on env vars/defaults")
		} else {
			return nil, err
		}
	} else {
		slog.DebugContext(ctx, "Using config file", slog.String("file", v.ConfigFileUsed()))
	}

	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	logger.SetGlobalLogger(cfg.Logger.Level, cfg.Logger.Format)

	SetCurrent(cfg)

	return cfg, nil
}

// GetCurrent returns the current application configuration in a concurrency-safe manner.
func GetCurrent() *Config {
	currentMu.RLock()
	defer currentMu.RUnlock()
	return current
}

// SetCurrent sets the current application configuration in a concurrency-safe manner.
func SetCurrent(cfg *Config) {
	currentMu.Lock()
	defer currentMu.Unlock()
	current = cfg
}

// bindEnv binds all supported config keys to environment variables.
func bindEnv(v *viper.Viper) {
	mustBindEnv(v, KeyLoggerLevel, KeyLoggerFormat)

	mustBindEnv(v, KeyServerPort, KeyServerReadTimeout, KeyServerWriteTimeout, KeyServerShutdownTimeout,
		KeyServerRequestTimeout, KeyServerIdleTimeout, KeyServerWaitTimeout,
		KeyServerTLSCertFilePath, KeyServerTLSKeyFilePath)

	mustBindEnv(v, KeyOIDCAudience)

	mustBindEnv(v, KeyAuditBackend, KeyAuditDynamoDBTableName, KeyAuditDynamoDBTTLDays, KeyAuditSQLDSN)

	mustBindEnv(v, KeySelectorType, KeySelectorRedisAddress, KeySelectorRedisPassword,
		KeySelectorRedisDB, KeySelectorRedisTLS, KeySelectorDynamoDBTableName,
		KeySelectorDynamoDBTTLMinutes)

	mustBindEnv(v, KeyPolicyVersion, KeyPolicyTrustPolicyPath, KeyPolicyDefaultTokenTTL, KeyPolicyMaxTokenTTL,
		KeyPolicyRequireExplicitPolicy, KeyPolicyGitHubAPIBaseURL, KeyPolicyGitHubRawBaseURL,
		KeyPolicyProviders, KeyPolicyMaxPermissions)

	mustBindEnv(v, KeyOriginEnabled, KeyOriginHeaderName, KeyOriginHeaderValue)

	mustBindEnv(v, KeyFIPSEnabled, KeyFIPSMode)

	mustBindEnv(v, KeyAWSRegion)
}

// setDefaults applies default values for all config keys that support them.
func setDefaults(v *viper.Viper) {
	v.SetDefault(KeyLoggerLevel, string(logger.LogLevelInfo))
	v.SetDefault(KeyLoggerFormat, string(logger.LogFormatJSON))

	v.SetDefault(KeyServerPort, DefaultServerPort)
	v.SetDefault(KeyServerReadTimeout, DefaultServerReadTimeout)
	v.SetDefault(KeyServerWriteTimeout, DefaultServerWriteTimeout)
	v.SetDefault(KeyServerShutdownTimeout, DefaultServerShutdownTimeout)
	v.SetDefault(KeyServerRequestTimeout, DefaultServerRequestTimeout)
	v.SetDefault(KeyServerIdleTimeout, DefaultServerIdleTimeout)
	v.SetDefault(KeyServerWaitTimeout, DefaultServerWaitTimeout)

	v.SetDefault(KeyOIDCAudience, DefaultOIDCAudience)

	v.SetDefault(KeyAuditDynamoDBTableName, DefaultDynamoDBTableName)
	v.SetDefault(KeyAuditDynamoDBTTLDays, DefaultDynamoDBTTLDays)

	v.SetDefault(KeySelectorType, string(DefaultSelectorStoreType))
	v.SetDefault(KeySelectorRedisDB, DefaultRedisDB)

	v.SetDefault(KeyPolicyVersion, DefaultPolicyVersion)
	v.SetDefault(KeyPolicyDefaultTokenTTL, DefaultPolicyDefaultTokenTTL)
	v.SetDefault(KeyPolicyMaxTokenTTL, DefaultPolicyMaxTokenTTL)
	v.SetDefault(KeyPolicyGitHubAPIBaseURL, DefaultGitHubAPIBaseURL)
	v.SetDefault(KeyPolicyGitHubRawBaseURL, DefaultGitHubRawBaseURL)
}

// mustBindEnv binds each key to its environment variable; panics on failure.
func mustBindEnv(v *viper.Viper, keys ...string) {
	for _, k := range keys {
		if err := v.BindEnv(k); err != nil {
			panic(fmt.Sprintf("failed to bind env var %s: %v", k, err))
		}
	}
}
