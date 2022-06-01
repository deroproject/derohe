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

package main

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net"
	"runtime/debug"
	"strings"
	"time"

	"github.com/blang/semver/v4"
	"github.com/miekg/dns"
	"github.com/stratumfarm/derohe/config"
	"github.com/stratumfarm/derohe/globals"
)

//import "io/ioutil"
//import "net/http"

//import "crypto/tls"

/* this needs to be set on update.dero.io. as TXT record,  in encoded form as base64
 *
{ "version" : "1.0.2",
  "message" : "\n\n\u001b[32m This is a mandatory update\u001b[0m",
  "critical" : ""
}

base64 eyAidmVyc2lvbiIgOiAiMS4wLjIiLAogIm1lc3NhZ2UiIDogIlxuXG5cdTAwMWJbMzJtIFRoaXMgaXMgYSBtYW5kYXRvcnkgdXBkYXRlXHUwMDFiWzBtIiwgCiJjcml0aWNhbCIgOiAiIiAKfQ==



TXT record should be set as update=eyAidmVyc2lvbiIgOiAiMS4wLjIiLAogIm1lc3NhZ2UiIDogIlxuXG5cdTAwMWJbMzJtIFRoaXMgaXMgYSBtYW5kYXRvcnkgdXBkYXRlXHUwMDFiWzBtIiwgCiJjcml0aWNhbCIgOiAiIiAKfQ==
*/

func check_update_loop() {

	for {

		if config.DNS_NOTIFICATION_ENABLED {

			globals.Logger.V(2).Info("Checking update..")
			check_update()
		}
		time.Sleep(2 * 3600 * time.Second) // check every 2 hours
	}

}

// wrapper to make requests using proxy
func dialContextwrapper(ctx context.Context, network, address string) (net.Conn, error) {
	return globals.Dialer.Dial(network, address)
}

type socks_dialer net.Dialer

func (d *socks_dialer) Dial(network, address string) (net.Conn, error) {
	return globals.Dialer.Dial(network, address)
}

func (d *socks_dialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return globals.Dialer.Dial(network, address)
}

func dial_random_read_response(in []byte) (out []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			logger.V(2).Error(nil, "Recovered while checking updates", "r", r, "stack", debug.Stack())
		}
	}()

	// since we may be connecting through socks, grab the remote ip for our purpose rightnow
	//conn, err := globals.Dialer.Dial("tcp", "208.67.222.222:53")
	//conn, err := net.Dial("tcp", "8.8.8.8:53")
	random_feeder := rand.New(globals.NewCryptoRandSource())                          // use crypto secure resource
	server_address := config.DNS_servers[random_feeder.Intn(len(config.DNS_servers))] // choose a random server cryptographically
	conn, err := net.Dial("tcp", server_address)

	//conn, err := tls.Dial("tcp", remote_ip.String(),&tls.Config{InsecureSkipVerify: true})
	if err != nil {
		logger.V(2).Error(err, "Dial failed ")
		return
	}

	defer conn.Close() // close connection at end

	// upgrade connection TO TLS ( tls.Dial does NOT support proxy)
	//conn = tls.Client(conn, &tls.Config{InsecureSkipVerify: true})

	//rlog.Tracef(1, "Sending %d bytes", len(in))

	var buf [2]byte
	binary.BigEndian.PutUint16(buf[:], uint16(len(in)))
	conn.Write(buf[:]) // write length in bigendian format

	conn.Write(in) // write data

	// now we must wait for response to arrive
	var frame_length_buf [2]byte

	conn.SetReadDeadline(time.Now().Add(20 * time.Second))
	nbyte, err := io.ReadFull(conn, frame_length_buf[:])
	if err != nil || nbyte != 2 {
		// error while reading from connection we must disconnect it
		logger.V(2).Error(err, "Could not read DNS length prefix")
		return
	}

	frame_length := binary.BigEndian.Uint16(frame_length_buf[:])
	if frame_length == 0 {
		// most probably memory DDOS attack, kill the connection
		logger.V(2).Error(nil, "Frame length is too small")
		return
	}

	out = make([]byte, frame_length)

	conn.SetReadDeadline(time.Now().Add(20 * time.Second))
	data_size, err := io.ReadFull(conn, out)
	if err != nil || data_size <= 0 || uint16(data_size) != frame_length {
		// error while reading from connection we must kiil it
		//rlog.Warnf("Could not read  DNS data size  read %d, frame length %d err %s", data_size, frame_length, err)
		logger.V(2).Error(err, "Could not read  DNS data")
		return

	}
	out = out[:frame_length]

	return
}

