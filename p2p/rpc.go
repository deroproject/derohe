package p2p

// this function implements bidirectional rpc using yamux multiplexor
//import "fmt"
import "net"
import "net/rpc"
import "time"
import "io/ioutil"
import "github.com/hashicorp/yamux"

type RPC_Connection struct {
	ClientConn net.Conn    // its used to trigger requests
	Client     *rpc.Client // used to dispatch RPC requests

	ServerConn net.Conn // its used to serve requests

	Session *yamux.Session
	Conn    net.Conn // connection backing everything
}

func yamux_config() *yamux.Config {
	return &yamux.Config{
		AcceptBacklog:          2,
		EnableKeepAlive:        false,
		KeepAliveInterval:      5 * time.Second,
		ConnectionWriteTimeout: 17 * time.Second,
		MaxStreamWindowSize:    uint32(256 * 1024),
		LogOutput:              ioutil.Discard,
	}
}

// do server side processing
func wait_stream_creation_server_side(conn net.Conn) (*RPC_Connection, error) {

	session, err := yamux.Server(conn, yamux_config()) // Setup server side of yamux
	if err != nil {
		conn.Close()
		panic(err)
	}

	// Accept a stream
	client, err := session.Accept()
	if err != nil {
		panic(err)
	}

	server, err := session.Accept()
	if err != nil {
		panic(err)
	}

	rconn := &RPC_Connection{ClientConn: client, ServerConn: server, Session: session, Conn: conn}
	rconn.common_processing()
	return rconn, nil
}

// do client side processing
func stream_creation_client_side(conn net.Conn) (*RPC_Connection, error) {
	session, err := yamux.Client(conn, yamux_config()) // Setup client side of yamux
	if err != nil {
		conn.Close()
		panic(err)
	}

	// create a stream
	client, err := session.Open()
	if err != nil {
		panic(err)
	}
	//create a stream
	server, err := session.Open()
	if err != nil {
		panic(err)
	}

	rconn := &RPC_Connection{ClientConn: server, ServerConn: client, Session: session, Conn: conn} // this line is flipped between client/server
	rconn.common_processing()
	return rconn, nil
}

func (r *RPC_Connection) common_processing() {
	//r.Client = rpc.NewClient(r.ClientConn) // will use GOB encoding, but doesn't have certain protections
	r.Client = rpc.NewClientWithCodec(NewCBORClientCodec(r.ClientConn)) // will use CBOR encoding with protections

	// fmt.Printf("client connection %+v\n", r.Client)

}
