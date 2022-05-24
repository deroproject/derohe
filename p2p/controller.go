// Copyright 2017-2021 DERO Project. All rights reserved.
// Use of this source code in any form is governed by RESEARCH license.
// license can be found in the LICENSE file.
// GPG: 0F39 E425 8C65 3947 702A  8234 08B2 0360 A03A 9DE8
//
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND ANY
// EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF
// MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL
// THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
// SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO,
// PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS
// INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT,
// STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF
// THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

package p2p

import "fmt"
import "net"

import "os"
import "time"
import "sort"
import "sync"
import "strings"
import "math/big"
import "strconv"

import "crypto/sha1"
import "crypto/ecdsa"
import "crypto/elliptic"

import "crypto/tls"
import "crypto/rand"
import "crypto/x509"
import "encoding/pem"
import "sync/atomic"
import "runtime/debug"

import "github.com/go-logr/logr"

import "github.com/deroproject/derohe/config"
import "github.com/deroproject/derohe/globals"
import "github.com/deroproject/derohe/metrics"
import "github.com/deroproject/derohe/blockchain"

import "github.com/xtaci/kcp-go/v5"
import "golang.org/x/crypto/pbkdf2"
import "golang.org/x/time/rate"

import "github.com/cenkalti/rpc2"

//import "github.com/txthinking/socks5"

var chain *blockchain.Blockchain // external reference to chain

var P2P_Port int // this will be exported while doing handshake

var Exit_Event = make(chan bool) // causes all threads to exit
var Exit_In_Progress bool        // marks we are doing exit
var logger logr.Logger           // global logger, every logger in this package is a child of this
var sync_node bool               // whether sync mode is activated

var nonbanlist []string // any ips in this list will never be banned
// the list will include seed nodes, any nodes provided at command prompt

var ClockOffset time.Duration //Clock Offset related to all the peer2 connected

// also backoff is used if we have initiated a connect we will not connect to it again for another 10 secs
var backoff = map[string]int64{} // if server receives a connection, then it will not initiate connection to that ip for another 60 secs
var backoff_mutex = sync.Mutex{}

var Min_Peers = int64(31) // we need to expose this to be modifieable at runtime without taking daemon offline
var Max_Peers = int64(101)

// return true if we should back off else we can connect
func shouldwebackoff(ip string) bool {
	backoff_mutex.Lock()
	defer backoff_mutex.Unlock()

	now := time.Now().Unix()
	for k, v := range backoff { // random backing off
		if v < now {
			delete(backoff, k)
		}
	}

	if backoff[ip] != 0 { // now lets do the test
		return true
	}
	return false
}

