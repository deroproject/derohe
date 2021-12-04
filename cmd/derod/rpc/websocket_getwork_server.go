package rpc

import (
	"flag"
	"fmt"
	"net/http"

	"time"

	"github.com/lesismal/llib/std/crypto/tls"
	"github.com/lesismal/nbio/nbhttp"
	"github.com/lesismal/nbio/nbhttp/websocket"
)

import "github.com/lesismal/nbio"
import "github.com/lesismal/nbio/logging"

import "net"
import "bytes"
import "encoding/hex"
import "encoding/json"
import "runtime"
import "strings"
import "math/big"
import "crypto/ecdsa"
import "crypto/elliptic"

//import "crypto/tls"
import "crypto/rand"
import "crypto/x509"
import "encoding/pem"

import "github.com/deroproject/derohe/globals"
import "github.com/deroproject/derohe/rpc"
import "github.com/deroproject/graviton"
import "github.com/go-logr/logr"

// this file implements the non-blocking job streamer
// only job is to stream jobs to thousands of workers, if any is successful,accept and report back

import "sync"

var memPool = sync.Pool{
	New: func() interface{} {
		return make([]byte, 16*1024)
	},
}

var logger_getwork logr.Logger
var (
	svr   *nbhttp.Server
	print = flag.Bool("print", false, "stdout output of echoed data")
)

type user_session struct {
	blocks        uint64
	miniblocks    uint64
	lasterr       string
	address       rpc.Address
	valid_address bool
	address_sum   [32]byte
}

var client_list_mutex sync.Mutex
var client_list = map[*websocket.Conn]*user_session{}

func CountMiners() int {
	client_list_mutex.Lock()
	defer client_list_mutex.Unlock()
	return len(client_list)
}

func SendJob() {

	var params rpc.GetBlockTemplate_Result

	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)

	// get a block template, and then we will fill the address here as optimization
	bl, mbl, _, _, err := chain.Create_new_block_template_mining(chain.IntegratorAddress())
	if err != nil {
		return
	}

	prev_hash := ""
	for i := range bl.Tips {
		prev_hash = prev_hash + bl.Tips[i].String()
	}

	params.JobID = fmt.Sprintf("%d.%d.%s", bl.Timestamp, 0, "notified")
	diff := chain.Get_Difficulty_At_Tips(bl.Tips)

	params.Height = bl.Height
	params.Prev_Hash = prev_hash
	params.Difficultyuint64 = diff.Uint64()
	params.Difficulty = diff.String()
	client_list_mutex.Lock()
	defer client_list_mutex.Unlock()

	for k, v := range client_list {
		if !mbl.Final { //write miners address only if possible
			copy(mbl.KeyHash[:], v.address_sum[:])
		}

		for i := range mbl.Nonce { // give each user different work
			mbl.Nonce[i] = globals.Global_Random.Uint32() // fill with randomness
		}

		if v.lasterr != "" {
			params.LastError = v.lasterr
			v.lasterr = ""
		}

		if !v.valid_address && !chain.IsAddressHashValid(false, v.address_sum) {
			params.LastError = "unregistered miner or you need to wait 15 mins"
		} else {
			v.valid_address = true
		}
		params.Blockhashing_blob = fmt.Sprintf("%x", mbl.Serialize())
		params.Blocks = v.blocks
		params.MiniBlocks = v.miniblocks

		encoder.Encode(params)
		k.WriteMessage(websocket.TextMessage, buf.Bytes())
		buf.Reset()

	}

}

