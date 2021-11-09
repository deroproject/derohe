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

/* this file implements the connection pool manager, keeping a list of active connections etc
 * this will also ensure that a single IP is connected only once
 *
 */
import "fmt"
import "net"
import "math"
import "sync"
import "sort"
import "time"
import "strings"
import "math/rand"
import "sync/atomic"
import "runtime/debug"
import "encoding/binary"

import "github.com/go-logr/logr"

import "github.com/dustin/go-humanize"

import "github.com/paulbellamy/ratecounter"

import "github.com/deroproject/derohe/block"
import "github.com/deroproject/derohe/cryptography/crypto"
import "github.com/deroproject/derohe/globals"
import "github.com/deroproject/derohe/metrics"
import "github.com/deroproject/derohe/transaction"

// any connection incoming/outgoing can only be in this state
//type Conn_State uint32

const (
	HANDSHAKE_PENDING uint32 = 0 // "Pending"
	IDLE                     = 1 // "Idle"
	ACTIVE                   = 2 // "Active"
)

type Queued_Command struct {
	Command uint64 // we are waiting for this response
	BLID    []crypto.Hash
	TXID    []crypto.Hash
	Topos   []int64
}

const MAX_CLOCK_DATA_SET = 16

// This structure is used to do book keeping for the connection and keeps other DATA related to peer
// golang restricts 64 bit uint64/int atomic on a 64 bit boundary
// therefore all atomics are on the top
type Connection struct {
	Height       int64       // last height sent by peer  ( first member alignments issues)
	StableHeight int64       // last stable height
	TopoHeight   int64       // topo height, current topo height, this is the only thing we require for syncing
	StateHash    crypto.Hash // statehash at the top
	Pruned       int64       // till where chain has been pruned on this node

	LastObjectRequestTime int64  // when was the last item placed in object list
	BytesIn               uint64 // total bytes in
	BytesOut              uint64 // total bytes out
	Latency               int64  // time.Duration            // latency to this node when sending timed sync

	Incoming          bool              // is connection incoming or outgoing
	Addr              *net.TCPAddr      // endpoint on the other end
	Port              uint32            // port advertised by other end as its server,if it's 0 server cannot accept connections
	Peer_ID           uint64            // Remote peer id
	SyncNode          bool              // whether the peer has been added to command line as sync node
	Top_Version       uint64            // current hard fork version supported by peer
	TXpool_cache      map[uint64]uint32 // used for ultra blocks in miner mode,cache where we keep TX which have been broadcasted to this peer
	TXpool_cache_lock sync.RWMutex
	ProtocolVersion   string
	Tag               string // tag for the other end
	DaemonVersion     string
	//Exit                  chan bool   // Exit marker that connection needs to be killed
	ExitCounter           int32
	State                 uint32       // state of the connection
	Top_ID                crypto.Hash  // top block id of the connection
	Cumulative_Difficulty string       // cumulative difficulty of top block of peer, this is NOT required
	CDIFF                 atomic.Value //*big.Int    // NOTE: this field is used internally and is the parsed from Cumulative_Difficulty

	logger            logr.Logger     // connection specific logger
	logid             string          // formatted version of connection
	Requested_Objects [][32]byte      // currently unused as we sync up with a single peer at a time
	Conn              net.Conn        // actual object to talk
	RConn             *RPC_Connection // object  for communication
	//	Command_queue     *list.List               // New protocol is partly syncronous
	Objects      chan Queued_Command      // contains all objects that are requested
	SpeedIn      *ratecounter.RateCounter // average speed in last 60 seconds
	SpeedOut     *ratecounter.RateCounter // average speed in last 60 secs
	request_time atomic.Value             //time.Time                // used to track latency
	writelock    sync.Mutex               // used to Serialize writes

	peer_sent_time time.Time // contains last time when peerlist was sent

	clock_index   int
	clock_offsets [MAX_CLOCK_DATA_SET]time.Duration
	delays        [MAX_CLOCK_DATA_SET]time.Duration
	clock_offset  int64 // duration updated on every miniblock

	Mutex sync.Mutex // used only by connection go routine
}

func (c *Connection) exit() {
	c.RConn.Session.Close()

}