// Initialize P2P subsystem
func P2P_Init(params map[string]interface{}) error {
	logger = globals.Logger.WithName("P2P") // all components must use this logger

	// register_handlers()

	GetPeerID() // Initialize peer id once

	// parse node tag if availble
	if _, ok := globals.Arguments["--node-tag"]; ok {
		if globals.Arguments["--node-tag"] != nil {
			node_tag = globals.Arguments["--node-tag"].(string)
		}
	}
	if os.Getenv("TURBO") == "0" {
		logger.Info("P2P is in normal mode")
	} else {
		logger.Info("P2P is in turbo mode")
	}

	if os.Getenv("BW_FACTOR") != "" {
		bw_factor, _ := strconv.Atoi(os.Getenv("BW_FACTOR"))
		if bw_factor <= 0 {
			bw_factor = 1
		}
		logger.Info("", "BW_FACTOR", bw_factor)
	}

	if os.Getenv("UDP_READ_BUF_CONN") != "" {
		size, _ := strconv.Atoi(os.Getenv("UDP_READ_BUF_CONN"))
		if size <= 64*1024 {
			size = 64 * 1024
		}
		logger.Info("", "UDP_READ_BUF_CONN", size)
	}

	// permanently unban any seed nodes
	if globals.IsMainnet() {
		for i := range config.Mainnet_seed_nodes {
			nonbanlist = append(nonbanlist, strings.ToLower(config.Mainnet_seed_nodes[i]))
		}
	} else { // initial bootstrap
		for i := range config.Testnet_seed_nodes {
			nonbanlist = append(nonbanlist, strings.ToLower(config.Testnet_seed_nodes[i]))
		}
	}

	chain = params["chain"].(*blockchain.Blockchain)
	load_ban_list()  // load ban list
	load_peer_list() // load old list if availble

	// if user provided a sync node, connect with it
	if _, ok := globals.Arguments["--sync-node"]; ok { // check if parameter is supported
		if globals.Arguments["--sync-node"].(bool) {
			sync_node = true
			// disable p2p port
			globals.Arguments["--p2p-bind"] = ":0"

			// disable all connections except seed nodes
			globals.Arguments["--add-exclusive-node"] = []string{"0.0.0.0:0"}
			globals.Arguments["--add-priority-node"] = []string{"0.0.0.0:0"}

			go maintain_seed_node_connection()

			logger.Info("Sync mode is enabled. Please remove this option after chain syncs successfully")
		}
	}

	go P2P_Server_v2()                                          // start accepting connections
	go P2P_engine()                                             // start outgoing engine
	globals.Cron.AddFunc("@every 4s", syncroniser)              // start sync engine
	globals.Cron.AddFunc("@every 5s", Connection_Pending_Clear) // clean dead connections
	globals.Cron.AddFunc("@every 10s", ping_loop)               // ping every one
	globals.Cron.AddFunc("@every 10s", chunks_clean_up)         // clean chunks

	go time_check_routine() // check whether server time is in sync using ntp

	metrics.Set.NewGauge("p2p_peer_count", func() float64 { // set a new gauge
		count := float64(0)
		connection_map.Range(func(k, value interface{}) bool {
			if v := value.(*Connection); atomic.LoadUint32(&v.State) != HANDSHAKE_PENDING {
				count++
			}
			return true
		})
		return count
	})
	metrics.Set.NewGauge("p2p_peer_incoming_count", func() float64 { // set a new gauge
		count := float64(0)
		connection_map.Range(func(k, value interface{}) bool {
			if v := value.(*Connection); atomic.LoadUint32(&v.State) != HANDSHAKE_PENDING && v.Incoming {
				count++
			}
			return true
		})
		return count
	})
	metrics.Set.NewGauge("p2p_peer_outgoing_count", func() float64 { // set a new gauge
		count := float64(0)
		connection_map.Range(func(k, value interface{}) bool {
			if v := value.(*Connection); atomic.LoadUint32(&v.State) != HANDSHAKE_PENDING && !v.Incoming {
				count++
			}
			return true
		})
		return count
	})

	logger.Info("P2P started")
	atomic.AddUint32(&globals.Subsystem_Active, 1) // increment subsystem
	return nil
}

// TODO we need to make sure that exclusive/priority nodes are never banned
func P2P_engine() {

	var end_point_list []string
	if _, ok := globals.Arguments["--add-exclusive-node"]; ok { // check if parameter is supported
		if globals.Arguments["--add-exclusive-node"] != nil {
			tmp_list := globals.Arguments["--add-exclusive-node"].([]string)
			for i := range tmp_list {
				end_point_list = append(end_point_list, tmp_list[i])
				nonbanlist = append(nonbanlist, tmp_list[i])
			}
		}
	}

	// all prority nodes will be always connected
	if _, ok := globals.Arguments["--add-priority-node"]; ok { // check if parameter is supported
		if globals.Arguments["--add-priority-node"] != nil {
			tmp_list := globals.Arguments["--add-priority-node"].([]string)
			for i := range tmp_list {
				end_point_list = append(end_point_list, tmp_list[i])
				nonbanlist = append(nonbanlist, tmp_list[i])
			}
		}
	}

	{ // remove duplicates if any
		sort.Strings(end_point_list)
	start_again: // this list is expected to be less  than 100
		for i := range end_point_list {
			if i > 0 && end_point_list[i-1] == end_point_list[i] {
				end_point_list = append(end_point_list[:i-1], end_point_list[i:]...)
				goto start_again
			}
		}
	}

	//logger.Debugf("Priority list %+v", end_point_list)

	// maintain connection to exclusive/priority nodes
	for i := range end_point_list {
		go maintain_outgoing_priority_connection(end_point_list[i], false)
	}

	// do not create connections to peers , if requested
	if _, ok := globals.Arguments["--add-exclusive-node"]; ok && len(globals.Arguments["--add-exclusive-node"].([]string)) == 0 { // check if parameter is supported
		go maintain_connection_to_peers() // maintain certain number of connections for peer to peers

		// skip any connections, to allow more testing in closed environments
		if os.Getenv("SKIP_SEED_NODES") != "" {
			return
		}
		go maintain_seed_node_connection() // maintain connection with atleast 1 seed node

		// this code only triggers when we do not have peer list
		if find_peer_to_connect(1) == nil { // either we donot have a peer list or everyone is banned
			// trigger connection to all seed nodes hoping some will be up
			if globals.IsMainnet() { // initial boot strap should be quick
				for i := range config.Mainnet_seed_nodes {
					go connect_with_endpoint(config.Mainnet_seed_nodes[i], true)
				}
			} else { // initial bootstrap
				for i := range config.Testnet_seed_nodes {
					go connect_with_endpoint(config.Testnet_seed_nodes[i], true)
				}
			}

		}

	}

}

