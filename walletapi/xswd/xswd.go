package xswd

import (
	"context"
	"encoding/hex"
	"encoding/json"
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

type RPCResponse struct {
	JsonRPC string      `json:"jsonrpc"`
	ID      string      `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   interface{} `json:"error,omitempty"`
}

func ResponseWithError(request *jrpc2.Request, err *jrpc2.Error) RPCResponse {
	var id string
	if request != nil {
		id = request.ID()
	}

	return RPCResponse{
		JsonRPC: "2.0",
		ID:      id,
		Error:   err,
	}
}

func ResponseWithResult(request *jrpc2.Request, result interface{}) RPCResponse {
	var id string
	if request != nil {
		id = request.ID()
	}

	return RPCResponse{
		JsonRPC: "2.0",
		ID:      id,
		Result:  result,
	}
}

type AuthorizationResponse struct {
	Message  string `json:"message"`
	Accepted bool   `json:"accepted"`
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

func (perm Permission) String() string {
	var str string
	if perm == Ask {
		str = "Ask"
	} else if perm == Allow {
		str = "Allow"
	} else if perm == Deny {
		str = "Deny"
	} else if perm == AlwaysAllow {
		str = "Always Allow"
	} else if perm == AlwaysDeny {
		str = "Always Deny"
	} else {
		str = "Unknown"
	}

	return str
}

type XSWD struct {
	// The websocket connected to and its app data
	applications map[*websocket.Conn]ApplicationData
	// function to request access of a dApp to wallet
	appHandler func(*ApplicationData) bool
	// function to request the permission
	requestHandler func(*ApplicationData, *jrpc2.Request) Permission
	handlerMutex   sync.Mutex
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
func NewXSWDServer(wallet *walletapi.Wallet_Disk, appHandler func(*ApplicationData) bool, requestHandler func(*ApplicationData, *jrpc2.Request) Permission) *XSWD {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("XSWD server"))
	})

	server := &http.Server{Addr: ":44326", Handler: mux}

	logger := globals.Logger.WithName("XSWD")

	xswd := &XSWD{
		applications:   make(map[*websocket.Conn]ApplicationData),
		appHandler:     appHandler,
		requestHandler: requestHandler,
		logger:         logger,
		server:         server,
		context:        rpcserver.NewWalletContext(logger, wallet),
		wallet:         wallet,
		// don't create a different API, we provide the same
		rpcHandler: rpcserver.WalletHandler,
		exit:       false,
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
			delete(x.applications, conn)
			if err := conn.Close(); err != nil {
				x.logger.Error(err, "error while closing websocket session")
			}
			break
		}
	}
}

// Add an application from a websocket connection
// it verify that application is valid and add it to the list
func (x *XSWD) addApplication(r *http.Request, conn *websocket.Conn, app ApplicationData) bool {
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
		} else {
			app.Permissions = make(map[string]Permission)
		}

		// Signature can be optional but verify its len
		if len(app.Signature) > 0 {
			if len(app.Signature) != 64 {
				x.logger.Info("Invalid signature size", "signature", len(app.Signature))
				return false
			}

			// TODO verify signature
			/*signer, message, err := x.wallet.CheckSignature(app.Signature)
			if err != nil {
				x.logger.V(1).Info("Invalid signature", "signature", app.Signature)
				return false
			}

			if signer.String() != x.wallet.GetAddress().String() {
				x.logger.V(1).Info("Invalid signer")
				return false
			}*/
		}

	}

	// Check that we don't already have this application
	for _, a := range x.applications {
		if a.Id == app.Id {
			return false
		}
	}

	// check the permission from user
	x.handlerMutex.Lock()
	x.Lock()
	defer func() {
		x.handlerMutex.Unlock()
		x.Unlock()
	}()
	if x.appHandler(&app) {
		x.applications[conn] = app
		x.logger.Info("Application accepted", "id", app.Id, "name", app.Name, "description", app.Description, "url", app.Url)
		return true
	} else {
		x.logger.Info("Application rejected", "id", app.Id, "name", app.Name, "description", app.Description, "url", app.Url)
	}

	return false
}

// Remove an application from the list for a session
func (x *XSWD) removeApplication(conn *websocket.Conn) {
	x.Lock()
	defer x.Unlock()

	app, found := x.applications[conn]
	if !found {
		x.logger.Error(nil, "WebSocket disconnected but was not found!")
		return
	}

	delete(x.applications, conn)
	x.logger.Info("Application deleted", "id", app.Id, "name", app.Name, "description", app.Description, "url", app.Url)
}

// built-in function to sign the ApplicationData with current wallet
func (x *XSWD) handleSignData(app *ApplicationData, request *jrpc2.Request) RPCResponse {
	if x.requestPermission(app, request) {
		x.logger.Info("Signature request accepted")
		app.Signature = make([]byte, 0)
		_, err := json.Marshal(app)
		if err != nil {
			x.logger.Error(err, "Error while marshaling application data")
			return ResponseWithError(request, jrpc2.Errorf(code.InternalError, "Error while marshaling application data"))
		}

		// TODO only save the signature
		//signature := x.wallet.SignData(bytes)
		return ResponseWithError(request, jrpc2.Errorf(code.Cancelled, "WIP"))
		//return signature // TODO
	} else {
		x.logger.Info("Signature request rejected")
		return ResponseWithError(request, jrpc2.Errorf(code.Cancelled, "Permission not granted for signing application data"))
	}
}

// Handle a RPC Request from a session
// We check that the method exists, that the application has the permission to use it
func (x *XSWD) handleMessage(app *ApplicationData, request *jrpc2.Request) interface{} {
	methodName := request.Method()
	handler := x.rpcHandler[methodName]

	// Check that the method exists
	if handler == nil {
		// check if its SignData method
		if methodName == "SignData" {
			return x.handleSignData(app, request)
		}

		// Only requests methods starting with DERO. are sent to daemon
		if strings.HasPrefix(methodName, "DERO.") {
			// if daemon is online, request the daemon
			// wallet play the proxy here
			// and because no sensitive data can be obtained, we allow without requests
			if x.wallet.IsDaemonOnlineCached() {
				var params json.RawMessage
				err := request.UnmarshalParams(&params)
				if err != nil {
					x.logger.V(1).Error(err, "Error while unmarshaling params")
					return ResponseWithError(request, jrpc2.Errorf(code.InvalidParams, "Error while unmarshaling params: %q", err.Error()))
				}

				x.logger.V(2).Info("requesting daemon with", "method", request.Method(), "param", request.ParamString())
				result, err := walletapi.GetRPCClient().RPC.Call(context.Background(), request.Method(), params)
				if err != nil {
					x.logger.V(1).Error(err, "Error on daemon call")
					return ResponseWithError(request, jrpc2.Errorf(code.InvalidRequest, "Error on daemon call: %q", err.Error()))
				}

				// we set original ID
				result.SetID(request.ID())

				json, err := result.MarshalJSON()
				if err != nil {
					x.logger.V(1).Error(err, "Error on marshal daemon response")
					return ResponseWithError(request, jrpc2.Errorf(code.InternalError, "Error on daemon call: %q", err.Error()))
				}

				x.logger.V(2).Info("received response", "response", string(json))
				return result
			}
		}

		x.logger.Info("RPC Method not found", "method", methodName)
		return ResponseWithError(request, jrpc2.Errorf(code.MethodNotFound, "method %q not found", methodName))
	}

	if x.requestPermission(app, request) {
		ctx := context.WithValue(context.Background(), "wallet_context", x.context)
		response, err := handler.Handle(ctx, request)
		if err != nil {
			return ResponseWithError(request, jrpc2.Errorf(code.InternalError, "Error while handling request method %q: %v", methodName, err))
		}

		return ResponseWithResult(request, response)
	} else {
		x.logger.Info("Permission not granted for method", "method", methodName)
		return ResponseWithError(request, jrpc2.Errorf(code.Cancelled, "Permission not granted for method %q", methodName))
	}
}

// Request the permission for a method and save its result if it must be persisted
func (x *XSWD) requestPermission(app *ApplicationData, request *jrpc2.Request) bool {
	x.handlerMutex.Lock()
	defer x.handlerMutex.Unlock()

	perm, found := app.Permissions[request.Method()]
	if !found {
		perm = x.requestHandler(app, request)
		if perm == AlwaysDeny || perm == AlwaysAllow {
			app.Permissions[request.Method()] = perm
		}

		if perm.IsPositive() {
			x.logger.Info("Permission granted", "method", request.Method(), "permission", perm)
		} else {
			x.logger.Info("Permission rejected", "method", request.Method(), "permission", perm)
		}
	} else {
		x.logger.V(1).Info("Permission already granted for method", "method", request.Method(), "permission", perm)
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
			x.logger.V(2).Error(err, "Error while reading message from session")
			return
		}

		// unmarshal the request
		requests, err := jrpc2.ParseRequests(buff)
		if err != nil {
			x.logger.V(2).Error(err, "Error while parsing request")
			conn.WriteJSON(ResponseWithError(nil, jrpc2.Errorf(code.ParseError, "Error while parsing request")))
			continue
		}

		request := requests[0]
		// We only support one request at a time for permission request
		if len(requests) != 1 {
			x.logger.V(2).Error(nil, "Invalid number of requests")
			conn.WriteJSON(ResponseWithError(nil, jrpc2.Errorf(code.ParseError, "Batch are not supported")))
			continue
		}

		x.Lock()
		app, found := x.applications[conn]
		x.Unlock()
		if !found {
			x.logger.V(2).Error(nil, "Application not found")
			conn.WriteJSON(ResponseWithError(request, jrpc2.Errorf(code.InternalError, "Application not found")))
			return
		}

		response := x.handleMessage(&app, request)
		if err := conn.WriteJSON(response); err != nil {
			x.logger.V(2).Error(err, "Error while writing JSON")
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
		x.logger.V(1).Error(err, "WebSocket upgrade error")
		return
	}
	defer conn.Close()

	// first message of the session should be its ApplicationData
	var app_data ApplicationData
	if err := conn.ReadJSON(&app_data); err != nil {
		x.logger.V(1).Error(err, "Error while reading app_data")
		return
	}

	if x.addApplication(r, conn, app_data) {
		conn.WriteJSON(AuthorizationResponse{
			Message:  "User has authorized the application",
			Accepted: true,
		})
		// TODO we should handle the case where user open multiple tabs of the same dApp ?
		x.readMessageFromSession(conn)
	} else {
		conn.WriteJSON(AuthorizationResponse{
			Message:  "User has rejected the application",
			Accepted: false,
		})
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