var connection_map sync.Map                      // map[string]*Connection{}
var connection_per_ip_counter = map[string]int{} // only keeps the counter of counter of connections

// for incoming connections we use their peer id to assertain uniquenesss
// for outgoing connections, we use the tcp endpoint address, so as not more than 1 connection is done
func Key(c *Connection) string {
	if c.Incoming {
		return fmt.Sprintf("%d", c.Peer_ID)
	}
	return string(c.Addr.String()) // Simple []byte => string conversion
}

// check whether an IP is in the map already
func IsAddressConnected(address string) bool {

	if _, ok := connection_map.Load(strings.TrimSpace(address)); ok {
		return true
	}
	return false
}

// add connection to  map
// we also check for limits for incoming connections
// same ip max 8 ip ( considering NAT)
//same Peer ID   4
func Connection_Add(c *Connection) {
	//connection_mutex.Lock()
	//defer connection_mutex.Unlock()

	ip_count := 0
	peer_id_count := 0

	incoming_ip := c.Addr.IP.String()
	incoming_peer_id := c.Peer_ID

	if c.Incoming { // we need extra protection for incoming for various attacks

		connection_map.Range(func(k, value interface{}) bool {
			v := value.(*Connection)
			if v.Incoming {
				if incoming_ip == v.Addr.IP.String() {
					ip_count++
				}

				if incoming_peer_id == v.Peer_ID {
					peer_id_count++
				}
			}
			return true
		})

	}

	if ip_count >= 8 || peer_id_count >= 4 {
		c.logger.V(3).Info("IP address already has too many connections, exiting this connection", "ip", incoming_ip, "count", ip_count, "peerid", incoming_peer_id)
		c.exit()
		return
	}

	connection_map.Store(Key(c), c)
}

// unique connection list
// since 2 nodes may be connected in both directions, we need to deliver new blocks/tx to only one
// thereby saving NW/computing costs
// we find duplicates using peer id
func UniqueConnections() map[uint64]*Connection {
	unique_map := map[uint64]*Connection{}
	connection_map.Range(func(k, value interface{}) bool {
		v := value.(*Connection)
		if atomic.LoadUint32(&v.State) != HANDSHAKE_PENDING && GetPeerID() != v.Peer_ID { //and skip ourselves
			unique_map[v.Peer_ID] = v // map will automatically deduplicate/overwrite previous
		}
		return true
	})
	return unique_map
}

// this function has infinite loop to keep ping every few sec
func ping_loop() {
	for {
		time.Sleep(1 * time.Second)
		connection_map.Range(func(k, value interface{}) bool {
			c := value.(*Connection)
			if atomic.LoadUint32(&c.State) != HANDSHAKE_PENDING && GetPeerID() != c.Peer_ID {
				go func() {
					defer globals.Recover(3)
					var request, response Dummy
					fill_common(&request.Common) // fill common info

					if c.peer_sent_time.Add(10 * time.Minute).Before(time.Now()) {
						c.peer_sent_time = time.Now()
						request.Common.PeerList = get_peer_list()
					}
					if err := c.RConn.Client.Call("Peer.Ping", request, &response); err != nil {
						return
					}
					c.update(&response.Common) // update common information
				}()
			}
			return true
		})
	}
}

// add connection to  map
func Connection_Delete(c *Connection) {
	connection_map.Delete(Key(c))
}