func tunekcp(conn *kcp.UDPSession) {
	conn.SetACKNoDelay(true)
	if os.Getenv("TURBO") == "0" {
		conn.SetNoDelay(1, 10, 2, 1) // tuning paramters for local stack for fast retransmission stack
	} else {
		conn.SetNoDelay(0, 40, 0, 0) // tuning paramters for local
	}

	size := 1 * 1024 * 1024 // set the buffer size max possible upto 1 MB, default is 1 MB
	if os.Getenv("UDP_READ_BUF_CONN") != "" {
		size, _ = strconv.Atoi(os.Getenv("UDP_READ_BUF_CONN"))
		if size <= 64*1024 {
			size = 64 * 1024
		}
	}
	for size >= 64*1024 {
		if err := conn.SetReadBuffer(size); err == nil {
			break
		}
		size = size - (64 * 1024)
	}
}

// will try to connect with given endpoint
// will block until the connection dies or is killed
func connect_with_endpoint(endpoint string, sync_node bool) {

	defer globals.Recover(2)

	remote_ip, err := net.ResolveUDPAddr("udp", endpoint)
	if err != nil {
		logger.V(3).Error(err, "Resolve address failed:", "endpoint", endpoint)
		return
	}

	if IsAddressInBanList(ParseIPNoError(remote_ip.IP.String())) {
		logger.V(2).Info("Connecting to banned IP is prohibited", "IP", remote_ip.IP.String())
		return
	}

	// check whether are already connected to this address if yes, return
	if IsAddressConnected(ParseIPNoError(remote_ip.String())) {
		logger.V(4).Info("outgoing address is already connected", "ip", remote_ip.String())
		return //nil, fmt.Errorf("Already connected")
	}

	if shouldwebackoff(ParseIPNoError(remote_ip.String())) {
		logger.V(1).Info("backing off from this connection", "ip", remote_ip.String())
		return
	} else {
		backoff_mutex.Lock()
		backoff[ParseIPNoError(remote_ip.String())] = time.Now().Unix() + 10
		backoff_mutex.Unlock()
	}

	var masterkey = pbkdf2.Key(globals.Config.Network_ID.Bytes(), globals.Config.Network_ID.Bytes(), 1024, 32, sha1.New)
	var blockcipher, _ = kcp.NewAESBlockCrypt(masterkey)

	var conn *kcp.UDPSession

	// since we may be connecting through socks, grab the remote ip for our purpose rightnow
	//conn, err := globals.Dialer.Dial("tcp", remote_ip.String())
	if globals.Arguments["--socks-proxy"] == nil {
		conn, err = kcp.DialWithOptions(remote_ip.String(), blockcipher, 10, 3)
	} else { // we must move through a socks 5 UDP ASSOCIATE supporting proxy, ssh implementation is partial
		err = fmt.Errorf("socks proxying is not supported")
		logger.V(0).Error(err, "Not suported", "server", globals.Arguments["--socks-proxy"])
		return
		/*uri, err := url.Parse("socks5://" + globals.Arguments["--socks-proxy"].(string)) // "socks5://demo:demo@192.168.99.100:1080"
		if err != nil {
			logger.V(0).Error(err, "Error parsing socks proxy", "server", globals.Arguments["--socks-proxy"])
			return
		}
		_ = uri
		sserver := uri.Host
		if uri.Port() != "" {

			host, _, err := net.SplitHostPort(uri.Host)
			if err != nil {
				logger.V(0).Error(err, "Error parsing socks proxy", "server", globals.Arguments["--socks-proxy"])
				return
			}
			sserver = host  + ":"+ uri.Port()
		}

		fmt.Printf("sserver %s   host %s port %s\n", sserver, uri.Host, uri.Port())
		username := ""
		password := ""
		if uri.User != nil {
			username = uri.User.Username()
			password,_ = uri.User.Password()
		}
		tcpTimeout := 10
		udpTimeout := 10
		c, err := socks5.NewClient(sserver, username, password, tcpTimeout, udpTimeout)
		if err != nil {
			logger.V(0).Error(err, "Error connecting to socks proxy", "server", globals.Arguments["--socks-proxy"])
			return
		}
		udpconn, err := c.Dial("udp", remote_ip.String())
		if err != nil {
			logger.V(0).Error(err, "Error connecting to remote host using socks proxy", "socks", globals.Arguments["--socks-proxy"],"remote",remote_ip.String())
			return
		}
		conn,err = kcp.NewConn(remote_ip.String(),blockcipher,10,3,udpconn)
		*/
	}

	if err != nil {
		logger.V(3).Error(err, "Dial failed", "endpoint", endpoint)
		Peer_SetFail(ParseIPNoError(remote_ip.String())) // update peer list as we see
		conn.Close()
		return //nil, fmt.Errorf("Dial failed err %s", err.Error())
	}

	tunekcp(conn) // set tunings for low latency

	// TODO we need to choose fastest cipher here ( so both clients/servers are not loaded)
	conntls := tls.Client(conn, &tls.Config{InsecureSkipVerify: true})
	process_outgoing_connection(conn, conntls, remote_ip, false, sync_node)

}

