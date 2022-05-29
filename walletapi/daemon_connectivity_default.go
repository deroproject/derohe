package walletapi

import (
	"net/http"

	"github.com/creachadair/jrpc2"
	"github.com/gorilla/websocket"
)

var netClient *http.Client

type Client struct {
	WS  *websocket.Conn
	RPC *jrpc2.Client
}

var rpc_client = &Client{}
