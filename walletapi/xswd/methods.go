package xswd

import (
	"context"

	"github.com/deroproject/derohe/rpc"
	"github.com/deroproject/derohe/walletapi/rpcserver"
)

type HasMethod_Params struct {
	Name string `json:"name"`
}

type Subscribe_Params struct {
	Event rpc.EventType `json:"event"`
}

func HasMethod(ctx context.Context, p HasMethod_Params) bool {
	w := rpcserver.FromContext(ctx)
	xswd := w.Extra["xswd"].(*XSWD)
	_, ok := xswd.rpcHandler[p.Name]
	return ok
}

func Subscribe(ctx context.Context, p Subscribe_Params) bool {
	w := rpcserver.FromContext(ctx)
	app := w.Extra["app_data"].(*ApplicationData)

	_, ok := app.RegisteredEvents[p.Event]
	if ok {
		return false
	}

	app.RegisteredEvents[p.Event] = true

	return true
}

func Unsubscribe(ctx context.Context, p Subscribe_Params) bool {
	w := rpcserver.FromContext(ctx)
	app := w.Extra["app_data"].(*ApplicationData)

	_, ok := app.RegisteredEvents[p.Event]
	if !ok {
		return false
	}

	delete(app.RegisteredEvents, p.Event)

	return true
}

// TODO WIP sign data
func SignData(ctx context.Context, p string) string {
	// w := rpcserver.FromContext(ctx)
	// xswd := w.Extra["xswd"].(*XSWD)
	return "WIP"
}