// maintains a persistant connection to endpoint
// if connection drops, tries again after 4 secs
func maintain_outgoing_priority_connection(endpoint string, sync_node bool) {
	delay := time.NewTicker(4 * time.Second)
	for {
		select {
		case <-Exit_Event:
			return
		case <-delay.C:
		}
		connect_with_endpoint(endpoint, sync_node)
	}
}

// this will maintain connection to 1 seed node randomly
func maintain_seed_node_connection() {

	delay := time.NewTicker(2 * time.Second)

	for {
		select {
		case <-Exit_Event:
			return
		case <-delay.C:
		}
		endpoint := ""
		if globals.IsMainnet() { // choose mainnet seed node
			r, _ := rand.Int(rand.Reader, big.NewInt(10240))
			endpoint = config.Mainnet_seed_nodes[r.Int64()%int64(len(config.Mainnet_seed_nodes))]
		} else { // choose testnet peer node
			r, _ := rand.Int(rand.Reader, big.NewInt(10240))
			endpoint = config.Testnet_seed_nodes[r.Int64()%int64(len(config.Testnet_seed_nodes))]
		}
		if endpoint != "" {
			//connect_with_endpoint(endpoint, sync_node)
			connect_with_endpoint(endpoint, true) // seed nodes always have sync mode
		}
	}
}

