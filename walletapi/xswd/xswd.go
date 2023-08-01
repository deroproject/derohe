package xswd

import (
	"context"
	"log"
	"net/http"
	"sync"

	"github.com/creachadair/jrpc2/handler"
	"github.com/deroproject/derohe/globals"
	"github.com/deroproject/derohe/walletapi"
	"github.com/go-logr/logr"
	"github.com/gorilla/websocket"

	rpcserver "github.com/deroproject/derohe/walletapi/rpcserver"
)

type ApplicationData struct {
	id          []byte
	name        string
	description string
	url         string
	permissions map[string]Permission
	signature   []byte
}

type Permission int

const (
	Ask Permission = iota
	Allow
	Deny
	AlwaysAllow
	AlwaysDeny
)

func (perm Permission) IsPositive() bool {
	return perm == Allow || perm == AlwaysAllow
}

type RPCRequest struct {
	// Method to be called
	Method string
	// its parameters
	Params interface{}
}

type XSWD struct {
	// The websocket connected to and its app data
	applications map[*websocket.Conn]ApplicationData
	// function to request access of a dApp to wallet
	appHandler func(*ApplicationData) Permission
	// function to request the permission
	requestHandler func(*ApplicationData, RPCRequest) Permission
	server         *http.Server
	logger         logr.Logger
	ctx            rpcserver.WalletContext
	rpcHandler     handler.Map
	// mutex for applications map
	sync.Mutex
}

func NewXSWDServer(wallet *walletapi.Wallet_Disk, appHandler func(*ApplicationData) Permission, requestHandler func(*ApplicationData, RPCRequest) Permission) *XSWD {
	mux := http.NewServeMux()
	server := &http.Server{Addr: ":44326", Handler: mux}
	server.Shutdown(context.Background())

	logger := globals.Logger.WithName("XSWD")
	ctx := rpcserver.NewWalletContext(logger, wallet)
	var rpcHandler handler.Map
	xswd := &XSWD{
		applications:   make(map[*websocket.Conn]ApplicationData),
		appHandler:     appHandler,
		requestHandler: requestHandler,
		logger:         logger,
		server:         server,
		ctx:            ctx,
		rpcHandler:     rpcHandler,
	}

	mux.HandleFunc("/xswd", xswd.handleWebSocket)
	xswd.server.ListenAndServe()

	return xswd
}

func (x *XSWD) Stop() {
	x.server.Shutdown(context.Background())
}

func (x *XSWD) addApplication(conn *websocket.Conn, app ApplicationData) bool {
	x.Lock()
	defer x.Unlock()

	if !x.appHandler(&app).IsPositive() {
		return false
	}

	// Sanity check
	{
		if app.id == nil || len(app.id) != 32 {
			return false
		}

		if len(app.name) == 0 || len(app.name) > 255 {
			return false
		}

		if len(app.description) == 0 || len(app.description) > 255 {
			return false
		}

		// URL can be optional
		if len(app.url) > 255 {
			return false
		}

		// Signature can be optional but permissions can't exist without signature
		if app.permissions != nil {
			if (len(app.permissions) > 0 && len(app.signature) != 64) || len(app.permissions) > 255 {
				return false
			}
		}

		// Signature can be optional but verify its len
		if len(app.signature) > 0 && len(app.signature) != 64 {
			return false
		}

		// TODO verify signature
	}

	x.applications[conn] = app

	return true
}

func (x *XSWD) removeApplication(conn *websocket.Conn) {
	x.Lock()
	defer x.Unlock()

	delete(x.applications, conn)
}

func (x *XSWD) handleMessage(app ApplicationData, request RPCRequest) {
	if x.requestPermission(app, request) {

	} else {
		// TODO send error
	}
}

func (x *XSWD) requestPermission(app ApplicationData, request RPCRequest) bool {
	x.Lock()
	defer x.Unlock()

	perm, found := app.permissions[request.Method]
	if !found {
		perm = x.requestHandler(&app, request)
		if perm == AlwaysDeny || perm == AlwaysAllow {
			app.permissions[request.Method] = perm
		}
	}

	return perm.IsPositive()
}

func (x *XSWD) readMessageFromSession(conn *websocket.Conn) {
	defer x.removeApplication(conn)

	var request RPCRequest
	for {
		if err := conn.ReadJSON(&request); err != nil {
			log.Println("Error while reading JSON:", err)
			return
		}

		app, found := x.applications[conn]
		if !found {
			log.Println("Application not found")
			return
		}

		x.handleMessage(app, request)
	}
}

func (x *XSWD) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Accept from any origin
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket upgrade error:", err)
		return
	}
	defer conn.Close()

	var app_data ApplicationData
	if err := conn.ReadJSON(&app_data); err != nil {
		log.Println("Error while reading app_data:", err)
		return
	}

	if x.addApplication(conn, app_data) {
		// Application was successfully accepted, send response
		// if err := conn.WriteJSON(response); err != nil {
		// 	log.Println("Error while writing JSON:", err)
		// 	return
		// }
		go x.readMessageFromSession(conn)
	} else {
		// TODO Application was rejected, send error
		conn.Close()
	}
}
