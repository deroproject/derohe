package xswd

import (
	"log"
	"net/http"

	"github.com/gorilla/websocket"
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
	// method to be called
	method string
	// its parameters
	params interface{}
}

type XSWD struct {
	// The websocket connected to and its app data
	applications map[*websocket.Conn]ApplicationData
	// function to request access of a dApp to wallet
	appHandler func(*ApplicationData) Permission
	// function to request the permission
	requestHandler func(*ApplicationData, RPCRequest) Permission
}

func NewXSWDServer(appHandler func(*ApplicationData) Permission, requestHandler func(*ApplicationData, RPCRequest) Permission) *XSWD {
	return &XSWD{
		applications:   make(map[*websocket.Conn]ApplicationData),
		appHandler:     appHandler,
		requestHandler: requestHandler,
	}
}

func (x *XSWD) AddApplication(conn *websocket.Conn, app ApplicationData) bool {
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

// TODO
func (x *XSWD) RequestMethod() {

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

	if x.AddApplication(conn, app_data) {
		// Application was successfully accepted, send response
		// if err := conn.WriteJSON(response); err != nil {
		// 	log.Println("Error while writing JSON:", err)
		// 	return
		// }
	} else {
		// TODO Application was rejected, send error
		conn.Close()
	}
}