// keep building connections to network, we are talking outgoing connections
func maintain_connection_to_peers() {

	// check how many connections are active
	if _, ok := globals.Arguments["--min-peers"]; ok && globals.Arguments["--min-peers"] != nil { // user specified a limit, use it if possible
		i, err := strconv.ParseInt(globals.Arguments["--min-peers"].(string), 10, 64)
		if err != nil {
			logger.Error(err, "Error Parsing --min-peers")
		} else {
			if i <= 1 {
				logger.Error(fmt.Errorf("--min-peers should be positive and more than 1"), "")
			} else {
				Min_Peers = i
			}
		}
		logger.Info("Min peers", "min-peers", Min_Peers)
	}

	if _, ok := globals.Arguments["--max-peers"]; ok && globals.Arguments["--max-peers"] != nil { // user specified a limit, use it if possible
		i, err := strconv.ParseInt(globals.Arguments["--max-peers"].(string), 10, 64)
		if err != nil {
			logger.Error(err, "Error Parsing --max-peers")
		} else {
			if i < Min_Peers {
				logger.Error(fmt.Errorf("--max-peers should be positive and more than --min-peers"), "")
			} else {
				Max_Peers = i
			}
		}
		logger.Info("Max peers", "max-peers", Max_Peers)
	}

	delay := time.NewTicker(200 * time.Millisecond)

	for {
		select {
		case <-Exit_Event:
			return
		case <-delay.C:
		}

		// check number of connections, if limit is reached, trigger new connections if we have peers
		// if we have more do nothing
		_, out := Peer_Direction_Count()
		if out >= uint64(Min_Peers) { // we already have required number of peers, donot connect to more peers
			continue
		}

		peer := find_peer_to_connect(1)
		if peer != nil && !IsAddressConnected(ParseIPNoError(peer.Address)) {
			go connect_with_endpoint(peer.Address, false)
		}
	}
}

func P2P_Server_v2() {

	var accept_limiter = rate.NewLimiter(10.0, 40) // 10 incoming per sec, burst of 40 is okay

	default_address := "0.0.0.0:0" // be default choose a random port
	if _, ok := globals.Arguments["--p2p-bind"]; ok && globals.Arguments["--p2p-bind"] != nil {
		addr, err := net.ResolveTCPAddr("tcp", globals.Arguments["--p2p-bind"].(string))
		if err != nil {
			logger.Error(err, "--p2p-bind address is invalid")
		} else {
			if addr.Port == 0 {
				logger.Info("P2P server is disabled, No ports will be opened for P2P activity")
				return
			} else {
				default_address = addr.String()
				P2P_Port = addr.Port
			}
		}
	}

	srv := rpc2.NewServer()
	srv.OnConnect(func(c *rpc2.Client) {
		remote_addr_interface, _ := c.State.Get("addr")
		remote_addr := remote_addr_interface.(net.Addr)

		conn_interface, _ := c.State.Get("conn")
		conn := conn_interface.(net.Conn)

		tlsconn_interface, _ := c.State.Get("tlsconn")
		tlsconn := tlsconn_interface.(net.Conn)

		connection := &Connection{Client: c, Conn: conn, ConnTls: tlsconn, Addr: remote_addr, State: HANDSHAKE_PENDING, Incoming: true}
		connection.logger = logger.WithName("incoming").WithName(remote_addr.String())

		in, out := Peer_Direction_Count()

		if int64(in+out) > Max_Peers { // do not allow incoming ddos
			connection.exit()
			return
		}

		c.State.Set("c", connection) // set pointer to connection

		//connection.logger.Info("connected  OnConnect")
		go func() {
			time.Sleep(2 * time.Second)
			connection.dispatch_test_handshake()
		}()

	})

	set_handlers(srv)

	tlsconfig := &tls.Config{Certificates: []tls.Certificate{generate_random_tls_cert()}}
	//l, err := tls.Listen("tcp", default_address, tlsconfig) // listen as TLS server

	_ = tlsconfig

	var masterkey = pbkdf2.Key(globals.Config.Network_ID.Bytes(), globals.Config.Network_ID.Bytes(), 1024, 32, sha1.New)
	var blockcipher, _ = kcp.NewAESBlockCrypt(masterkey)

	// listen to incoming tcp connections tls style
	l, err := kcp.ListenWithOptions(default_address, blockcipher, 10, 3)
	if err != nil {
		logger.Error(err, "Could not listen", "address", default_address)
		return
	}
	defer l.Close()

	_, P2P_Port_str, _ := net.SplitHostPort(l.Addr().String())
	P2P_Port, _ = strconv.Atoi(P2P_Port_str)

	logger.Info("P2P is listening", "address", l.Addr().String())

	// A common pattern is to start a loop to continously accept connections
	for {
		conn, err := l.AcceptKCP() //accept connections using Listener.Accept()
		if err != nil {
			select {
			case <-Exit_Event:
				l.Close() // p2p is shutting down, close the listening socket
				return
			default:
			}
			logger.V(1).Error(err, "Err while accepting incoming connection")
			continue
		}

		if !accept_limiter.Allow() { // if rate limiter allows, then only add else drop the connection
			conn.Close()
			continue
		}

		raddr := conn.RemoteAddr().(*net.UDPAddr)

		backoff_mutex.Lock()
		backoff[ParseIPNoError(raddr.String())] = time.Now().Unix() + globals.Global_Random.Int63n(200) // random backing of upto 200 secs
		backoff_mutex.Unlock()

		logger.V(3).Info("accepting incoming connection", "raddr", raddr.String())

		if IsAddressConnected(ParseIPNoError(raddr.String())) {
			logger.V(4).Info("incoming address is already connected", "ip", raddr.String())
			conn.Close()

		} else if IsAddressInBanList(ParseIPNoError(raddr.IP.String())) { //if incoming IP is banned, disconnect now
			logger.V(2).Info("Incoming IP is banned, disconnecting now", "IP", raddr.IP.String())
			conn.Close()
		}

		tunekcp(conn) // tuning paramters for local stack
		tlsconn := tls.Server(conn, tlsconfig)
		state := rpc2.NewState()
		state.Set("addr", raddr)
		state.Set("conn", conn)
		state.Set("tlsconn", tlsconn)

		go srv.ServeCodecWithState(NewCBORCodec(tlsconn), state)

	}

}

