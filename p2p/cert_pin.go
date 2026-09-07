// Copyright 2017-2021 DERO Project. All rights reserved.
// Use of this source code in any form is governed by RESEARCH license.
// license can be found in the LICENSE file.

package p2p

import (
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// certPinStore tracks TLS certificate fingerprints per peer ID.
// After the first handshake, we pin the cert. On reconnection,
// we verify it hasn't changed — preventing MITM attacks.
type certPinStore struct {
	mu           sync.RWMutex
	fingerprints map[uint64][32]byte // peer_id -> cert fingerprint (SHA-256 of DER)
}

var knownCerts = &certPinStore{
	fingerprints: make(map[uint64][32]byte),
}

// The P2P certificate is self-signed, so ordinary CA verification is not
// applicable. Both TLS endpoints use one process-local certificate and the
// handshake advertises its fingerprint; the peer-ID pin is checked separately.
var localTLSIdentity struct {
	sync.Once
	cert        tls.Certificate
	fingerprint [32]byte
}

func localTLSCertificate() tls.Certificate {
	localTLSIdentity.Do(func() {
		localTLSIdentity.cert = generate_random_tls_cert()
		if len(localTLSIdentity.cert.Certificate) > 0 {
			localTLSIdentity.fingerprint = sha256.Sum256(localTLSIdentity.cert.Certificate[0])
		}
	})
	return localTLSIdentity.cert
}

func localTLSFingerprint() [32]byte {
	localTLSCertificate()
	return localTLSIdentity.fingerprint
}

// ComputeCertFingerprint returns SHA-256 of the DER-encoded X.509 certificate.
func ComputeCertFingerprint(cert *x509.Certificate) [32]byte {
	return sha256.Sum256(cert.Raw)
}

// ComputeTLSCertFingerprint extracts the peer's certificate from a TLS connection
// state and computes its fingerprint. Returns zero hash if no peer cert available.
func ComputeTLSCertFingerprint(connState tls.ConnectionState) [32]byte {
	if len(connState.PeerCertificates) == 0 {
		return [32]byte{}
	}
	return ComputeCertFingerprint(connState.PeerCertificates[0])
}

// Pin stores a certificate fingerprint for a peer ID.
func (s *certPinStore) Pin(peerID uint64, fingerprint [32]byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fingerprints[peerID] = fingerprint
}

// Verify checks whether a peer's certificate fingerprint matches what we've seen before.
// Returns true if:
// - We have no previous record (first connection, trust on first use)
// - The fingerprint matches
// Returns false if the fingerprint has changed (potential MITM).
func (s *certPinStore) Verify(peerID uint64, fingerprint [32]byte) bool {
	if fingerprint == ([32]byte{}) {
		return false // no cert provided
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	known, exists := s.fingerprints[peerID]
	if !exists {
		return true // first connection, trust on first use
	}
	return subtle.ConstantTimeCompare(known[:], fingerprint[:]) == 1
}

// Get returns the stored fingerprint for a peer, if any.
func (s *certPinStore) Get(peerID uint64) ([32]byte, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	fp, ok := s.fingerprints[peerID]
	return fp, ok
}

// Save persists the cert pin store to disk as JSON.
func (s *certPinStore) Save(dir string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	type entry struct {
		PeerID      uint64 `json:"peer_id"`
		Fingerprint string `json:"fingerprint"`
	}
	var entries []entry
	for id, fp := range s.fingerprints {
		entries = append(entries, entry{
			PeerID:      id,
			Fingerprint: fmt.Sprintf("%x", fp[:]),
		})
	}

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(dir, "cert_pins.json")
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}

// Load restores the cert pin store from disk.
func (s *certPinStore) Load(dir string) error {
	path := filepath.Join(dir, "cert_pins.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // no file yet, start empty
		}
		return err
	}

	type entry struct {
		PeerID      uint64 `json:"peer_id"`
		Fingerprint string `json:"fingerprint"`
	}
	var entries []entry
	if err := json.Unmarshal(data, &entries); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range entries {
		var fp [32]byte
		b, err := hex.DecodeString(e.Fingerprint)
		if err != nil || len(b) != 32 {
			continue
		}
		copy(fp[:], b)
		s.fingerprints[e.PeerID] = fp
	}
	return nil
}

var ErrInvalidHex = fmt.Errorf("invalid hex string")
