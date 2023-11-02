package xswd

import (
	"context"

	"github.com/deroproject/derohe/walletapi/rpcserver"
)

type HasMethod_Params struct {
	Name string `json:"name"`
}

func HasMethod(ctx context.Context, p HasMethod_Params) bool {
	w := rpcserver.FromContext(ctx)
	xswd := w.Extra["xswd"].(*XSWD)
	_, ok := xswd.rpcHandler[p.Name]
	return ok
}

// TODO WIP sign data
func SignData(ctx context.Context, p string) string {
	// w := rpcserver.FromContext(ctx)
	// xswd := w.Extra["xswd"].(*XSWD)
	return "WIP"
}
