package ccvclient

import (
	"github.com/stellar/go-stellar-sdk/clients/rpcclient"
)

type Client struct {
	rpcClient *rpcclient.Client
}

func NewClient(rpcClient *rpcclient.Client) *Client {
	return &Client{rpcClient: rpcClient}
}
