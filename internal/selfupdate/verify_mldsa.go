// Package selfupdate implements trvl's hybrid quantum-safe self-update
// pipeline.
//
// Threat model. The auto-update mechanism downloads release archives from
// GitHub and replaces the running trvl binary. A binary that auto-replaces
// itself from the network is a high-value supply-chain target, so
// verification is layered:
//
//  1. SHA-256 of the archive against the release's checksums.txt — defends
//     against random network corruption and CDN tampering.
//  2. cosign keyless signature (ECDSA-P256 via Sigstore + GitHub OIDC) —
//     defends against compromised release uploads under classical
//     cryptanalysis.
//  3. ML-DSA-65 signature (NIST FIPS 204) over the SHA-256 digest, verified
//     against the trust-anchor pubkey embedded at build time — defends
//     against future quantum-cryptanalysis of layer 2.
//
// All three layers must pass before the binary swap happens; partial pass
// aborts the update and logs to ~/.trvl/autoupdate.log.
//
// Defense in depth: even if Sigstore/Fulcio is compromised, the ML-DSA
// layer holds. Even if our long-lived ML-DSA private key leaks, the cosign
// keyless layer holds (an attacker would need to compromise both).
//
// This file implements the ML-DSA-65 verifier and the embedded trust
// anchor. The cosign verifier and the SHA-256 checker live in sibling
// files.
package selfupdate

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"io"
	"strings"

	"github.com/cloudflare/circl/sign/mldsa/mldsa65"
)

// trustAnchorMLDSA65V1 is the ML-DSA-65 public key that signs every trvl
// release archive. Generated 2026-05-02 via cmd/keygen-mldsa, stored
// alongside this package at internal/selfupdate/keys/mldsa65_v1.pubkey.hex
// (3904 hex chars + trailing newline). The matching private key lives in
// the GitHub Secret TRVL_MLDSA_PRIVKEY_V1 with a 1Password backup;
// rotation procedure in docs/security/release-signing.md.
//
//go:embed keys/mldsa65_v1.pubkey.hex
var trustAnchorMLDSA65V1Hex string

// MLDSA65PubkeyFingerprint returns the first 16 hex chars of the embedded
// trust-anchor pubkey. Used in update logs and the `trvl version --verify`
// path so users can spot-check the trust anchor matches what they expect.
func MLDSA65PubkeyFingerprint() string {
	hex := strings.TrimSpace(trustAnchorMLDSA65V1Hex)
	if len(hex) < 16 {
		return ""
	}
	return hex[:16]
}

// loadTrustAnchorMLDSA65 parses the embedded pubkey hex into an ML-DSA-65
// public key suitable for Verify. Returns an error only if the embedded
// data is malformed — which would be a build-time error, not runtime.
func loadTrustAnchorMLDSA65() (*mldsa65.PublicKey, error) {
	hexStr := strings.TrimSpace(trustAnchorMLDSA65V1Hex)
	pkBytes, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, fmt.Errorf("trust anchor: decode hex: %w", err)
	}
	if len(pkBytes) != mldsa65.PublicKeySize {
		return nil, fmt.Errorf("trust anchor: wrong length: got %d, want %d",
			len(pkBytes), mldsa65.PublicKeySize)
	}
	var pk mldsa65.PublicKey
	if err := pk.UnmarshalBinary(pkBytes); err != nil {
		return nil, fmt.Errorf("trust anchor: unmarshal: %w", err)
	}
	return &pk, nil
}

// VerifyMLDSA65 verifies an ML-DSA-65 detached signature over the SHA-256
// digest of artifact, using the embedded trust-anchor public key.
//
// The signing convention (set by cmd/sign-mldsa) is:
//
//	digest = sha256(artifact_bytes)
//	sig    = ML-DSA-65.Sign(privkey, digest)
//
// So verification reads the artifact streamingly, hashes it once, then
// feeds (digest, sig) to mldsa65.Verify. Returns nil on a valid signature
// produced by the holder of the corresponding private key, or a non-nil
// error explaining what failed.
//
// The error path explicitly distinguishes "I/O failure reading the
// artifact" (transient, retryable) from "signature did not verify" (the
// archive is forged or corrupted; never retry — abort the update).
func VerifyMLDSA65(artifact io.Reader, sig []byte) error {
	if len(sig) != mldsa65.SignatureSize {
		return fmt.Errorf("mldsa65 verify: signature wrong length: got %d, want %d",
			len(sig), mldsa65.SignatureSize)
	}
	pk, err := loadTrustAnchorMLDSA65()
	if err != nil {
		return fmt.Errorf("mldsa65 verify: %w", err)
	}
	h := sha256.New()
	if _, err := io.Copy(h, artifact); err != nil {
		return fmt.Errorf("mldsa65 verify: read artifact: %w", err)
	}
	digest := h.Sum(nil)
	if !mldsa65.Verify(pk, digest, nil, sig) {
		return errSignatureMismatch
	}
	return nil
}

// errSignatureMismatch is returned when the ML-DSA-65 signature does not
// verify against the embedded trust anchor for the given artifact. This
// is fatal: the archive has been tampered with or the wrong signature
// file was fetched. Auto-update callers must abort and never retry on
// this error.
var errSignatureMismatch = fmt.Errorf(
	"mldsa65 verify: signature did not verify against trust anchor — refusing to apply update")
