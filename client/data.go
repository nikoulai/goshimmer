package client

import (
	"net/http"

	"github.com/iotaledger/goshimmer/packages/jsonmodels"
	"github.com/iotaledger/hive.go/crypto/ed25519"
)

const (
	routeData = "data"
)

// Data sends the given data (payload) by creating a message in the backend.
func (api *GoShimmerAPI) Data(data []byte, publicKey ed25519.PublicKey, privateKey ed25519.PrivateKey) (string, error) {
	res := &jsonmodels.DataResponse{}
	request := &jsonmodels.DataRequest{
		Data:       data,
		PublicKey:  publicKey,
		PrivateKey: privateKey,
	}
	if err := api.do(http.MethodPost, routeData, request, res); err != nil {
		return "", err
	}

	return res.ID, nil
}
