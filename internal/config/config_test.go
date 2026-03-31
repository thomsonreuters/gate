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

package config

import (
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thomsonreuters/gate/internal/constants"
	"github.com/thomsonreuters/gate/internal/logger"
)

func validProviderConfig() ProviderConfig {
	return ProviderConfig{
		Issuer:          "https://token.actions.githubusercontent.com",
		Name:            "GitHub Actions",
		RequiredClaims:  map[string]string{},
		ForbiddenClaims: map[string]string{},
	}
}

func validGitHubAppConfig() GitHubAppConfig {
	return GitHubAppConfig{
		ClientID:       "client-1",
		PrivateKeyPath: "/etc/gate/private-key.pem",
		Organization:   "example-org",
	}
}

func validConfig() Config {
	return Config{
		Logger:   LoggerConfig{Level: logger.LogLevelInfo, Format: logger.LogFormatJSON},
		Selector: SelectorConfig{},
		Policy: PolicyConfig{
			Version:         "1.0",
			TrustPolicyPath: ".github/gate/trust-policy.yaml",
			DefaultTokenTTL: 900,
			MaxTokenTTL:     3600,
			Providers:       []ProviderConfig{validProviderConfig()},
			MaxPermissions:  map[string]string{"contents": "read"},
		},
		Server: ServerConfig{
			Port:            8080,
			ReadTimeout:     30 * time.Second,
			WriteTimeout:    30 * time.Second,
			ShutdownTimeout: 10 * time.Second,
			RequestTimeout:  30 * time.Second,
			IdleTimeout:     10 * time.Second,
			WaitTimeout:     10 * time.Second,
		},
		OIDC:   OIDCConfig{Audience: "gate"},
		Origin: OriginConfig{Enabled: false},
		FIPS:   FIPSConfig{Enabled: false},
	}
}

func TestConfig_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		modify func(*Config)
		fails  error
	}{
		{
			name: "valid",
		},
		{
			name:   "invalid_logger",
			modify: func(c *Config) { c.Logger.Level = "invalid" },
			fails:  ErrInvalidLogLevel,
		},
		{
			name:   "invalid_audit_backend",
			modify: func(c *Config) { c.Audit = AuditConfig{Backend: "unknown"} },
			fails:  ErrInvalidAuditBackendType,
		},
		{
			name:   "invalid_selector_store_type",
			modify: func(c *Config) { c.Selector = SelectorConfig{Type: "unknown"} },
			fails:  ErrInvalidSelectorStoreType,
		},
		{
			name:   "selector_redis_without_config",
			modify: func(c *Config) { c.Selector = SelectorConfig{Type: SelectorStoreTypeRedis} },
			fails:  ErrInvalidRedisConfig,
		},
		{
			name:   "invalid_policy",
			modify: func(c *Config) { c.Policy.Version = "" },
			fails:  ErrInvalidPolicyVersion,
		},
		{
			name:   "invalid_server",
			modify: func(c *Config) { c.Server.Port = 0 },
			fails:  ErrInvalidPort,
		},
		{
			name:   "invalid_oidc",
			modify: func(c *Config) { c.OIDC.Audience = "" },
			fails:  ErrInvalidOIDCAudience,
		},
		{
			name: "invalid_origin",
			modify: func(c *Config) {
				c.Origin = OriginConfig{Enabled: true, HeaderName: ""}
			},
			fails: ErrInvalidOriginHeaderName,
		},
		{
			name:   "invalid_fips",
			modify: func(c *Config) { c.FIPS = FIPSConfig{Enabled: true, Mode: "invalid"} },
			fails:  ErrInvalidFIPSMode,
		},
		{
			name: "invalid_github_apps",
			modify: func(c *Config) {
				c.GitHubApps = []GitHubAppConfig{{ClientID: ""}}
			},
			fails: ErrInvalidGithubAppClientID,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := validConfig()
			if tt.modify != nil {
				tt.modify(&cfg)
			}
			err := cfg.Validate()
			if tt.fails != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.fails)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestConfig_getDirectory(t *testing.T) {
	t.Parallel()

	cfg := &Config{}
	dir := cfg.getDirectory()

	var want string
	switch runtime.GOOS {
	case "linux":
		want = filepath.Join(ConfigRootLinux, constants.ProgramIdentifier)
	case "windows":
		want = filepath.Join(ConfigRootWindows, constants.ProgramIdentifier)
	case "darwin":
		want = filepath.Join(ConfigRootDarwin, constants.ProgramIdentifier)
	default:
		want = constants.ProgramIdentifier
	}

	assert.Equal(t, want, dir)
}

func TestLoad_ValidConfig(t *testing.T) {
	configPath := filepath.Join("testdata", "valid_app_config.yaml")

	cfg, err := Load(t.Context(), configPath)

	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, 8080, cfg.Server.Port)
	assert.Equal(t, logger.LogLevelInfo, cfg.Logger.Level)
	assert.Equal(t, logger.LogFormatJSON, cfg.Logger.Format)
	assert.Equal(t, "gate", cfg.OIDC.Audience)
	assert.Empty(t, cfg.Audit.Backend)
	assert.Equal(t, SelectorStoreTypeMemory, cfg.Selector.Type)
	assert.Equal(t, "1.0", cfg.Policy.Version)
	assert.Equal(t, ".github/gate/trust-policy.yaml", cfg.Policy.TrustPolicyPath)
	assert.Equal(t, 900, cfg.Policy.DefaultTokenTTL)
	assert.Equal(t, 3600, cfg.Policy.MaxTokenTTL)
	assert.Equal(t, DefaultGitHubAPIBaseURL, cfg.Policy.GitHubAPIBaseURL)
	assert.Equal(t, DefaultGitHubRawBaseURL, cfg.Policy.GitHubRawBaseURL)
	assert.Equal(t, cfg, GetCurrent())
}

