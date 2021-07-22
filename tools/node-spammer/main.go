package main

import (
	"fmt"
	"time"

	"github.com/iotaledger/goshimmer/client"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/identity"
)

var (
	apiURL = "http://localhost:8080"
)

func main() {
	nodes := make(map[string]*identity.LocalIdentity)

	nodeIdentity := identity.GenerateLocalIdentity()
	nodes[nodeIdentity.ID().String()] = nodeIdentity

	keyPair := ed25519.GenerateKeyPair()

	// fmt.Println(nodeIdentity.PublicKey(), base58.Encode(nodeIdentity.ID().Bytes()))
	fmt.Println(keyPair.PrivateKey, keyPair.PublicKey)

	api := client.NewGoShimmerAPI(apiURL)

	for i := 0; i < 100; i++ {
		message, err := api.Data(nodeIdentity.ID().Bytes(), keyPair.PublicKey, keyPair.PrivateKey)
		fmt.Println(message, err)
		time.Sleep(1 * time.Second)
	}

}
