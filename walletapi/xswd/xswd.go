package xswd

import (
	"context"
	"encoding/hex"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/code"
	"github.com/creachadair/jrpc2/handler"
	"github.com/deroproject/derohe/globals"
	"github.com/deroproject/derohe/walletapi"
	"github.com/deroproject/derohe/walletapi/rpcserver"
	"github.com/go-logr/logr"
	"github.com/gorilla/websocket"
)

type ApplicationData struct {
	Id          string                `json:"id"`
	Name        string                `json:"name"`
	Description string                `json:"description"`
	Url         string                `json:"url"`
	Permissions map[string]Permission `json:"permissions"`
	Signature   []byte                `json:"signature"`
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

type XSWD struct {
	// The websocket connected to and its app data
	applications map[*websocket.Conn]ApplicationData
	// function to request access of a dApp to wallet
	appHandler func(*ApplicationData) Permission
	// function to request the permission
	requestHandler func(*ApplicationData, jrpc2.Request) Permission
	server         *http.Server
	logger         logr.Logger
	context        *rpcserver.WalletContext
	wallet         *walletapi.Wallet_Disk
	rpcHandler     handler.Map
	// mutex for applications map
	sync.Mutex
}

func NewXSWDServer(wallet *walletapi.Wallet_Disk, appHandler func(*ApplicationData) Permission, requestHandler func(*ApplicationData, jrpc2.Request) Permission) *XSWD {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("XSWD server"))
	})

	server := &http.Server{Addr: ":44326", Handler: mux}

	logger := globals.Logger.WithName("XSWD")
	var rpcHandler handler.Map
	xswd := &XSWD{
		applications:   make(map[*websocket.Conn]ApplicationData),
		appHandler:     appHandler,
		requestHandler: requestHandler,
		logger:         logger,
		server:         server,
		context:        rpcserver.NewWalletContext(logger, wallet),
		wallet:         wallet,
		rpcHandler:     rpcHandler,
	}

	mux.HandleFunc("/xswd", xswd.handleWebSocket)
	logger.Info("Starting XSWD server", "addr", server.Addr)

	go func() {
		if err := xswd.server.ListenAndServe(); err != nil {
			logger.Error(err, "Error while starting XSWD server")
		}
	}()

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
		id := strings.TrimSpace(app.Id)
		if len(id) != 64 {
			return false
		}

		if _, err := hex.DecodeString(id); err != nil {
			return false
		}

		if len(app.Name) == 0 || len(app.Name) > 255 {
			return false
		}

		if len(app.Description) == 0 || len(app.Description) > 255 {
			return false
		}

		// URL can be optional
		if len(app.Url) > 255 {
			return false
		}

		// Signature can be optional but permissions can't exist without signature
		if app.Permissions != nil {
			if (len(app.Permissions) > 0 && len(app.Signature) != 64) || len(app.Permissions) > 255 {
				return false
			}
		}

		// Signature can be optional but verify its len
		if len(app.Signature) > 0 && len(app.Signature) != 64 {
			return false
		}

		// TODO verify signature
	}

	x.applications[conn] = app
	x.logger.Info("Application accepted", "id", app.Id, "name", app.Name, "description", app.Description, "url", app.Url)

	return true
}

func (x *XSWD) removeApplication(conn *websocket.Conn) {
	x.Lock()
	defer x.Unlock()

	app, found := x.applications[conn]
	if !found {
		return
	}

	delete(x.applications, conn)
	x.logger.Info("Application deleted", "id", app.Id, "name", app.Name, "description", app.Description, "url", app.Url)
}

func (x *XSWD) handleMessage(app ApplicationData, request jrpc2.Request) interface{} {
	methodName := request.Method()
	handler := rpcserver.WalletHandler[methodName]
	if handler == nil {
		x.logger.Info("RPC Method not found", "method", methodName)
		return jrpc2.Errorf(code.MethodNotFound, "method %q not found", methodName)
	}
	if x.requestPermission(app, request) {
		ctx := context.WithValue(context.Background(), "wallet_context", x.context)
		response, err := handler.Handle(ctx, &request)
		if err != nil {
			return jrpc2.Errorf(code.InternalError, "Error while handling request method %q: %v", methodName, err)
		}

		return response
	} else {
		x.logger.Info("Permission not granted for method", "method", methodName)
		return jrpc2.Errorf(code.Cancelled, "Permission not granted for method %q", methodName)
	}
}

func (x *XSWD) requestPermission(app ApplicationData, request jrpc2.Request) bool {
	x.Lock()
	defer x.Unlock()

	perm, found := app.Permissions[request.Method()]
	if !found {
		perm = x.requestHandler(&app, request)
		if perm == AlwaysDeny || perm == AlwaysAllow {
			app.Permissions[request.Method()] = perm
		}
	}

	return perm.IsPositive()
}

func (x *XSWD) readMessageFromSession(conn *websocket.Conn) {
	defer x.removeApplication(conn)

	var request jrpc2.Request
	for {
		// TODO read requets
		if err := conn.ReadJSON(&request); err != nil {
			x.logger.Error(err, "Error while reading message from session")
			return
		}

		app, found := x.applications[conn]
		if !found {
			x.logger.Error(nil, "Application not found")
			return
		}

		response := x.handleMessage(app, request)
		if err := conn.WriteJSON(response); err != nil {
			x.logger.Error(err, "Error while writing JSON")
			return
		}
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
		x.readMessageFromSession(conn)
	} else {
		// TODO Application was rejected, send error
		conn.Close()
	}
}