func TestLoad_MinimalConfig_DefaultsApplied(t *testing.T) {
	configPath := filepath.Join("testdata", "minimal_config.yaml")

	cfg, err := Load(t.Context(), configPath)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, "gate", cfg.OIDC.Audience)
	assert.Equal(t, ".github/gate/trust-policy.yaml", cfg.Policy.TrustPolicyPath)
	assert.Equal(t, 8080, cfg.Server.Port)
	assert.Equal(t, 30*time.Second, cfg.Server.ReadTimeout)
	assert.Equal(t, 30*time.Second, cfg.Server.WriteTimeout)
	assert.Equal(t, 10*time.Second, cfg.Server.ShutdownTimeout)
	assert.Equal(t, 30*time.Second, cfg.Server.RequestTimeout)
	assert.Equal(t, 10*time.Second, cfg.Server.IdleTimeout)
	assert.Equal(t, 10*time.Second, cfg.Server.WaitTimeout)
	assert.False(t, cfg.Server.TLS.Enabled())
	assert.Equal(t, logger.LogLevelInfo, cfg.Logger.Level)
	assert.Equal(t, logger.LogFormatJSON, cfg.Logger.Format)
	assert.Empty(t, cfg.Audit.Backend)
	assert.Equal(t, SelectorStoreTypeMemory, cfg.Selector.Type)
	assert.Equal(t, "1.0", cfg.Policy.Version)
	assert.Equal(t, 900, cfg.Policy.DefaultTokenTTL)
	assert.Equal(t, 3600, cfg.Policy.MaxTokenTTL)
	assert.Equal(t, DefaultGitHubAPIBaseURL, cfg.Policy.GitHubAPIBaseURL)
	assert.Equal(t, DefaultGitHubRawBaseURL, cfg.Policy.GitHubRawBaseURL)
	assert.Empty(t, cfg.AWSRegion)
	assert.False(t, cfg.Origin.Enabled)
	assert.False(t, cfg.FIPS.Enabled)
	assert.Empty(t, cfg.GitHubApps)
}

func TestLoad_YAMLOverridesDefaults(t *testing.T) {
	configPath := filepath.Join("testdata", "overrides_config.yaml")

	cfg, err := Load(t.Context(), configPath)

	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, 9090, cfg.Server.Port)
	assert.Equal(t, 60*time.Second, cfg.Server.ReadTimeout)
	assert.Equal(t, 60*time.Second, cfg.Server.WriteTimeout)
	assert.Equal(t, 20*time.Second, cfg.Server.ShutdownTimeout)
	assert.Equal(t, 60*time.Second, cfg.Server.RequestTimeout)
	assert.Equal(t, 20*time.Second, cfg.Server.IdleTimeout)
	assert.Equal(t, 20*time.Second, cfg.Server.WaitTimeout)
	assert.Equal(t, logger.LogLevelDebug, cfg.Logger.Level)
	assert.Equal(t, logger.LogFormatText, cfg.Logger.Format)
}