// prints all the connection info to screen
func Connection_Print() {
	var clist []*Connection

	connection_map.Range(func(k, value interface{}) bool {
		v := value.(*Connection)
		clist = append(clist, v)
		return true
	})

	logger.Info("Connection info for peers", "count", len(clist))

	if globals.Arguments["--debug"].(bool) == true {
		fmt.Printf("%-20s %-16s %-5s %-7s %-7s %-7s %23s %3s %5s %s %s %s %s %16s %16s\n", "Remote Addr", "PEER ID", "PORT", " State", "Latency", "Offset", "S/H/T", "DIR", "QUEUE", "     IN", "    OUT", " IN SPEED", " OUT SPEED", "Version", "Statehash")
	} else {
		fmt.Printf("%-20s %-16s %-5s %-7s %-7s %-7s %17s %3s %5s %s %s %s %s %16s %16s\n", "Remote Addr", "PEER ID", "PORT", " State", "Latency", "Offset", "H/T", "DIR", "QUEUE", "     IN", "    OUT", " IN SPEED", " OUT SPEED", "Version", "Statehash")

	}

	// sort the list
	sort.Slice(clist, func(i, j int) bool { return clist[i].Addr.String() < clist[j].Addr.String() })

	our_topo_height := chain.Load_TOPO_HEIGHT()

	for i := range clist {

		// skip pending  handshakes and skip ourselves
		if atomic.LoadUint32(&clist[i].State) == HANDSHAKE_PENDING || GetPeerID() == clist[i].Peer_ID {
			//	continue
		}

		dir := "OUT"
		if clist[i].Incoming {
			dir = "INC"
		}
		state := "PENDING"
		if atomic.LoadUint32(&clist[i].State) == IDLE {
			state = "IDLE"
		} else if atomic.LoadUint32(&clist[i].State) == ACTIVE {
			state = "ACTIVE"
		}

		version := clist[i].DaemonVersion

		if len(version) > 20 {
			version = version[:20]
		}

		tag := clist[i].Tag
		if len(tag) > 20 {
			tag = tag[:20]
		}

		var color_yellow = "\033[33m"
		var color_normal = "\033[0m"

		//if our_height is more than
		if our_topo_height > clist[i].TopoHeight {
			fmt.Print(color_yellow)
		}

		if globals.Arguments["--debug"].(bool) == true {
			hstring := fmt.Sprintf("%d/%d/%d", clist[i].StableHeight, clist[i].Height, clist[i].TopoHeight)
			fmt.Printf("%-20s %16x %5d %7s %7s %7s %23s %s %5d %7s %7s %8s %9s     %16s %s %x\n", clist[i].Addr.IP, clist[i].Peer_ID, clist[i].Port, state, time.Duration(atomic.LoadInt64(&clist[i].Latency)).Round(time.Millisecond).String(), time.Duration(atomic.LoadInt64(&clist[i].clock_offset)).Round(time.Millisecond).String(), hstring, dir, clist[i].isConnectionSyncing(), humanize.Bytes(atomic.LoadUint64(&clist[i].BytesIn)), humanize.Bytes(atomic.LoadUint64(&clist[i].BytesOut)), humanize.Bytes(uint64(clist[i].SpeedIn.Rate()/60)), humanize.Bytes(uint64(clist[i].SpeedOut.Rate()/60)), version, tag, clist[i].StateHash[:])

		} else {
			hstring := fmt.Sprintf("%d/%d", clist[i].Height, clist[i].TopoHeight)
			fmt.Printf("%-20s %16x %5d %7s %7s %7s %17s %s %5d %7s %7s %8s %9s     %16s %s %x\n", clist[i].Addr.IP, clist[i].Peer_ID, clist[i].Port, state, time.Duration(atomic.LoadInt64(&clist[i].Latency)).Round(time.Millisecond).String(), time.Duration(atomic.LoadInt64(&clist[i].clock_offset)).Round(time.Millisecond).String(), hstring, dir, clist[i].isConnectionSyncing(), humanize.Bytes(atomic.LoadUint64(&clist[i].BytesIn)), humanize.Bytes(atomic.LoadUint64(&clist[i].BytesOut)), humanize.Bytes(uint64(clist[i].SpeedIn.Rate()/60)), humanize.Bytes(uint64(clist[i].SpeedOut.Rate()/60)), version, tag, clist[i].StateHash[:8])

		}

		fmt.Print(color_normal)
	}

}

// for continuos update on command line, get the maximum height of all peers
// show the average network status
func Best_Peer_Height() (best_height, best_topo_height int64) {

	var heights []uint64
	var topoheights []uint64

	connection_map.Range(func(k, value interface{}) bool {
		v := value.(*Connection)
		if atomic.LoadUint32(&v.State) != HANDSHAKE_PENDING {
			height := atomic.LoadInt64(&v.Height)
			heights = append(heights, uint64(height))
			topoheights = append(topoheights, uint64(atomic.LoadInt64(&v.TopoHeight)))
		}
		return true
	})

	best_height = int64(Median(heights))
	best_topo_height = int64(Median(topoheights))

	return
}

