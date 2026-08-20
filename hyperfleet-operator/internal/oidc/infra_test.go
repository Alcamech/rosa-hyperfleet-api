/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package oidc

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"testing"
)

func rsaKeyPEM(t *testing.T, bits int) []byte {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
}

func TestValidateRSAPrivateKey(t *testing.T) {
	t.Run("accepts a key at the minimum size", func(t *testing.T) {
		if err := ValidateRSAPrivateKey(rsaKeyPEM(t, minKeyBits)); err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	t.Run("rejects a key below the minimum size", func(t *testing.T) {
		if err := ValidateRSAPrivateKey(rsaKeyPEM(t, 1024)); err == nil {
			t.Error("expected an error for a key below the minimum size, got nil")
		}
	})
}
