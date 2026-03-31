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

// Package utils provides cryptographic and general-purpose utility functions.
package utils

import (
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
)

const (
	// PEMTypeRSAPrivateKey is the PEM block type for PKCS#1 RSA private keys.
	PEMTypeRSAPrivateKey = "RSA PRIVATE KEY"
	// PEMTypePrivateKey is the PEM block type for PKCS#8 private keys.
	PEMTypePrivateKey = "PRIVATE KEY"
)

var (
	// ErrInvalidPrivateKey is returned when the PEM block cannot be
	// decoded or the key bytes are invalid.
	ErrInvalidPrivateKey = errors.New("failed to decode PEM block")
	// ErrNotRSAKey is returned when a PKCS#8 key is not an RSA private key.
	ErrNotRSAKey = errors.New("not an RSA private key")
	// ErrInvalidPEMType is returned when the PEM block type is not RSA PRIVATE KEY or PRIVATE KEY.
	ErrInvalidPEMType = errors.New("unexpected PEM block type")
)

// HashString returns the SHA-256 hex digest of a string, prefixed with "sha256:"
// to identify the algorithm used.
func HashString(s string) string {
	h := sha256.Sum256([]byte(s))
	return "sha256:" + hex.EncodeToString(h[:])
}

// ParseRSAPrivateKey decodes a PEM-encoded RSA private key.
// It supports both PKCS#1 (RSA PRIVATE KEY) and PKCS#8 (PRIVATE KEY) formats.
func ParseRSAPrivateKey(pemStr string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, ErrInvalidPrivateKey
	}

	if block.Type != PEMTypeRSAPrivateKey && block.Type != PEMTypePrivateKey {
		return nil, fmt.Errorf("%w: got %q, want %q or %q", ErrInvalidPEMType, block.Type, PEMTypeRSAPrivateKey, PEMTypePrivateKey)
	}

	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		parsed, pkcs8Err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if pkcs8Err != nil {
			return nil, fmt.Errorf("%w: %w", ErrInvalidPrivateKey, pkcs8Err)
		}
		var ok bool
		key, ok = parsed.(*rsa.PrivateKey)
		if !ok {
			return nil, ErrNotRSAKey
		}
	}

	return key, nil
}