// this function return peer count which have successful handshake
func Disconnect_All() (Count uint64) {
	return
	/*
		connection_mutex.Lock()
		for _, v := range connection_map {
			// v.Lock()
			close(v.Exit) // close the connection
			//v.Unlock()
		}
		connection_mutex.Unlock()
		return
	*/
}

// this function return peer count which have successful handshake
func Peer_Count() (Count uint64) {
	connection_map.Range(func(k, value interface{}) bool {
		v := value.(*Connection)
		if atomic.LoadUint32(&v.State) != HANDSHAKE_PENDING && GetPeerID() != v.Peer_ID {
			Count++
		}
		return true
	})
	return
}

// this function returnw random connection which have successful handshake
func Random_Connection(height int64) (c *Connection) {

	var clist []*Connection

	connection_map.Range(func(k, value interface{}) bool {
		v := value.(*Connection)
		if atomic.LoadInt64(&v.Height) >= height {
			clist = append(clist, v)
		}
		return true
	})

	if len(clist) > 0 {
		return clist[rand.Int()%len(clist)]
	}

	return nil
}

// this returns count of peers in both directions
func Peer_Direction_Count() (Incoming uint64, Outgoing uint64) {

	connection_map.Range(func(k, value interface{}) bool {
		v := value.(*Connection)
		if atomic.LoadUint32(&v.State) != HANDSHAKE_PENDING && GetPeerID() != v.Peer_ID {
			if v.Incoming {
				Incoming++
			} else {
				Outgoing++
			}
		}
		return true
	})

	return
}

func Broadcast_Block(cbl *block.Complete_Block, PeerID uint64) {

	//Broadcast_Block_Ultra(cbl,PeerID)

	Broadcast_Block_Coded(cbl, PeerID)

}

// broad cast a block to all connected peers
// we can only broadcast a block which is in our db
// this function is trigger from 2 points, one when we receive a unknown block which can be successfully added to chain
// second from the blockchain which has to relay locally  mined blocks as soon as possible
func Broadcast_Block_Ultra(cbl *block.Complete_Block, PeerID uint64) { // if peerid is provided it is skipped
	var cblock_serialized Complete_Block

	defer globals.Recover(3)

	/*if IsSyncing() { // if we are syncing, do NOT broadcast the block
		return
	}*/

	cblock_serialized.Block = cbl.Bl.Serialize()

	for i := range cbl.Txs {
		cblock_serialized.Txs = append(cblock_serialized.Txs, cbl.Txs[i].Serialize())
	}

	our_height := chain.Get_Height()
	// build the request once and dispatch it to all possible peers
	count := 0
	unique_map := UniqueConnections()

	for _, v := range unique_map {
		select {
		case <-Exit_Event:
			return
		default:
		}
		if atomic.LoadUint32(&v.State) != HANDSHAKE_PENDING && PeerID != v.Peer_ID { // skip pre-handshake connections

			// if the other end is > 50 blocks behind, do not broadcast block to hime
			// this is an optimisation, since if the other end is syncing
			// every peer will keep on broadcasting and thus making it more lagging
			// due to overheads
			peer_height := atomic.LoadInt64(&v.Height)
			if (our_height - peer_height) > 25 {
				continue
			}

			count++
			go func(connection *Connection) {
				defer globals.Recover(3)

				{ // everyone needs ultra compact block if possible
					var peer_specific_block Objects

					var cblock Complete_Block
					cblock.Block = cblock_serialized.Block

					sent := 0
					skipped := 0
					connection.TXpool_cache_lock.RLock()
					for i := range cbl.Bl.Tx_hashes {
						// in ultra compact mode send a transaction only if we know that we have not sent that transaction earlier
						// send only tx not found in cache

						if _, ok := connection.TXpool_cache[binary.LittleEndian.Uint64(cbl.Bl.Tx_hashes[i][:])]; !ok {
							cblock.Txs = append(cblock.Txs, cblock_serialized.Txs[i])
							sent++

						} else {
							skipped++
						}

					}
					connection.TXpool_cache_lock.RUnlock()

					connection.logger.V(3).Info("Sending ultra block to peer ", "total", len(cbl.Bl.Tx_hashes), "tx skipped", skipped, "sent", sent)
					peer_specific_block.CBlocks = append(peer_specific_block.CBlocks, cblock)
					var dummy Dummy
					fill_common(&peer_specific_block.Common) // fill common info
					if err := connection.RConn.Client.Call("Peer.NotifyBlock", peer_specific_block, &dummy); err != nil {
						return
					}
					connection.update(&dummy.Common) // update common information

				}
			}(v)
		}

	}
	//rlog.Infof("Broadcasted block %s to %d peers", cbl.Bl.GetHash(), count)
}

