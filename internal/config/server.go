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
	"errors"
	"fmt"
	"time"
)

const (
	// KeyServerPort is the Viper key for the server listen port.
	KeyServerPort = "server.port"
	// KeyServerReadTimeout is the Viper key for the HTTP read timeout.
	KeyServerReadTimeout = "server.read_timeout"
	// KeyServerWriteTimeout is the Viper key for the HTTP write timeout.
	KeyServerWriteTimeout = "server.write_timeout"
	// KeyServerShutdownTimeout is the Viper key for the server shutdown timeout.
	KeyServerShutdownTimeout = "server.shutdown_timeout"
	// KeyServerRequestTimeout is the Viper key for the per-request timeout.
	KeyServerRequestTimeout = "server.request_timeout"
	// KeyServerIdleTimeout is the Viper key for the HTTP idle (keep-alive) timeout.
	KeyServerIdleTimeout = "server.idle_timeout"
	// KeyServerWaitTimeout is the Viper key for the graceful shutdown wait timeout.
	KeyServerWaitTimeout = "server.wait_timeout"
	// KeyServerTLSCertFilePath is the Viper key for the TLS certificate file path.
	KeyServerTLSCertFilePath = "server.tls.cert_file_path"
	// KeyServerTLSKeyFilePath is the Viper key for the TLS private key file path.
	KeyServerTLSKeyFilePath = "server.tls.key_file_path"

	// DefaultServerPort is the default HTTP listen port.
	DefaultServerPort = 8080
	// DefaultServerReadTimeout is the default HTTP read timeout.
	DefaultServerReadTimeout = 30 * time.Second
	// DefaultServerWriteTimeout is the default HTTP write timeout.
	DefaultServerWriteTimeout = 30 * time.Second
	// DefaultServerShutdownTimeout is the default server shutdown timeout.
	DefaultServerShutdownTimeout = 10 * time.Second
	// DefaultServerRequestTimeout is the default per-request timeout.
	DefaultServerRequestTimeout = 30 * time.Second
	// DefaultServerIdleTimeout is the default HTTP idle timeout.
	DefaultServerIdleTimeout = 10 * time.Second
	// DefaultServerWaitTimeout is the default graceful shutdown wait duration.
	DefaultServerWaitTimeout = 10 * time.Second

	portMin = 1
	portMax = 65535
)

var (
	// ErrInvalidPort is returned when the server port is outside the valid range (1-65535).
	ErrInvalidPort = errors.New("invalid port")
	// ErrInvalidReadTimeout is returned when read_timeout is not positive.
	ErrInvalidReadTimeout = errors.New("invalid read timeout")
	// ErrInvalidWriteTimeout is returned when write_timeout is not positive.
	ErrInvalidWriteTimeout = errors.New("invalid write timeout")
	// ErrInvalidShutdownTimeout is returned when shutdown_timeout is not positive.
	ErrInvalidShutdownTimeout = errors.New("invalid shutdown timeout")
	// ErrInvalidRequestTimeout is returned when request_timeout is not positive.
	ErrInvalidRequestTimeout = errors.New("invalid request timeout")
	// ErrInvalidIdleTimeout is returned when idle_timeout is not positive.
	ErrInvalidIdleTimeout = errors.New("invalid idle timeout")
	// ErrInvalidWaitTimeout is returned when wait_timeout is not positive.
	ErrInvalidWaitTimeout = errors.New("invalid wait timeout")
	// ErrInvalidTLSConfig is returned when only one of cert_file_path or key_file_path is set.
	ErrInvalidTLSConfig = errors.New("both cert_file_path and key_file_path must be set for TLS")
)

// TLSConfig holds TLS certificate paths for the server.
type TLSConfig struct {
	CertFilePath string `mapstructure:"cert_file_path"`
	KeyFilePath  string `mapstructure:"key_file_path"`
}

// Validate validates the TLS configuration.
func (t *TLSConfig) Validate() error {
	if (t.CertFilePath == "") != (t.KeyFilePath == "") {
		return ErrInvalidTLSConfig
	}
	return nil
}

// Enabled returns true if TLS is configured with both cert and key files.
func (t *TLSConfig) Enabled() bool {
	return t.CertFilePath != "" && t.KeyFilePath != ""
}

// ServerConfig holds HTTP server and TLS settings.
type ServerConfig struct {
	Port            int           `mapstructure:"port"`
	ReadTimeout     time.Duration `mapstructure:"read_timeout"`
	WriteTimeout    time.Duration `mapstructure:"write_timeout"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
	RequestTimeout  time.Duration `mapstructure:"request_timeout"`
	IdleTimeout     time.Duration `mapstructure:"idle_timeout"`
	WaitTimeout     time.Duration `mapstructure:"wait_timeout"`
	TLS             TLSConfig     `mapstructure:"tls"`
}

// Validate validates the server configuration.
func (s *ServerConfig) Validate() error {
	if s.Port < portMin || s.Port > portMax {
		return ErrInvalidPort
	}
	if s.ReadTimeout <= 0 {
		return ErrInvalidReadTimeout
	}
	if s.WriteTimeout <= 0 {
		return ErrInvalidWriteTimeout
	}
	if s.ShutdownTimeout <= 0 {
		return ErrInvalidShutdownTimeout
	}
	if s.RequestTimeout <= 0 {
		return ErrInvalidRequestTimeout
	}
	if s.IdleTimeout <= 0 {
		return ErrInvalidIdleTimeout
	}
	if s.WaitTimeout <= 0 {
		return ErrInvalidWaitTimeout
	}
	return s.TLS.Validate()
}

// GetAddr returns the server listen address in host:port format.
func (s *ServerConfig) GetAddr() string {
	return fmt.Sprintf(":%d", s.Port)
}
