package main

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/client"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/identity"
	"github.com/mr-tron/base58"
)

var (
	apiURL = "http://localhost:8080"
)

var (
	seeds = []string{
		"CDDzcUNbok6zyoF8zC8gpD2E2pjGdBEm2Lkpauc3PSGk",
		"7RcW1L4xfUXCyubnYxSeJ3XWMfhXyAJMBDppQUmQAo6z",
		"2j9tYwGkannQ92FPZ5uwn6eutcQaJDDvuEDFZNESGQxz",
		"AzZ4wGrPqgP5mbZLGQc9onKzsJ2NJvtjLQQ9Bkrins87",
		"BBew186Ms89jqaAyuVANuhkoR9wu37o1nZ36K5NztDze",
	}
)

func main() {
	nodes := make(map[string]*identity.LocalIdentity)
	api := client.NewGoShimmerAPI(apiURL)
	shutdown := make(chan struct{})
	var wg sync.WaitGroup
	for _, seed := range seeds {
		s, _ := base58.Decode(seed)
		pk := ed25519.PrivateKeyFromSeed(s[:])
		nodeIdentity := identity.NewLocalIdentity(pk.Public(), pk)
		fmt.Println(base58.Encode(nodeIdentity.ID().Bytes()))
		nodes[nodeIdentity.ID().String()] = nodeIdentity
		wg.Add(1)

		go func() {
			defer wg.Done()
			randomizedStart := rand.Intn(5000)
			time.Sleep(time.Duration(randomizedStart) * time.Millisecond)
			spam(api, pk, 5*time.Second, shutdown)
		}()
	}
	time.Sleep(2 * time.Minute)
	close(shutdown)
	wg.Wait()

	// for i := 0; i < 100; i++ {
	// 	message, err := api.Data(nodeIdentity.ID().Bytes(), keyPair.PublicKey, keyPair.PrivateKey)
	// 	fmt.Println(message, err)
	// 	time.Sleep(1 * time.Second)
	// }

}

func spam(api *client.GoShimmerAPI, pk ed25519.PrivateKey, rate time.Duration, shutdown chan struct{}) {
	ticker := time.NewTicker(rate)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			msgID, err := api.Data(pk.Public().Bytes(), pk.Public(), pk)
			fmt.Println(msgID, err)
		case <-shutdown:
			return
		}
	}
}