// broad cast a block to all connected peers in cut up in chunks with erasure coding
// we can only broadcast a block which is in our db
// this function is trigger from 2 points, one when we receive a unknown block which can be successfully added to chain
// second from the blockchain which has to relay locally  mined blocks as soon as possible
func Broadcast_Block_Coded(cbl *block.Complete_Block, PeerID uint64) { // if peerid is provided it is skipped
	broadcast_Block_Coded(cbl, PeerID, globals.Time().UTC().UnixMicro())
}

func broadcast_Block_Coded(cbl *block.Complete_Block, PeerID uint64, first_seen int64) {

	defer globals.Recover(3)

	/*if IsSyncing() { // if we are syncing, do NOT broadcast the block
		return
	}*/

	blid := cbl.Bl.GetHash()

	hhash, chunk_count := convert_block_to_chunks(cbl, 16, 32)

	our_height := chain.Get_Height()
	// build the request once and dispatch it to all possible peers
	count := 0
	unique_map := UniqueConnections()

	for { // we must send all blocks atleast once, once we are done, break ut
		old_count := count
		for _, v := range unique_map {
			select {
			case <-Exit_Event:
				return
			default:
			}
			if atomic.LoadUint32(&v.State) != HANDSHAKE_PENDING && PeerID != v.Peer_ID { // skip pre-handshake connections

				// if the other end is > 50 blocks behind, do not broadcast block to hime
				// this is an optimisation, since if the other end is syncing
				// every peer will keep on broadcasting and thus making it more lagging
				// due to overheads
				peer_height := atomic.LoadInt64(&v.Height)
				if (our_height - peer_height) > 25 {
					continue
				}

				if count > chunk_count {
					goto done
				}

				go func(connection *Connection, cid int) {
					defer globals.Recover(3)
					var peer_specific_list ObjectList

					var chunkid [32 + 1 + 32]byte
					copy(chunkid[:], blid[:])
					chunkid[32] = byte(cid % chunk_count)
					copy(chunkid[33:], hhash[:])
					peer_specific_list.Sent = first_seen
					peer_specific_list.Chunk_list = append(peer_specific_list.Chunk_list, chunkid)
					connection.logger.V(3).Info("Sending erasure coded chunk to peer ", "cid", cid)
					var dummy Dummy
					fill_common(&peer_specific_list.Common) // fill common info
					if err := connection.RConn.Client.Call("Peer.NotifyINV", peer_specific_list, &dummy); err != nil {
						return
					}
					connection.update(&dummy.Common) // update common information

				}(v, count)
				count++
			}
		}
		if old_count == count { // exit the loop
			break
		}
		old_count = count

	}

done:

	//rlog.Infof("Broadcasted block %s to %d peers", cbl.Bl.GetHash(), count)

}

// broad cast a block to all connected peers in cut up in chunks with erasure coding
// we can only broadcast a block which is in our db
// this function is triggerred from 2 points, one when we receive a unknown block which can be successfully added to chain
// second from the blockchain which has to relay locally  mined blocks as soon as possible
func broadcast_Chunk(chunk *Block_Chunk, PeerID uint64, first_seen int64) { // if peerid is provided it is skipped

	defer globals.Recover(3)

	/*if IsSyncing() { // if we are syncing, do NOT broadcast the block
		return
	}*/

	our_height := chain.Get_Height()
	// build the request once and dispatch it to all possible peers
	count := 0
	unique_map := UniqueConnections()

	chash := chunk.HeaderHash()

	for _, v := range unique_map {
		select {
		case <-Exit_Event:
			return
		default:
		}
		if atomic.LoadUint32(&v.State) != HANDSHAKE_PENDING && PeerID != v.Peer_ID { // skip pre-handshake connections

			// if the other end is > 50 blocks behind, do not broadcast block to hime
			// this is an optimisation, since if the other end is syncing
			// every peer will keep on broadcasting and thus making it more lagging
			// due to overheads
			peer_height := atomic.LoadInt64(&v.Height)
			if (our_height - peer_height) > 25 {
				continue
			}

			count++

			go func(connection *Connection) {
				defer globals.Recover(3)
				var peer_specific_list ObjectList

				var chunkid [33 + 32]byte
				copy(chunkid[:], chunk.BLID[:])
				chunkid[32] = byte(chunk.CHUNK_ID)
				copy(chunkid[33:], chash[:])
				peer_specific_list.Sent = first_seen

				peer_specific_list.Chunk_list = append(peer_specific_list.Chunk_list, chunkid)
				connection.logger.V(3).Info("Sending erasure coded chunk to peer ", "cid", chunk.CHUNK_ID)
				var dummy Dummy
				fill_common(&peer_specific_list.Common) // fill common info
				if err := connection.RConn.Client.Call("Peer.NotifyINV", peer_specific_list, &dummy); err != nil {
					return
				}
				connection.update(&dummy.Common) // update common information
			}(v)

		}
	}
}