func newUpgrader() *websocket.Upgrader {
	u := websocket.NewUpgrader()

	u.OnMessage(func(c *websocket.Conn, messageType websocket.MessageType, data []byte) {
		// echo
		c.WriteMessage(messageType, data)

		if messageType != websocket.TextMessage {
			return
		}

		sess := c.Session().(*user_session)

		client_list_mutex.Lock()
		client_list_mutex.Unlock()

		var p rpc.SubmitBlock_Params

		if err := json.Unmarshal(data, &p); err != nil {

		}

		mbl_block_data_bytes, err := hex.DecodeString(p.MiniBlockhashing_blob)
		if err != nil {
			//logger.Info("Submitting block could not be decoded")
			sess.lasterr = fmt.Sprintf("Submitted block could not be decoded. err: %s", err)
			return
		}

		var tstamp, extra uint64
		fmt.Sscanf(p.JobID, "%d.%d", &tstamp, &extra)

		_, blid, sresult, err := chain.Accept_new_block(tstamp, mbl_block_data_bytes)

		if sresult {
			//logger.Infof("Submitted block %s accepted", blid)
			if blid.IsZero() {
				sess.miniblocks++
			} else {
				sess.blocks++
			}
		}

	})
	u.OnClose(func(c *websocket.Conn, err error) {
		client_list_mutex.Lock()
		delete(client_list, c)
		client_list_mutex.Unlock()
	})

	return u
}

func onWebsocket(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, "/ws/") {
		http.NotFound(w, r)
		return
	}
	address := strings.TrimPrefix(r.URL.Path, "/ws/")

	addr, err := globals.ParseValidateAddress(address)
	if err != nil {
		fmt.Fprintf(w, "err: %s\n", err)
		return
	}
	addr_raw := addr.PublicKey.EncodeCompressed()

	upgrader := newUpgrader()
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		//panic(err)
		return
	}
	wsConn := conn.(*websocket.Conn)

	session := user_session{address: *addr, address_sum: graviton.Sum(addr_raw)}
	wsConn.SetSession(&session)

	client_list_mutex.Lock()
	client_list[wsConn] = &session
	client_list_mutex.Unlock()
}

func Getwork_server() {

	var err error

	logger_getwork = globals.Logger.WithName("GETWORK")

	logging.SetLevel(logging.LevelNone) //LevelDebug)//LevelNone)

	tlsConfig := &tls.Config{
		Certificates:       []tls.Certificate{generate_random_tls_cert()},
		InsecureSkipVerify: true,
	}

	mux := &http.ServeMux{}
	mux.HandleFunc("/", onWebsocket) // handle everything

	default_address := fmt.Sprintf("0.0.0.0:%d", globals.Config.GETWORK_Default_Port)

	if _, ok := globals.Arguments["--getwork-bind"]; ok && globals.Arguments["--getwork-bind"] != nil {
		addr, err := net.ResolveTCPAddr("tcp", globals.Arguments["--getwork-bind"].(string))
		if err != nil {
			logger_getwork.Error(err, "--getwork-bind address is invalid")
			return
		} else {
			if addr.Port == 0 {
				logger_getwork.Info("GETWORK server is disabled, No ports will be opened for miners to get work")
				return
			} else {
				default_address = addr.String()
			}
		}
	}

	logger_getwork.Info("GETWORK will listen", "address", default_address)

	svr = nbhttp.NewServer(nbhttp.Config{
		Name:                    "GETWORK",
		Network:                 "tcp",
		AddrsTLS:                []string{default_address},
		TLSConfig:               tlsConfig,
		Handler:                 mux,
		MaxLoad:                 10 * 1024,
		MaxWriteBufferSize:      32 * 1024,
		ReleaseWebsocketPayload: true,
		KeepaliveTime:           240 * time.Hour, // we expects all miners to find a block every 10 days,
		NPoller:                 runtime.NumCPU(),
	})

	svr.OnReadBufferAlloc(func(c *nbio.Conn) []byte {
		return memPool.Get().([]byte)
	})
	svr.OnReadBufferFree(func(c *nbio.Conn, b []byte) {
		memPool.Put(b)
	})

	globals.Cron.AddFunc("@every 2s", SendJob) // if daemon restart automaticaly send job

	if err = svr.Start(); err != nil {
		logger_getwork.Error(err, "nbio.Start failed.")
		return
	}
	logger.Info("GETWORK/Websocket server started")
	svr.Wait()
	defer svr.Stop()

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
		SerialNumber: big.NewInt(int64(time.Now().UnixNano())),

		// TODO do we need to add more parameters to make our certificate more authentic
		// and thwart traffic identification as a mass scale

		// you can add any attr that you need
		NotBefore: time.Now().AddDate(0, -1, 0),
		NotAfter:  time.Now().AddDate(1, 0, 0),
		// you have to generate a different serial number each execution
		/*
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