func TestLoad_EnvOverridesDefaults(t *testing.T) {
	configPath := filepath.Join("testdata", "minimal_config.yaml")

	t.Setenv("GATE_SERVER_PORT", "3000")
	t.Setenv("GATE_LOGGER_LEVEL", "warn")

	cfg, err := Load(t.Context(), configPath)

	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, 3000, cfg.Server.Port)
	assert.Equal(t, logger.LogLevelWarn, cfg.Logger.Level)
}

func TestLoad_EnvOverridesYAML(t *testing.T) {
	configPath := filepath.Join("testdata", "valid_app_config.yaml")

	t.Setenv("GATE_SERVER_PORT", "9090")
	t.Setenv("GATE_OIDC_AUDIENCE", "gate-staging")

	cfg, err := Load(t.Context(), configPath)

	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, 9090, cfg.Server.Port)
	assert.Equal(t, "gate-staging", cfg.OIDC.Audience)
}

func TestLoad_EnvOnlyField(t *testing.T) {
	configPath := filepath.Join("testdata", "valid_app_config.yaml")

	t.Setenv("GATE_AWS_REGION", "us-west-2")

	cfg, err := Load(t.Context(), configPath)

	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "us-west-2", cfg.AWSRegion)
}

func TestLoad_NonExistentFile(t *testing.T) {
	cfg, err := Load(t.Context(), "/nonexistent/config.yaml")

	require.Error(t, err)
	assert.Nil(t, cfg)
}

func TestLoad_InvalidYAML(t *testing.T) {
	configPath := filepath.Join("testdata", "invalid_syntax.yaml")

	cfg, err := Load(t.Context(), configPath)

	require.Error(t, err)
	assert.Nil(t, cfg)
}

func TestLoad_ValidationFailure(t *testing.T) {
	configPath := filepath.Join("testdata", "invalid_port.yaml")

	cfg, err := Load(t.Context(), configPath)

	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidPort)
	assert.Nil(t, cfg)
}

func TestLoad_ValidationFailure_MissingRequired(t *testing.T) {
	configPath := filepath.Join("testdata", "empty_config.yaml")

	cfg, err := Load(t.Context(), configPath)

	require.Error(t, err)
	assert.Nil(t, cfg)
}

func TestLoad_NoConfigFile(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	cfg, loadErr := Load(t.Context(), "")

	require.Error(t, loadErr)
	assert.Nil(t, cfg)
}

func TestLoad_SetsCurrent(t *testing.T) {
	configPath := filepath.Join("testdata", "valid_app_config.yaml")

	cfg, err := Load(t.Context(), configPath)

	require.NoError(t, err)
	assert.Same(t, cfg, GetCurrent())
}