// broad cast a block to all connected peers
// we can only broadcast a block which is in our db
// this function is trigger from 2 points, one when we receive a unknown block which can be successfully added to chain
// second from the blockchain which has to relay locally  mined blocks as soon as possible
func Broadcast_MiniBlock(mbl block.MiniBlock, PeerID uint64) { // if peerid is provided it is skipped
	broadcast_MiniBlock(mbl, PeerID, globals.Time().UTC().UnixMicro())
}
func broadcast_MiniBlock(mbl block.MiniBlock, PeerID uint64, first_seen int64) { // if peerid is provided it is skipped

	defer globals.Recover(3)

	miniblock_serialized := mbl.Serialize()

	var peer_specific_block Objects
	peer_specific_block.MiniBlocks = append(peer_specific_block.MiniBlocks, miniblock_serialized)
	fill_common(&peer_specific_block.Common) // fill common info
	peer_specific_block.Sent = first_seen

	our_height := chain.Get_Height()
	// build the request once and dispatch it to all possible peers
	count := 0
	unique_map := UniqueConnections()

	for _, v := range unique_map {
		select {
		case <-Exit_Event:
			return
		default:
		}
		if atomic.LoadUint32(&v.State) != HANDSHAKE_PENDING && PeerID != v.Peer_ID { // skip pre-handshake connections

			// if the other end is > 50 blocks behind, do not broadcast block to hime
			// this is an optimisation, since if the other end is syncing
			// every peer will keep on broadcasting and thus making it more lagging
			// due to overheads
			peer_height := atomic.LoadInt64(&v.Height)
			if (our_height - peer_height) > 25 {
				continue
			}

			count++
			go func(connection *Connection) {
				defer globals.Recover(3)
				connection.logger.V(4).Info("Sending mini block to peer ")
				var dummy Dummy
				if err := connection.RConn.Client.Call("Peer.NotifyMiniBlock", peer_specific_block, &dummy); err != nil {
					return
				}
				connection.update(&dummy.Common) // update common information
			}(v)
		}

	}
	//rlog.Infof("Broadcasted block %s to %d peers", cbl.Bl.GetHash(), count)
}

// broadcast a new transaction, return to how many peers the transaction has been broadcasted
// this function is trigger from 2 points, one when we receive a unknown tx
// second from the mempool which may want to relay local ot soon going to expire transactions