func handle_connection_panic(c *Connection) {
	defer globals.Recover(2)
	if r := recover(); r != nil {
		logger.V(2).Error(nil, "Recovered while handling connection", "r", r, "stack", string(debug.Stack()))
		c.exit()
	}
}

func set_handler(base interface{}, methodname string, handler interface{}) {
	switch o := base.(type) {
	case *rpc2.Client:
		o.Handle(methodname, handler)
		//fmt.Printf("setting client handler %s\n", methodname)
	case *rpc2.Server:
		o.Handle(methodname, handler)
		//fmt.Printf("setting server handler %s\n", methodname)
	default:
		panic(fmt.Sprintf("object cannot handle such handler %T", base))

	}
}

func getc(client *rpc2.Client) *Connection {
	if ci, found := client.State.Get("c"); found {
		return ci.(*Connection)
	} else {
		//panic("no connection attached") // automatically handled by higher layers
		return nil
	}
}

// we need the following RPCS to work
func set_handlers(o interface{}) {
	set_handler(o, "Peer.Handshake", func(client *rpc2.Client, args Handshake_Struct, reply *Handshake_Struct) error {
		return getc(client).Handshake(args, reply)
	})
	set_handler(o, "Peer.Chain", func(client *rpc2.Client, args Chain_Request_Struct, reply *Chain_Response_Struct) error {
		return getc(client).Chain(args, reply)
	})
	set_handler(o, "Peer.ChangeSet", func(client *rpc2.Client, args ChangeList, reply *Changes) error {
		return getc(client).ChangeSet(args, reply)
	})
	set_handler(o, "Peer.NotifyINV", func(client *rpc2.Client, args ObjectList, reply *Dummy) error {
		return getc(client).NotifyINV(args, reply)
	})
	set_handler(o, "Peer.GetObject", func(client *rpc2.Client, args ObjectList, reply *Objects) error {
		return getc(client).GetObject(args, reply)
	})
	set_handler(o, "Peer.TreeSection", func(client *rpc2.Client, args Request_Tree_Section_Struct, reply *Response_Tree_Section_Struct) error {
		return getc(client).TreeSection(args, reply)
	})
	set_handler(o, "Peer.NotifyMiniBlock", func(client *rpc2.Client, args Objects, reply *Dummy) error {
		return getc(client).NotifyMiniBlock(args, reply)
	})
	set_handler(o, "Peer.Ping", func(client *rpc2.Client, args Dummy, reply *Dummy) error {
		return getc(client).Ping(args, reply)
	})

}

