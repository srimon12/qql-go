package server

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rsa"
	"encoding/base64"
	"fmt"
	"math/big"
)

func base64URLEncode(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(s)
}

func rsaPublicKeyFromJWKS(k jwksKey) (*rsa.PublicKey, error) {
	nBytes, err := base64URLEncode(k.N)
	if err != nil {
		return nil, fmt.Errorf("invalid RSA n: %w", err)
	}
	eBytes, err := base64URLEncode(k.E)
	if err != nil {
		return nil, fmt.Errorf("invalid RSA e: %w", err)
	}

	n := new(big.Int).SetBytes(nBytes)

	// e is typically small (65537), decode from big-endian bytes.
	e := 0
	for _, b := range eBytes {
		e = e<<8 + int(b)
	}
	if e == 0 {
		return nil, fmt.Errorf("invalid RSA exponent")
	}

	return &rsa.PublicKey{N: n, E: e}, nil
}

func ecPublicKeyFromJWKS(k jwksKey) (*ecdsa.PublicKey, error) {
	xBytes, err := base64URLEncode(k.X)
	if err != nil {
		return nil, fmt.Errorf("invalid EC x: %w", err)
	}
	yBytes, err := base64URLEncode(k.Y)
	if err != nil {
		return nil, fmt.Errorf("invalid EC y: %w", err)
	}

	var curve elliptic.Curve
	switch k.Crv {
	case "P-256":
		curve = elliptic.P256()
	case "P-384":
		curve = elliptic.P384()
	case "P-521":
		curve = elliptic.P521()
	default:
		return nil, fmt.Errorf("unsupported EC curve: %s", k.Crv)
	}

	return &ecdsa.PublicKey{
		Curve: curve,
		X:     new(big.Int).SetBytes(xBytes),
		Y:     new(big.Int).SetBytes(yBytes),
	}, nil
}

func ed25519PublicKeyFromJWKS(k jwksKey) (ed25519.PublicKey, error) {
	xBytes, err := base64URLEncode(k.X)
	if err != nil {
		return nil, fmt.Errorf("invalid OKP x: %w", err)
	}
	if len(xBytes) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid Ed25519 key size: %d", len(xBytes))
	}
	return ed25519.PublicKey(xBytes), nil
}