func check_update() {

	// add panic handler, in case DNS acts rogue and tries to attack
	defer func() {
		if r := recover(); r != nil {
			logger.V(2).Error(nil, "Recovered while checking updates", r, "r", "stack", debug.Stack())
		}
	}()

	if !config.DNS_NOTIFICATION_ENABLED { // if DNS notifications are disabled bail out
		return
	}

	/*  var u update_message
	    u.Version = "2.0.0"
	    u.Message = "critical msg txt\x1b[35m should \n be in RED"

	     globals.Logger.Infof("testing %s",u.Message)

	     j,err := json.Marshal(u)
	     globals.Logger.Infof("json format %s err %s",j,err)
	*/
	/*extract_parse_version("update=eyAidmVyc2lvbiIgOiAiMS4xLjAiLCAibWVzc2FnZSIgOiAiXG5cblx1MDAxYlszMm0gVGhpcyBpcyBhIG1hbmRhdG9yeSB1cGdyYWRlIHBsZWFzZSB1cGdyYWRlIGZyb20geHl6IFx1MDAxYlswbSIsICJjcml0aWNhbCIgOiAiIiB9")

	  return
	*/

	m1 := new(dns.Msg)
	// m1.SetEdns0(65000, true), dnssec probably leaks current timestamp, it's disabled until more invetigation
	m1.Id = dns.Id()
	m1.RecursionDesired = true
	m1.Question = make([]dns.Question, 1)
	m1.Question[0] = dns.Question{Name: config.DNS_UPDATE_CHECK, Qtype: dns.TypeTXT, Qclass: dns.ClassINET}

	packed, err := m1.Pack()
	if err != nil {
		globals.Logger.V(2).Error(err, "Error which packing DNS query for program update")
		return
	}

	/*

			// setup a http client
			httpTransport := &http.Transport{}
			httpClient := &http.Client{Transport: httpTransport}
			// set our socks5 as the dialer
			httpTransport.Dial = globals.Dialer.Dial



		        packed_base64:= base64.RawURLEncoding.EncodeToString(packed)
		response, err := httpClient.Get("https://1.1.1.1/dns-query?ct=application/dns-udpwireformat&dns="+packed_base64)

		_ = packed_base64

		if err != nil {
		    rlog.Warnf("error making DOH request err %s",err)
		    return
		}

		defer response.Body.Close()
		        contents, err := ioutil.ReadAll(response.Body)
		        if err != nil {
		            rlog.Warnf("error reading DOH response err %s",err)
		            return
		}
	*/

	contents, err := dial_random_read_response(packed)
	if err != nil {
		logger.V(2).Error(err, "error reading response from DNS server")
		return

	}

	//rlog.Debugf("DNS response length from DNS server %d bytes", len(contents))

	err = m1.Unpack(contents)
	if err != nil {
		logger.V(2).Error(err, "error decoding DOH response")
		return

	}

	for i := range m1.Answer {
		if t, ok := m1.Answer[i].(*dns.TXT); ok {

			// replace any spaces so as records could be joined

			logger.V(2).Info("Processing record ", "record", t.Txt)
			joined := strings.Join(t.Txt, "")
			extract_parse_version(joined)

		}
	}

	//globals.Logger.Infof("response %+v err ",m1,err)

}

type update_message struct {
	Version  string `json:"version"`
	Message  string `json:"message"`
	Critical string `json:"critical"` // always broadcasted, without checks for version
}

// our version are TXT record of following format
// version=base64 encoded json
func extract_parse_version(str string) {

	strl := strings.ToLower(str)
	if !strings.HasPrefix(strl, "update=") {
		logger.V(2).Info("Skipping record", "record", str)
		return
	}

	parts := strings.SplitN(str, "=", 2)
	if len(parts) != 2 {
		return
	}

	data, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		logger.V(2).Error(err, "Could NOT decode base64 update message", "data", parts[1])
		return
	}

	var u update_message
	err = json.Unmarshal(data, &u)

	//globals.Logger.Infof("data %+v", u)

	if err != nil {
		logger.V(2).Error(err, "Could NOT decode json update message")
		return
	}

	uversion, err := semver.ParseTolerant(u.Version)
	if err != nil {
		logger.V(2).Error(err, "Could NOT update version")
	}

	current_version := config.Version
	current_version.Pre = current_version.Pre[:0]
	current_version.Build = current_version.Build[:0]

	// give warning to update the daemon
	if u.Message != "" && err == nil { // check semver
		if current_version.LT(uversion) {
			if current_version.Major != uversion.Major { // if major version is different give extract warning
				logger.Info("\033[31m CRITICAL MAJOR update, please upgrade ASAP.\033[0m")
			}

			logger.Info(fmt.Sprintf("%s", u.Message)) // give the version upgrade message
			logger.Info(fmt.Sprintf("\033[33mCurrent Version %s \033[32m-> Upgrade Version %s\033[0m ", current_version.String(), uversion.String()))
		}
	}

	if u.Critical != "" { // give the critical upgrade message
		logger.Info(fmt.Sprintf("%s", u.Critical))
	}

}