func process_outgoing_connection(conn net.Conn, tlsconn net.Conn, remote_addr net.Addr, incoming, sync_node bool) {
	defer globals.Recover(0)

	client := rpc2.NewClientWithCodec(NewCBORCodec(tlsconn))

	c := &Connection{Client: client, Conn: conn, ConnTls: tlsconn, Addr: remote_addr, State: HANDSHAKE_PENDING, Incoming: incoming, SyncNode: sync_node}
	defer c.exit()
	c.logger = logger.WithName("outgoing").WithName(remote_addr.String())
	set_handlers(client)

	client.State = rpc2.NewState()
	client.State.Set("c", c)

	go func() {
		time.Sleep(2 * time.Second)
		c.dispatch_test_handshake()
	}()

	//	c.logger.V(4).Info("client running loop")
	client.Run() // see the original

	c.logger.V(4).Info("process_connection finished")
}

// shutdown the p2p component
func P2P_Shutdown() {
	//close(Exit_Event) // send signal to all connections to exit
	save_peer_list() // save peer list
	save_ban_list()  // save ban list

	// TODO we  must wait for connections to kill themselves
	logger.Info("P2P Shutdown")
	atomic.AddUint32(&globals.Subsystem_Active, ^uint32(0)) // this decrement 1 fom subsystem

}

// generate default tls cert to encrypt everything
// NOTE: this does NOT protect from individual active man-in-the-middle attacks
func generate_random_tls_cert() tls.Certificate {

	/* RSA can do only 500 exchange per second, we need to be faster
	     * reference https://github.com/golang/go/issues/20058
	    key, err := rsa.GenerateKey(rand.Reader, 512) // current using minimum size
	if err != nil {
	    log.Fatal("Private key cannot be created.", err.Error())
	}

	// Generate a pem block with the private key
	keyPem := pem.EncodeToMemory(&pem.Block{
	    Type:  "RSA PRIVATE KEY",
	    Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
	*/
	// EC256 does roughly 20000 exchanges per second
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	b, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		logger.Error(err, "Unable to marshal ECDSA private key")
		panic(err)
	}
	// Generate a pem block with the private key
	keyPem := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: b})

	tml := x509.Certificate{
		SerialNumber: big.NewInt(int64(GetPeerID()) ^ int64(time.Now().UnixNano())),

		// TODO do we need to add more parameters to make our certificate more authentic
		// and thwart traffic identification as a mass scale
		/*
		   // you can add any attr that you need
		   NotBefore:    time.Now(),
		   NotAfter:     time.Now().AddDate(5, 0, 0),
		   // you have to generate a different serial number each execution

		   Subject: pkix.Name{
		       CommonName:   "New Name",
		       Organization: []string{"New Org."},
		   },
		   BasicConstraintsValid: true,   // even basic constraints are not required
		*/
	}
	cert, err := x509.CreateCertificate(rand.Reader, &tml, &tml, &key.PublicKey, key)
	if err != nil {
		logger.Error(err, "Certificate cannot be created.")
		panic(err)
	}

	// Generate a pem block with the certificate
	certPem := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert})
	tlsCert, err := tls.X509KeyPair(certPem, keyPem)
	if err != nil {
		logger.Error(err, "Certificate cannot be loaded.")
		panic(err)
	}
	return tlsCert
}

func ParseIP(s string) (string, error) {
	ip, _, err := net.SplitHostPort(s)
	if err == nil {
		return ip, nil
	}

	ip2 := net.ParseIP(s)
	if ip2 == nil {
		return "", fmt.Errorf("invalid IP")
	}

	return ip2.String(), nil
}

func ParseIPNoError(s string) string {
	ip, _ := ParseIP(s)
	return ip
}