func Broadcast_Tx(tx *transaction.Transaction, PeerID uint64) (relayed_count int32) {
	return broadcast_Tx(tx, PeerID, globals.Time().UTC().UnixMicro())

}
func broadcast_Tx(tx *transaction.Transaction, PeerID uint64, sent int64) (relayed_count int32) {
	defer globals.Recover(3)

	var request ObjectList
	fill_common_skip_topoheight(&request.Common) // fill common info, but skip topo height
	txhash := tx.GetHash()

	request.Tx_list = append(request.Tx_list, txhash)
	request.Sent = sent

	our_height := chain.Get_Height()

	unique_map := UniqueConnections()

	for _, v := range unique_map {
		select {
		case <-Exit_Event:
			return
		default:
		}
		if atomic.LoadUint32(&v.State) != HANDSHAKE_PENDING && PeerID != v.Peer_ID { // skip pre-handshake connections

			// if the other end is > 50 blocks behind, do not broadcast block to hime
			// this is an optimisation, since if the other end is syncing
			// every peer will keep on broadcasting and thus making it more lagging
			// due to overheads
			// if we are lagging or peer is lagging, do not brodcast transactions
			peer_height := atomic.LoadInt64(&v.Height)
			if (our_height-peer_height) > 25 || (our_height+5) < peer_height {
				continue
			}

			go func(connection *Connection) {
				defer func() {
					if r := recover(); r != nil {
						connection.logger.V(1).Error(r.(error), "Recovere3d while sending tx", "stack", debug.Stack())
					}
				}()

				resend := true
				// disable cache if not possible due to options
				// assuming the peer is good, he would like to obtain the tx ASAP

				connection.TXpool_cache_lock.Lock()
				if _, ok := connection.TXpool_cache[binary.LittleEndian.Uint64(txhash[:])]; !ok {
					connection.TXpool_cache[binary.LittleEndian.Uint64(txhash[:])] = uint32(time.Now().Unix())
					resend = true
				} else {
					resend = false
				}
				connection.TXpool_cache_lock.Unlock()

				if resend {
					var dummy Dummy
					fill_common(&dummy.Common) // fill common info
					if err := connection.RConn.Client.Call("Peer.NotifyINV", request, &dummy); err != nil {
						return
					}
					connection.update(&dummy.Common) // update common information

					atomic.AddInt32(&relayed_count, 1)
				}

			}(v)
		}

	}
	if relayed_count > 0 {
		//rlog.Debugf("Broadcasted tx %s to %d peers", txhash, relayed_count)
	}
	return
}

//var sync_in_progress bool

// we can tell whether we are syncing by seeing the pending queue of expected response
// if objects response are queued, we are syncing
// if even one of the connection is syncing, then we are syncronising
// returns a number how many blocks are queued
func (connection *Connection) isConnectionSyncing() (count int) {
	//connection.Lock()
	//defer connection.Unlock()

	if atomic.LoadUint32(&connection.State) == HANDSHAKE_PENDING { // skip pre-handshake connections
		return 0
	}

	// check whether 15 secs have passed, if yes close the connection
	// so we can try some other connection
	if len(connection.Objects) > 0 {
		if time.Now().Unix() >= (13 + atomic.LoadInt64(&connection.LastObjectRequestTime)) {
			connection.exit()
			return 0
		}
	}

	return len(connection.Objects)

}

// trigger a sync with a random peer
func trigger_sync() {
	defer globals.Recover(3)

	unique_map := UniqueConnections()

	var clist []*Connection

	for _, value := range unique_map {
		clist = append(clist, value)

	}

	// sort the list random
	// do random shuffling, can we get away with len/2 random shuffling
	globals.Global_Random.Shuffle(len(clist), func(i, j int) {
		clist[i], clist[j] = clist[j], clist[i]
	})

	for _, connection := range clist {

		height := chain.Get_Height()

		//connection.Lock()   recursive mutex are not suported
		// only choose highest available peers for syncing
		if atomic.LoadUint32(&connection.State) != HANDSHAKE_PENDING && height < atomic.LoadInt64(&connection.Height) { // skip pre-handshake connections
			// check whether we are lagging with this connection
			//connection.Lock()
			islagging := height < atomic.LoadInt64(&connection.Height)

			//fmt.Printf("checking cdiff is lagging %+v  topoheight %d peer topoheight %d \n", islagging, topoheight, connection.TopoHeight)

			// islagging := true
			//connection.Unlock()
			if islagging {

				if connection.Pruned > chain.Load_Block_Topological_order(chain.Get_Top_ID()) {
					connection.logger.V(1).Info("We cannot resync with the peer, since peer chain is pruned", "height", connection.Height, "pruned", connection.Pruned)
					continue
				}

				if connection.Height > chain.Get_Height() { // give ourselves one sec, maybe the block is just being written
					time.Sleep(time.Second)
					height := chain.Get_Height()
					islagging = height < atomic.LoadInt64(&connection.Height) // we only use topoheight, since pruned chain might not have full cdiff
				} else {
					continue
				}

				if islagging {
					//connection.Lock()

					connection.logger.V(1).Info("We need to resync with the peer", "our_height", height, "height", connection.Height, "pruned", connection.Pruned)

					//connection.Unlock()
					// set mode to syncronising

					metrics.Set.GetOrCreateCounter("blockchain_sync_total").Inc() // tracks number of syncs

					if chain.Sync {
						//fmt.Printf("chain send chain request disabled\n")
						connection.sync_chain()
						connection.logger.V(1).Info("sync done")

					} else { // we need a state only sync, bootstrap without history but verified chain
						connection.bootstrap_chain()
						chain.Sync = true
					}
				}
				break
			}
		}

	}
}

