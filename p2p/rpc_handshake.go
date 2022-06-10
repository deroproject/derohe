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
import "bytes"
import "context"

import "sync/atomic"
import "time"

import "github.com/deroproject/derohe/config"
import "github.com/deroproject/derohe/globals"
import "github.com/blang/semver/v4"

// verify incoming handshake for number of checks such as mainnet/testnet etc etc
func Verify_Handshake(handshake *Handshake_Struct) bool {
	v := semver.MustParse(handshake.DaemonVersion)

	if v.Major >= 3 && v.Minor >= 5 && v.Patch >= 0 {

	} else {
		var pre int
		fmt.Sscanf(v.Pre[0].String(), "%d", &pre) // make sure previous releases can connect
		if pre < 88 {
			return false
		}
	}

	return bytes.Equal(handshake.Network_ID[:], globals.Config.Network_ID[:])
}

func (handshake *Handshake_Struct) Fill() {
	fill_common(&handshake.Common) // fill common info

	handshake.ProtocolVersion = "1.0.0"
	handshake.DaemonVersion = config.Version.String()
	handshake.Tag = node_tag
	handshake.UTC_Time = int64(time.Now().UTC().Unix()) // send our UTC time
	handshake.Local_Port = uint32(P2P_Port)             // export requested or default port
	handshake.Peer_ID = GetPeerID()                     // give our randomly generated peer id
	handshake.Pruned = chain.LocatePruneTopo()

	//	handshake.Flags = // add any flags necessary

	copy(handshake.Network_ID[:], globals.Config.Network_ID[:])
}

// this is used only once
// all clients start with handshake, then other party sends avtive to mark that connection is active
func (connection *Connection) dispatch_test_handshake() {
	defer handle_connection_panic(connection)
	var request, response Handshake_Struct
	request.Fill()

	//scan our peer list and send peers which have been recently communicated
	request.PeerList = get_peer_list_specific(Address(connection))

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	if err := connection.Client.CallWithContext(ctx, "Peer.Handshake", request, &response); err != nil {
		connection.logger.V(4).Error(err, "cannot handshake")
		connection.exit()
		return
	}

	if !Verify_Handshake(&response) { // if not same network boot off
		connection.logger.V(3).Info("terminating connection network id mismatch ", "networkid", response.Network_ID)
		connection.exit()
		return
	}
	connection.update(&response.Common) // update common information
	if !Connection_Add(connection) {    // add connection to pool
		connection.exit()
		return
	}

	if len(response.ProtocolVersion) < 128 {
		connection.ProtocolVersion = response.ProtocolVersion
	}

	if len(response.DaemonVersion) < 128 {
		connection.DaemonVersion = response.DaemonVersion
	}
	connection.Port = response.Local_Port
	connection.Peer_ID = response.Peer_ID
	if len(response.Tag) < 128 {
		connection.Tag = response.Tag
	}
	if response.Pruned >= 1 {
		connection.Pruned = response.Pruned
	}

	// TODO we must also add the peer to our list
	// which can be distributed to other peers
	if connection.Port != 0 && connection.Port <= 65535 { // peer is saying it has an open port, handshake is success so add peer

		var p Peer
		if net.ParseIP(Address(connection)).To4() != nil { // if ipv4
			p.Address = fmt.Sprintf("%s:%d", Address(connection), connection.Port)
		} else { // if ipv6
			p.Address = fmt.Sprintf("[%s]:%d", Address(connection), connection.Port)
		}
		p.ID = connection.Peer_ID

		p.LastConnected = uint64(time.Now().UTC().Unix())

		Peer_Add(&p)
	}

	// parse delivered peer list as grey list
	connection.logger.V(4).Info("Peer provides peers", "count", len(response.PeerList))
	for i := range response.PeerList {
		if i < 13 {
			Peer_Add(&Peer{Address: response.PeerList[i].Addr, LastConnected: uint64(time.Now().UTC().Unix())})
		}
	}

	atomic.StoreUint32(&connection.State, ACTIVE)
}

// used to ping pong
func (c *Connection) Ping(request Dummy, response *Dummy) error {
	defer handle_connection_panic(c)
	fill_common_T1(&request.Common)
	c.update(&request.Common)                             // update common information
	fill_common(&response.Common)                         // fill common info
	fill_common_T0T1T2(&request.Common, &response.Common) // fill time related information
	return nil
}

// serves handhake requests
func (c *Connection) Handshake(request Handshake_Struct, response *Handshake_Struct) error {
	defer handle_connection_panic(c)
	if request.Peer_ID == GetPeerID() { // check if self connection exit
		//rlog.Tracef(1, "Same peer ID, probably self connection, disconnecting from this client")
		c.exit()
		return fmt.Errorf("Same peer ID")
	}

	if !Verify_Handshake(&request) { // if not same network boot off
		logger.V(2).Info("kill connection network id mismatch peer network id.", "Network_ID", request.Network_ID)
		c.exit()
		return fmt.Errorf("NID mismatch")
	}

	response.Fill()

	c.update(&request.Common) // update common information
	if c.State == ACTIVE {
		for i := range request.PeerList {
			if i < 31 {
				Peer_Add(&Peer{Address: request.PeerList[i].Addr, LastConnected: uint64(time.Now().UTC().Unix())})
			}
		}
	}
	if !c.Incoming {
		Peer_SetSuccess(c.Addr.String())
	}

	return nil
}