func TestLoggerConfig_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		cfg   LoggerConfig
		fails error
	}{
		{name: "valid", cfg: LoggerConfig{Level: logger.LogLevelInfo, Format: logger.LogFormatJSON}},
		{name: "empty_defaults_ok", cfg: LoggerConfig{}},
		{name: "invalid_level", cfg: LoggerConfig{Level: "invalid"}, fails: ErrInvalidLogLevel},
		{name: "invalid_format", cfg: LoggerConfig{Level: logger.LogLevelInfo, Format: "xml"}, fails: ErrInvalidLogFormat},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.cfg.Validate()
			if tt.fails != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.fails)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSelectorConfig_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		cfg   SelectorConfig
		fails error
	}{
		{name: "empty_type", cfg: SelectorConfig{}},
		{name: "memory", cfg: SelectorConfig{Type: SelectorStoreTypeMemory}},
		{name: "redis", cfg: SelectorConfig{Type: SelectorStoreTypeRedis, Redis: &RedisConfig{Address: "localhost:6379"}}},
		{name: "redis_nil_config", cfg: SelectorConfig{Type: SelectorStoreTypeRedis}, fails: ErrInvalidRedisConfig},
		{name: "dynamodb", cfg: SelectorConfig{Type: SelectorStoreTypeDynamoDB, DynamoDB: &SelectorDynamoDBConfig{TableName: "rate_limits"}}},
		{name: "dynamodb_nil_config", cfg: SelectorConfig{Type: SelectorStoreTypeDynamoDB}, fails: ErrInvalidSelectorDynamoDBConfig},
		{
			name:  "dynamodb_empty_table",
			cfg:   SelectorConfig{Type: SelectorStoreTypeDynamoDB, DynamoDB: &SelectorDynamoDBConfig{}},
			fails: ErrInvalidSelectorDynamoDBTable,
		},
		{name: "unknown_type", cfg: SelectorConfig{Type: "unknown"}, fails: ErrInvalidSelectorStoreType},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.cfg.Validate()
			if tt.fails != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.fails)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestPolicyConfig_Validate(t *testing.T) {
	t.Parallel()

	valid := PolicyConfig{
		Version:         "1.0",
		TrustPolicyPath: ".github/gate/trust-policy.yaml",
		DefaultTokenTTL: 900,
		MaxTokenTTL:     3600,
		Providers:       []ProviderConfig{validProviderConfig()},
		MaxPermissions:  map[string]string{"contents": "read"},
	}

	tests := []struct {
		name   string
		modify func(*PolicyConfig)
		fails  error
	}{
		{name: "valid"},
		{name: "empty_version", modify: func(c *PolicyConfig) { c.Version = "" }, fails: ErrInvalidPolicyVersion},
		{name: "invalid_version", modify: func(c *PolicyConfig) { c.Version = "2.0" }, fails: ErrInvalidPolicyVersion},
		{name: "empty_trust_path", modify: func(c *PolicyConfig) { c.TrustPolicyPath = "" }, fails: ErrInvalidTrustPolicyPath},
		{name: "default_ttl_zero", modify: func(c *PolicyConfig) { c.DefaultTokenTTL = 0 }, fails: ErrInvalidDefaultTokenTTL},
		{name: "max_ttl_zero", modify: func(c *PolicyConfig) { c.MaxTokenTTL = 0 }, fails: ErrInvalidMaxTokenTTL},
		{
			name:   "default_exceeds_max",
			modify: func(c *PolicyConfig) { c.DefaultTokenTTL = 7200; c.MaxTokenTTL = 3600 },
			fails:  ErrDefaultTTLExceedsMax,
		},
		{name: "invalid_provider", modify: func(c *PolicyConfig) { c.Providers = []ProviderConfig{{Issuer: ""}} }, fails: ErrInvalidProviderIssuer},
		{
			name:   "invalid_max_permissions",
			modify: func(c *PolicyConfig) { c.MaxPermissions = map[string]string{"contents": "admin"} },
			fails:  ErrInvalidPermissionLevel,
		},
		{name: "none_is_valid", modify: func(c *PolicyConfig) { c.MaxPermissions = map[string]string{"actions": "none"} }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := valid
			if tt.modify != nil {
				tt.modify(&cfg)
			}
			err := cfg.Validate()
			if tt.fails != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.fails)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestProviderConfig_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		modify func(*ProviderConfig)
		fails  error
	}{
		{name: "valid"},
		{name: "empty_issuer", modify: func(c *ProviderConfig) { c.Issuer = "" }, fails: ErrInvalidProviderIssuer},
		{name: "empty_name", modify: func(c *ProviderConfig) { c.Name = "" }, fails: ErrInvalidProviderName},
		{
			name: "invalid_time_restriction",
			modify: func(c *ProviderConfig) {
				c.TimeRestrictions = &TimeRestriction{AllowedDays: []AllowedDays{"NotADay"}}
			},
			fails: ErrInvalidAllowedDays,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := validProviderConfig()
			if tt.modify != nil {
				tt.modify(&cfg)
			}
			err := cfg.Validate()
			if tt.fails != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.fails)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateMaxPermissions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		permissions map[string]string
		fails       error
	}{
		{name: "all_read", permissions: map[string]string{"contents": "read", "issues": "read", "packages": "read"}},
		{name: "empty_ok", permissions: map[string]string{}},
		{name: "nil_ok", permissions: nil},
		{name: "none_valid", permissions: map[string]string{"actions": "none"}},
		{name: "write_valid", permissions: map[string]string{"contents": "write"}},
		{name: "invalid_level", permissions: map[string]string{"contents": "admin"}, fails: ErrInvalidPermissionLevel},
		{name: "any_perm_name", permissions: map[string]string{"deployments": "read", "security_events": "write", "statuses": "none"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validatePermissions(tt.permissions)
			if tt.fails != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.fails)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestHourRange_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		cfg   HourRange
		fails error
	}{
		{name: "valid", cfg: HourRange{Start: 9, End: 17}},
		{name: "start_negative", cfg: HourRange{Start: -1, End: 17}, fails: ErrInvalidStartHour},
		{name: "start_exceeds_23", cfg: HourRange{Start: 24, End: 17}, fails: ErrInvalidStartHour},
		{name: "end_negative", cfg: HourRange{Start: 9, End: -1}, fails: ErrInvalidEndHour},
		{name: "end_exceeds_23", cfg: HourRange{Start: 9, End: 24}, fails: ErrInvalidEndHour},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.cfg.Validate()
			if tt.fails != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.fails)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestTimeRestriction_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		cfg   TimeRestriction
		fails error
	}{
		{name: "valid", cfg: TimeRestriction{AllowedDays: []AllowedDays{AllowedDaysMonday}, AllowedHours: &HourRange{Start: 9, End: 17}}},
		{name: "nil_hours", cfg: TimeRestriction{AllowedDays: []AllowedDays{AllowedDaysTuesday}}},
		{name: "invalid_day", cfg: TimeRestriction{AllowedDays: []AllowedDays{"NotADay"}}, fails: ErrInvalidAllowedDays},
		{name: "invalid_hours", cfg: TimeRestriction{AllowedHours: &HourRange{Start: -1, End: 17}}, fails: ErrInvalidStartHour},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.cfg.Validate()
			if tt.fails != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.fails)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestServerConfig_Validate(t *testing.T) {
	t.Parallel()

	valid := ServerConfig{
		Port:            8080,
		ReadTimeout:     30 * time.Second,
		WriteTimeout:    30 * time.Second,
		ShutdownTimeout: 10 * time.Second,
		RequestTimeout:  30 * time.Second,
		IdleTimeout:     10 * time.Second,
		WaitTimeout:     10 * time.Second,
	}

	tests := []struct {
		name   string
		modify func(*ServerConfig)
		fails  error
	}{
		{name: "valid"},
		{name: "port_min", modify: func(c *ServerConfig) { c.Port = portMin }},
		{name: "port_max", modify: func(c *ServerConfig) { c.Port = portMax }},
		{name: "port_zero", modify: func(c *ServerConfig) { c.Port = 0 }, fails: ErrInvalidPort},
		{name: "port_exceeds_max", modify: func(c *ServerConfig) { c.Port = portMax + 1 }, fails: ErrInvalidPort},
		{name: "read_timeout_zero", modify: func(c *ServerConfig) { c.ReadTimeout = 0 }, fails: ErrInvalidReadTimeout},
		{name: "write_timeout_zero", modify: func(c *ServerConfig) { c.WriteTimeout = 0 }, fails: ErrInvalidWriteTimeout},
		{name: "shutdown_timeout_zero", modify: func(c *ServerConfig) { c.ShutdownTimeout = 0 }, fails: ErrInvalidShutdownTimeout},
		{name: "request_timeout_zero", modify: func(c *ServerConfig) { c.RequestTimeout = 0 }, fails: ErrInvalidRequestTimeout},
		{name: "idle_timeout_zero", modify: func(c *ServerConfig) { c.IdleTimeout = 0 }, fails: ErrInvalidIdleTimeout},
		{name: "wait_timeout_zero", modify: func(c *ServerConfig) { c.WaitTimeout = 0 }, fails: ErrInvalidWaitTimeout},
		{name: "tls_both_set", modify: func(c *ServerConfig) { c.TLS = TLSConfig{CertFilePath: "/cert.pem", KeyFilePath: "/key.pem"} }},
		{name: "tls_empty", modify: func(c *ServerConfig) {}},
		{name: "tls_cert_only", modify: func(c *ServerConfig) { c.TLS = TLSConfig{CertFilePath: "/cert.pem"} }, fails: ErrInvalidTLSConfig},
		{name: "tls_key_only", modify: func(c *ServerConfig) { c.TLS = TLSConfig{KeyFilePath: "/key.pem"} }, fails: ErrInvalidTLSConfig},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := valid
			if tt.modify != nil {
				tt.modify(&cfg)
			}
			err := cfg.Validate()
			if tt.fails != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.fails)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestServerConfig_GetAddr(t *testing.T) {
	t.Parallel()

	cfg := ServerConfig{Port: 9090}
	assert.Equal(t, ":9090", cfg.GetAddr())
}

func TestTLSConfig_Enabled(t *testing.T) {
	t.Parallel()

	assert.True(t, (&TLSConfig{CertFilePath: "/cert.pem", KeyFilePath: "/key.pem"}).Enabled())
	assert.False(t, (&TLSConfig{}).Enabled())
	assert.False(t, (&TLSConfig{CertFilePath: "/cert.pem"}).Enabled())
}

func TestTLSConfig_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		cfg   TLSConfig
		fails error
	}{
		{name: "both_set", cfg: TLSConfig{CertFilePath: "/cert.pem", KeyFilePath: "/key.pem"}},
		{name: "neither_set", cfg: TLSConfig{}},
		{name: "cert_only", cfg: TLSConfig{CertFilePath: "/cert.pem"}, fails: ErrInvalidTLSConfig},
		{name: "key_only", cfg: TLSConfig{KeyFilePath: "/key.pem"}, fails: ErrInvalidTLSConfig},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.cfg.Validate()
			if tt.fails != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.fails)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestOriginConfig_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		cfg   OriginConfig
		fails error
	}{
		{name: "disabled", cfg: OriginConfig{Enabled: false}},
		{name: "enabled_valid", cfg: OriginConfig{Enabled: true, HeaderName: "X-Origin", HeaderValue: "gate"}},
		{name: "missing_header_name", cfg: OriginConfig{Enabled: true, HeaderValue: "gate"}, fails: ErrInvalidOriginHeaderName},
		{name: "missing_header_value", cfg: OriginConfig{Enabled: true, HeaderName: "X-Origin"}, fails: ErrInvalidOriginHeaderValue},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.cfg.Validate()
			if tt.fails != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.fails)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestFIPSConfig_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		cfg   FIPSConfig
		fails error
	}{
		{name: "enabled_on", cfg: FIPSConfig{Enabled: true, Mode: FIPSModeOn}},
		{name: "enabled_only", cfg: FIPSConfig{Enabled: true, Mode: FIPSModeOnly}},
		{name: "disabled", cfg: FIPSConfig{Enabled: false}},
		{name: "invalid_mode", cfg: FIPSConfig{Enabled: true, Mode: "off"}, fails: ErrInvalidFIPSMode},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.cfg.Validate()
			if tt.fails != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.fails)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestGitHubAppConfig_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		modify func(*GitHubAppConfig)
		fails  error
	}{
		{name: "valid"},
		{name: "missing_client_id", modify: func(c *GitHubAppConfig) { c.ClientID = "" }, fails: ErrInvalidGithubAppClientID},
		{name: "missing_private_key_path", modify: func(c *GitHubAppConfig) { c.PrivateKeyPath = "" }, fails: ErrInvalidGithubAppPrivateKeyPath},
		{name: "missing_organization", modify: func(c *GitHubAppConfig) { c.Organization = "" }, fails: ErrInvalidGithubAppOrganization},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := validGitHubAppConfig()
			if tt.modify != nil {
				tt.modify(&cfg)
			}
			err := cfg.Validate()
			if tt.fails != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.fails)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateGitHubApps(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		apps  []GitHubAppConfig
		fails error
	}{
		{name: "empty_apps", apps: nil},
		{name: "valid_app", apps: []GitHubAppConfig{validGitHubAppConfig()}},
		{
			name:  "invalid_app",
			apps:  []GitHubAppConfig{validGitHubAppConfig(), {ClientID: ""}},
			fails: ErrInvalidGithubAppClientID,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateGitHubApps(tt.apps)
			if tt.fails != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.fails)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