//detect if something is queued to any of the peer
// is something is queue we are syncing
func IsSyncing() (result bool) {

	syncing := false
	connection_map.Range(func(k, value interface{}) bool {
		v := value.(*Connection)
		if v.isConnectionSyncing() != 0 {
			syncing = true
			return false
		}
		return true
	})
	return syncing
}

//go:noinline
func Abs(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}

// detect whether we are behind any of the connected peers and trigger sync ASAP
// randomly with one of the peers
func syncroniser() {
	for {
		select {
		case <-Exit_Event:
			return
		case <-time.After(1000 * time.Millisecond):
		}

		calculate_network_time() // calculate time every sec

		if !IsSyncing() {
			trigger_sync() // check whether we are out of sync
		}

	}
}

// update P2P time
func calculate_network_time() {
	var total, count, mean int64
	unique_map := UniqueConnections()

	for _, v := range unique_map {
		if Abs(atomic.LoadInt64(&v.clock_offset)) < 100*1000000000 { //  this is 100 sec
			total += atomic.LoadInt64(&v.clock_offset)
			count++
		}
	}
	if count == 0 {
		return
	}
	mean = total / count
	total, count = 0, 0

	var total_float float64
	for _, v := range unique_map {
		if Abs(atomic.LoadInt64(&v.clock_offset)) < 100*1000000000 { //  this is 100 sec
			total_float += math.Pow(float64(atomic.LoadInt64(&v.clock_offset)-mean), 2)
			count++
		}
	}
	if count == 0 {
		return
	}

	variance := total_float / float64(count)
	std_deviation := int64(math.Trunc(math.Sqrt(variance)))

	//		fmt.Printf("\n1 mean %d std_deviation %d variance %f  total_float %f count %d",mean, std_deviation, variance, total_float,count)

	total, count = 0, 0
	for _, v := range unique_map {
		poffset := atomic.LoadInt64(&v.clock_offset)
		if poffset >= (mean-std_deviation) && poffset <= (mean+std_deviation) {
			total += atomic.LoadInt64(&v.clock_offset)
			count++
		}
	}
	//		fmt.Printf("\n2 mean %d std_deviation %d variance %f  total_float %f count %d totaloffset %d\n",mean, std_deviation, variance, total_float,count,total)

	if count == 0 {
		return
	}

	globals.ClockOffsetP2P = time.Duration(total / count)
}

// will return nil, if no peers available
func random_connection() *Connection {
	unique_map := UniqueConnections()

	var clist []*Connection

	for _, value := range unique_map {
		clist = append(clist, value)
	}

	if len(clist) == 0 {
		return nil
	} else if len(clist) == 1 {
		return clist[0]
	}

	// sort the list random
	// do random shuffling, can we get away with len/2 random shuffling
	globals.Global_Random.Shuffle(len(clist), func(i, j int) {
		clist[i], clist[j] = clist[j], clist[i]
	})

	return clist[0]
}

// this will request a tx
func (c *Connection) request_tx(txid [][32]byte, random bool) (err error) {
	var need ObjectList
	var oresponse Objects

	need.Tx_list = append(need.Tx_list, txid...)

	connection := c
	if random {
		connection = random_connection()
	}
	if connection == nil {
		err = fmt.Errorf("No peer available")
		return
	}

	fill_common(&need.Common) // fill common info
	if err = c.RConn.Client.Call("Peer.GetObject", need, &oresponse); err != nil {
		c.exit()
		return
	} else { // process the response
		if err = c.process_object_response(oresponse, 0, false); err != nil {
			return
		}
	}

	return

}
