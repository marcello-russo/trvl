package selfupdate

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"strings"
	"testing"

	"github.com/cloudflare/circl/sign/mldsa/mldsa65"
)

// TestTrustAnchorEmbedded verifies that the pubkey hex committed to the
// repo at cmd/trvl/keys/mldsa65_v1.pubkey.hex parses successfully into a
// usable ML-DSA-65 public key, and that the fingerprint matches what we
// expect (first 16 hex chars of the trust-anchor pubkey).
//
// If this test fails, the trust anchor in the repo is corrupted — every
// trvl binary built from this commit would refuse all auto-updates. This
// is intentionally a hard build-time check.
func TestTrustAnchorEmbedded(t *testing.T) {
	pk, err := loadTrustAnchorMLDSA65()
	if err != nil {
		t.Fatalf("loadTrustAnchorMLDSA65: %v", err)
	}
	if pk == nil {
		t.Fatal("loadTrustAnchorMLDSA65 returned nil pubkey without error")
	}

	// The hex constant should be exactly 3904 hex chars (1952 bytes binary)
	// plus a trailing newline.
	hex := strings.TrimSpace(trustAnchorMLDSA65V1Hex)
	if len(hex) != 2*mldsa65.PublicKeySize {
		t.Errorf("trust anchor hex length = %d, want %d (2 × pubkey-bytes)",
			len(hex), 2*mldsa65.PublicKeySize)
	}

	fp := MLDSA65PubkeyFingerprint()
	if len(fp) != 16 {
		t.Errorf("fingerprint length = %d, want 16", len(fp))
	}
	// We don't pin the exact fingerprint value here so this test still
	// passes after a key rotation; the build-time embed already enforces
	// the bytes match what's in the repo.
}

// TestVerifyMLDSA65_RoundtripWithAdhocKey checks the verifier against a
// signature produced by a freshly-generated keypair, using the same
// digest convention as cmd/sign-mldsa (SHA-256 of the artifact). This
// test does NOT use the embedded trust anchor — it uses a fake one — so
// it isolates the signing-and-verifying logic from the trust-anchor
// loading path tested separately in TestTrustAnchorEmbedded.
func TestVerifyMLDSA65_RoundtripWithAdhocKey(t *testing.T) {
	pk, sk, err := mldsa65.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("mldsa65.GenerateKey: %v", err)
	}

	artifactBytes := []byte("pretend this is a 12 MB tar.gz of trvl release binaries\n")
	digest := sha256.Sum256(artifactBytes)
	sig := make([]byte, mldsa65.SignatureSize)
	if err := mldsa65.SignTo(sk, digest[:], nil, false, sig); err != nil {
		t.Fatalf("SignTo: %v", err)
	}
	if !mldsa65.Verify(pk, digest[:], nil, sig) {
		t.Fatal("baseline mldsa65.Verify failed — keypair is broken before we even test our wrapper")
	}

	// Now we exercise our wrapper with a swapped-in trust anchor for the
	// duration of this test. We do this by saving and restoring the
	// embedded hex value. Concurrent tests can't run because the package
	// var is shared, but no other test mutates it.
	origHex := trustAnchorMLDSA65V1Hex
	defer func() { trustAnchorMLDSA65V1Hex = origHex }()

	pkBytes, err := pk.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}
	trustAnchorMLDSA65V1Hex = hexEncode(pkBytes) + "\n"

	if err := VerifyMLDSA65(bytes.NewReader(artifactBytes), sig); err != nil {
		t.Fatalf("VerifyMLDSA65 (good signature): %v", err)
	}
}

// TestVerifyMLDSA65_RejectsTampered ensures a flipped bit in the artifact
// causes Verify to fail with errSignatureMismatch — *not* an I/O error.
// The auto-update path treats these two outcomes very differently:
// I/O = transient (retry next start), mismatch = malicious (abort
// permanently).
func TestVerifyMLDSA65_RejectsTampered(t *testing.T) {
	pk, sk, err := mldsa65.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	artifact := []byte("trvl-1.2.0-darwin-arm64.tar.gz contents")
	digest := sha256.Sum256(artifact)
	sig := make([]byte, mldsa65.SignatureSize)
	if err := mldsa65.SignTo(sk, digest[:], nil, false, sig); err != nil {
		t.Fatalf("SignTo: %v", err)
	}

	// Swap trust anchor to our adhoc pubkey.
	origHex := trustAnchorMLDSA65V1Hex
	defer func() { trustAnchorMLDSA65V1Hex = origHex }()
	pkBytes, _ := pk.MarshalBinary()
	trustAnchorMLDSA65V1Hex = hexEncode(pkBytes)

	// Flip a single byte in the artifact.
	tampered := make([]byte, len(artifact))
	copy(tampered, artifact)
	tampered[0] ^= 0x01

	err = VerifyMLDSA65(bytes.NewReader(tampered), sig)
	if err == nil {
		t.Fatal("VerifyMLDSA65 accepted tampered artifact — would let a forged update through")
	}
	if !errors.Is(err, errSignatureMismatch) {
		t.Errorf("got error %q, want errSignatureMismatch (auto-update path needs to distinguish I/O failure from forged binary)", err)
	}
}

// TestVerifyMLDSA65_RejectsWrongSize ensures a malformed / truncated
// signature is rejected without crashing the verifier.
func TestVerifyMLDSA65_RejectsWrongSize(t *testing.T) {
	tooShort := make([]byte, 10)
	err := VerifyMLDSA65(bytes.NewReader([]byte("anything")), tooShort)
	if err == nil {
		t.Fatal("VerifyMLDSA65 accepted truncated signature")
	}
	if !strings.Contains(err.Error(), "wrong length") {
		t.Errorf("wrong error: %v", err)
	}
}

// hexEncode is a tiny local helper to avoid an extra import in the test
// file proper.
func hexEncode(b []byte) string {
	const hexChars = "0123456789abcdef"
	out := make([]byte, len(b)*2)
	for i, v := range b {
		out[i*2] = hexChars[v>>4]
		out[i*2+1] = hexChars[v&0x0f]
	}
	return string(out)
}
