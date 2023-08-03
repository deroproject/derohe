package xswd

import (
	"context"
	"encoding/hex"
	"log"
	"net/http"
	"strings"
	"sync"
	"unicode"

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
	requestHandler func(*ApplicationData, *jrpc2.Request) Permission
	server         *http.Server
	logger         logr.Logger
	context        *rpcserver.WalletContext
	wallet         *walletapi.Wallet_Disk
	rpcHandler     handler.Map
	exit           bool
	// mutex for applications map
	sync.Mutex
}

// Create a new XSWD server which allows to connect any dApp to the wallet safely through a websocket
// Each request done by the session will wait on the appHandler and requestHandler to be accepted
func NewXSWDServer(wallet *walletapi.Wallet_Disk, appHandler func(*ApplicationData) Permission, requestHandler func(*ApplicationData, *jrpc2.Request) Permission) *XSWD {
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
		exit:           false,
		rpcHandler:     rpcHandler,
	}

	mux.HandleFunc("/xswd", xswd.handleWebSocket)
	logger.Info("Starting XSWD server", "addr", server.Addr)

	go func() {
		if err := xswd.server.ListenAndServe(); err != nil {
			if !xswd.exit {
				logger.Error(err, "Error while starting XSWD server")
			}
		}
	}()

	return xswd
}

// Stop the XSWD server
// This will close all the connections
// and delete all applications
func (x *XSWD) Stop() {
	x.Lock()
	defer x.Unlock()
	x.exit = true

	x.server.Shutdown(context.Background())
	x.applications = make(map[*websocket.Conn]ApplicationData)
	x.logger.Info("XSWD server stopped")
}

// Get all connected Applications
// This will return a copy of the map
func (x *XSWD) GetApplications() []ApplicationData {
	x.Lock()
	defer x.Unlock()

	apps := make([]ApplicationData, 0, len(x.applications))
	for _, app := range x.applications {
		apps = append(apps, app)
	}

	return apps
}

// Remove an application
// It will automatically close the connection
func (x *XSWD) RemoveApplication(app *ApplicationData) {
	x.Lock()
	defer x.Unlock()

	for conn, a := range x.applications {
		if a.Id == app.Id {
			conn.Close()
			delete(x.applications, conn)
			break
		}
	}
}

// Add an application from a websocket connection
// it verify that application is valid and add it to the list
func (x *XSWD) addApplication(r *http.Request, conn *websocket.Conn, app ApplicationData) bool {
	x.Lock()
	defer x.Unlock()

	// Sanity check
	{
		id := strings.TrimSpace(app.Id)
		if len(id) != 64 {
			x.logger.V(1).Info("Invalid ID size")
			return false
		}

		if _, err := hex.DecodeString(id); err != nil {
			x.logger.V(1).Info("Invalid hexadecimal ID")
			return false
		}

		if len(strings.TrimSpace(app.Name)) == 0 || len(app.Name) > 255 || !isASCII(app.Name) {
			x.logger.V(1).Info("Invalid name", "name", len(app.Name))
			return false
		}

		if len(strings.TrimSpace(app.Description)) == 0 || len(app.Description) > 255 || !isASCII(app.Description) {
			x.logger.V(1).Info("Invalid description", "description", len(app.Description))
			return false
		}

		if len(app.Url) == 0 {
			app.Url = r.Header.Get("Origin")
			if len(app.Url) > 0 {
				x.logger.V(1).Info("No URL passed, checking origin header")
			}
		}

		// URL can be optional
		if len(app.Url) > 255 {
			x.logger.V(1).Info("Invalid URL", "url", len(app.Url))
			return false
		}

		// Check that URL is starting with valid protocol
		if !(strings.HasPrefix(app.Url, "http://") || strings.HasPrefix(app.Url, "https://")) {
			x.logger.V(1).Info("Invalid application URL", "url", app.Url)
			return false
		}

		// Signature can be optional but permissions can't exist without signature
		if app.Permissions != nil {
			if (len(app.Permissions) > 0 && len(app.Signature) != 64) || len(app.Permissions) > 255 {
				x.logger.V(1).Info("Invalid permissions", "permissions", len(app.Permissions))
				return false
			}
		}

		// Signature can be optional but verify its len
		if len(app.Signature) > 0 && len(app.Signature) != 64 {
			x.logger.Info("Invalid signature size", "signature", len(app.Signature))
			return false
		}

		// TODO verify signature
	}

	// Check that we don't already have this application
	for _, a := range x.applications {
		if a.Id == app.Id {
			return false
		}
	}

	// check the permission from user
	if x.appHandler(&app).IsPositive() {
		x.applications[conn] = app
		x.logger.Info("Application accepted", "id", app.Id, "name", app.Name, "description", app.Description, "url", app.Url)
		return true
	}

	return false
}

// Remove an application from the list for a session
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

// Handle a RPC Request from a session
// We check that the method exists, that the application has the permission to use it
func (x *XSWD) handleMessage(app ApplicationData, request *jrpc2.Request) interface{} {
	methodName := request.Method()
	handler := rpcserver.WalletHandler[methodName]

	if handler == nil {
		x.logger.Info("RPC Method not found", "method", methodName)
		return jrpc2.Errorf(code.MethodNotFound, "method %q not found", methodName)
	}

	if x.requestPermission(app, request) {
		ctx := context.WithValue(context.Background(), "wallet_context", x.context)
		response, err := handler.Handle(ctx, request)
		if err != nil {
			return jrpc2.Errorf(code.InternalError, "Error while handling request method %q: %v", methodName, err)
		}

		return response
	} else {
		x.logger.Info("Permission not granted for method", "method", methodName)
		return jrpc2.Errorf(code.Cancelled, "Permission not granted for method %q", methodName)
	}
}

// Request the permission for a method and save its result if it must be persisted
func (x *XSWD) requestPermission(app ApplicationData, request *jrpc2.Request) bool {
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

// block until the session is closed and read all its messages
func (x *XSWD) readMessageFromSession(conn *websocket.Conn) {
	defer x.removeApplication(conn)

	for {
		// block and read the message bytes from session
		_, buff, err := conn.ReadMessage()
		if err != nil {
			x.logger.Error(err, "Error while reading message from session")
			return
		}

		// unmarshal the request
		requests, err := jrpc2.ParseRequests(buff)
		request := requests[0]
		// We only support one request at a time for permission request
		if len(requests) != 1 {
			x.logger.Error(nil, "Invalid number of requests")
			return
		}

		if err != nil {
			x.logger.Error(err, "Error while parsing request")
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

// Handle a WebSocket connection
func (x *XSWD) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Accept from any origin
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket upgrade error:", err)
		return
	}
	defer conn.Close()

	// first message of the session should be its ApplicationData
	var app_data ApplicationData
	if err := conn.ReadJSON(&app_data); err != nil {
		log.Println("Error while reading app_data:", err)
		return
	}

	if x.addApplication(r, conn, app_data) {
		// TODO we should handle the case where user open multiple tabs of the same dApp ?
		x.readMessageFromSession(conn)
	}
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] > unicode.MaxASCII {
			return false
		}
	}
	return true
}
