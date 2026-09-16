// Command gen-dev-jwt is a LOCAL DEVELOPMENT ONLY tool. It signs a throwaway
// RS256 JWT with a private key you generate yourself (see the Zed task
// "Backend: generate dev RSA keypair" or run openssl directly), so you can
// call chat-history-proxy's protected endpoints without a real client
// auth backend.
//
// This is not part of the proxy service and must never be pointed at a real
// private key or used against a production deployment.
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func main() {
	keyPath := flag.String("key", ".dev/jwt_private.pem", "path to an RSA private key PEM (dev-only)")
	sub := flag.String("sub", "dev-user-1", "user id to put in the sub claim")
	ttl := flag.Duration("ttl", time.Hour, "token validity duration")
	flag.Parse()

	keyBytes, err := os.ReadFile(*keyPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gen-dev-jwt: reading %s: %v\n", *keyPath, err)
		os.Exit(1)
	}

	privateKey, err := jwt.ParseRSAPrivateKeyFromPEM(keyBytes)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gen-dev-jwt: parsing private key: %v\n", err)
		os.Exit(1)
	}

	claims := jwt.MapClaims{
		"sub": *sub,
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(*ttl).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signed, err := token.SignedString(privateKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gen-dev-jwt: signing: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(signed)
}
