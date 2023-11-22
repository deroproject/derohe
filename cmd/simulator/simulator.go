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
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strconv"
	"strings"
	"time"

	"github.com/chzyer/readline"
	"github.com/deroproject/derohe/blockchain"
	"github.com/deroproject/derohe/config"
	"github.com/deroproject/derohe/globals"
	"github.com/deroproject/derohe/p2p"
	"github.com/deroproject/derohe/rpc"
	"github.com/docopt/docopt-go"
	"github.com/go-logr/logr"
	"gopkg.in/natefinch/lumberjack.v2"

	//import "crypto/sha1"

	//import "golang.org/x/crypto/sha3"

	//import "github.com/deroproject/derohe/transaction"
	derodrpc "github.com/deroproject/derohe/cmd/derod/rpc"
	"github.com/deroproject/derohe/cmd/explorer/explorerlib"
	"github.com/deroproject/derohe/cryptography/crypto"
	"github.com/deroproject/derohe/walletapi"
)

//import "github.com/deroproject/derosuite/checkpoints"

//import "github.com/deroproject/derosuite/cryptonight"

//import "github.com/deroproject/derosuite/crypto/ringct"
//import "github.com/deroproject/derohe/blockchain/rpcserver"

var command_line string = `simulator 
DERO : A secure, private blockchain with smart-contracts
Simulates DERO block single node which helps in development and tests

Usage:
  simulator [--help] [--version] [--testnet] [--debug] [--noautomine] [--use-xswd] [--sync-node] [--data-dir=<directory>] [--rpc-bind=<127.0.0.1:9999>] [--http-address=<0.0.0.0:8080>] [--clog-level=1] [--flog-level=1]
  simulator -h | --help
  simulator --version

Options:
  -h --help     Show this screen.
  --version     Show version.
  --testnet  	Run in testnet mode.
  --debug       Debug mode enabled, print more log messages
  --noautomine  No blocks will be mined (except genesis), used for testing, supported only on linux
  --use-xswd    Use xswd for wallet rpcs
  --clog-level=1	Set console log level (0 to 127) 
  --flog-level=1	Set file log level (0 to 127)
  --data-dir=<directory>    Store blockchain data at this location
  --rpc-bind=<127.0.0.1:9999>    daemon RPC listens on this ip:port
  --http-address=<0.0.0.0:8080>   explorer listens on this port to serve user requests
  `

var Exit_In_Progress = make(chan bool)

var logger logr.Logger

var rpcport = "127.0.0.1:20000"

var TRIGGER_MINE_BLOCK string = "/dev/shm/mineblocknow"

const wallet_ports_start = 30000      // all wallets will rpc activated on ports
const wallet_ports_xswd_start = 40000 // xswd ports used by wallets if enabled
// this is a crude function used during tests

func Mine_block_single(chain *blockchain.Blockchain, miner_address rpc.Address) error {
	var blid crypto.Hash

	//if !chain.simulator{
	//	return fmt.Errorf("this function can only run in simulator mode")
	//}

	for {
		bl, mbl, _, _, err := chain.Create_new_block_template_mining(miner_address)
		if err != nil {
			logger.Error(err, "err while request block template")
			return err
		}
		if _, blid, _, err = chain.Accept_new_block(bl.Timestamp, mbl.Serialize()); err != nil {
			logger.Error(err, "err while accepting block template")
			return err
		} else if !blid.IsZero() {
			break
		}
	}
	return nil
}

func main() {
	var err error

	globals.Arguments, err = docopt.Parse(command_line, nil, true, config.Version.String(), false)

	if err != nil {
		fmt.Printf("Error while parsing options err: %s\n", err)
		return
	}

	if globals.Arguments["--rpc-bind"] != nil {
		rpcport = globals.Arguments["--rpc-bind"].(string)
	}
	globals.Arguments["--rpc-bind"] = rpcport
	globals.Arguments["--testnet"] = true
	globals.Arguments["--simulator"] = true
	globals.Arguments["--daemon-address"] = rpcport // feed it for wallets

	// We need to initialize readline first, so it changes stderr to ansi processor on windows

	l, err := readline.NewEx(&readline.Config{
		//Prompt:          "\033[92mDERO:\033[32m»\033[0m",
		Prompt:          "\033[92mDEROSIM:\033[32m>>>\033[0m ",
		HistoryFile:     filepath.Join(os.TempDir(), "derosim_readline.tmp"),
		AutoComplete:    completer,
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",

		HistorySearchFold:   true,
		FuncFilterInputRune: filterInput,
	})
	if err != nil {
		fmt.Printf("Error starting readline err: %s\n", err)
		return
	}
	defer l.Close()

	// parse arguments and setup logging , print basic information
	exename, _ := os.Executable()
	globals.InitializeLog(l.Stdout(), &lumberjack.Logger{
		Filename:   exename + ".log",
		MaxSize:    100, // megabytes
		MaxBackups: 2,
	})

	logger = globals.Logger.WithName("derod")

	logger.Info("DERO HE Simulator :  It is an alpha version, use it for testing/evaluations purpose only.")
	logger.Info("Copyright 2017-2021 DERO Project. All rights reserved.")
	logger.Info("", "OS", runtime.GOOS, "ARCH", runtime.GOARCH, "GOMAXPROCS", runtime.GOMAXPROCS(0))
	logger.Info("", "Version", config.Version.String())

	logger.V(1).Info("", "Arguments", globals.Arguments)

	create_genesis_wallet() // create genesis
	globals.Initialize()    // setup network and proxy

	params := map[string]interface{}{}
	params["--simulator"] = true
	globals.Arguments["--p2p-bind"] = ":0"

	logger.V(0).Info("", "MODE", globals.Config.Name)
	logger.V(0).Info("", "Daemon data directory", globals.GetDataDirectory())

	os.RemoveAll(globals.GetDataDirectory()) // remove oldirectory

	chain, err := blockchain.Blockchain_Start(params) //start chain in simulator mode

	if err != nil {
		logger.Error(err, "Error starting blockchain")
		return
	}

	params["chain"] = chain

	logger.Info("Disabled P2P server since we are a simulator")
	p2p.P2P_Init(params)

	rpcserver, _ := derodrpc.RPCServer_Start(params)

	register_wallets(chain)                               // setup 22 wallets
	Mine_block_single(chain, genesis_wallet.GetAddress()) //mine single block to confirm all 22 registrations

	go walletapi.Keep_Connectivity() // all wallets maintain connectivity

	// lets run the explorer at port 8080
	if globals.Arguments["--http-address"] == nil {
		globals.Arguments["--http-address"] = "127.0.0.1:8080"
	}
	if err = explorerlib.StartServer(logr.Discard(), rpcport, globals.Arguments["--http-address"].(string)); err != nil {
		logger.Error(err, "could not start internal explorer")
	}

	go mine_block_auto(chain, genesis_wallet.GetAddress()) // automatically keep mining blocks
	globals.Cron.Start()                                   // start cron jobs

	// This tiny goroutine continuously updates status as required
	go func() {
		last_our_height := int64(0)
		last_best_height := int64(0)
		last_peer_count := uint64(0)
		last_topo_height := int64(0)
		last_mempool_tx_count := 0
		last_regpool_tx_count := 0

		for {
			select {
			case <-Exit_In_Progress:
				return
			default:
			}
			our_height := chain.Get_Height()
			best_height, best_topo_height := p2p.Best_Peer_Height()
			peer_count := p2p.Peer_Count()
			topo_height := chain.Load_TOPO_HEIGHT()

			mempool_tx_count := len(chain.Mempool.Mempool_List_TX())
			regpool_tx_count := len(chain.Regpool.Regpool_List_TX())

			// only update prompt if needed
			if last_our_height != our_height || last_best_height != best_height || last_peer_count != peer_count || last_topo_height != topo_height || last_mempool_tx_count != mempool_tx_count || last_regpool_tx_count != regpool_tx_count {
				// choose color based on urgency
				color := "\033[32m" // default is green color
				if our_height < best_height {
					color = "\033[33m" // make prompt yellow
				} else if our_height > best_height {
					color = "\033[31m" // make prompt red
				}

				pcolor := "\033[32m" // default is green color
				if peer_count < 1 {
					pcolor = "\033[31m" // make prompt red
				} else if peer_count <= 8 {
					pcolor = "\033[33m" // make prompt yellow
				}

				hash_rate_string := ""
				hash_rate := chain.Get_Network_HashRate()
				switch {
				case hash_rate > 1000000000000:
					hash_rate_string = fmt.Sprintf("%.1f TH/s", float64(hash_rate)/1000000000000.0)
				case hash_rate > 1000000000:
					hash_rate_string = fmt.Sprintf("%.1f GH/s", float64(hash_rate)/1000000000.0)
				case hash_rate > 1000000:
					hash_rate_string = fmt.Sprintf("%.1f MH/s", float64(hash_rate)/1000000.0)
				case hash_rate > 1000:
					hash_rate_string = fmt.Sprintf("%.1f KH/s", float64(hash_rate)/1000.0)
				case hash_rate > 0:
					hash_rate_string = fmt.Sprintf("%d H/s", hash_rate)
				}

				testnet_string := ""
				if !globals.IsMainnet() {
					testnet_string = "\033[31m TESTNET"
				}

				l.SetPrompt(fmt.Sprintf("\033[1m\033[32mDEROSIM HE: \033[0m"+color+"%d/%d [%d/%d] "+pcolor+"P %d TXp %d:%d \033[32mNW %s %s> >>\033[0m ", our_height, topo_height, best_height, best_topo_height, peer_count, mempool_tx_count, regpool_tx_count, hash_rate_string, testnet_string))
				l.Refresh()
				last_our_height = our_height
				last_best_height = best_height
				last_peer_count = peer_count
				last_mempool_tx_count = mempool_tx_count
				last_regpool_tx_count = regpool_tx_count
				last_topo_height = best_topo_height
			}
			time.Sleep(1 * time.Second)
		}
	}()

	setPasswordCfg := l.GenPasswordConfig()
	setPasswordCfg.SetListener(func(line []rune, pos int, key rune) (newLine []rune, newPos int, ok bool) {
		l.SetPrompt(fmt.Sprintf("Enter password(%v): ", len(line)))
		l.Refresh()
		return nil, 0, false
	})
	l.Refresh() // refresh the prompt

	go func() {
		var gracefulStop = make(chan os.Signal, 1)
		signal.Notify(gracefulStop, os.Interrupt) // listen to all signals
		for {
			sig := <-gracefulStop
			logger.Info("received signal", "signal", sig)

			if sig.String() == "interrupt" {
				close(Exit_In_Progress)
			}
		}
	}()

	for {
		line, err := l.Readline()
		if err == readline.ErrInterrupt {
			if len(line) == 0 {
				logger.Info("Ctrl-C received, Exit in progress")
				close(Exit_In_Progress)
				break
			} else {
				continue
			}
		} else if err == io.EOF {
			<-Exit_In_Progress
			break
		}

		line = strings.TrimSpace(line)
		line_parts := strings.Fields(line)

		command := ""
		if len(line_parts) >= 1 {
			command = strings.ToLower(line_parts[0])
		}

		switch {
		case line == "help":
			usage(l.Stderr())

		case command == "profile": // writes cpu and memory profile
			// TODO enable profile over http rpc to enable better testing/tracking
			cpufile, err := os.Create(filepath.Join(globals.GetDataDirectory(), "cpuprofile.prof"))
			if err != nil {
				logger.Error(err, "Could not start cpu profiling.")
				continue
			}
			if err := pprof.StartCPUProfile(cpufile); err != nil {
				logger.Error(err, "could not start CPU profile")
			}
			logger.Info("CPU profiling will be available after program exits.", "path", filepath.Join(globals.GetDataDirectory(), "cpuprofile.prof"))
			defer pprof.StopCPUProfile()

			/*
				        	memoryfile,err := os.Create(filepath.Join(globals.GetDataDirectory(), "memoryprofile.prof"))
							if err != nil{
								globals.Logger.Errorf("Could not start memory profiling, err %s", err)
								continue
							}
							if err := pprof.WriteHeapProfile(memoryfile); err != nil {
				            	globals.Logger.Warnf("could not start memory profile: ", err)
				        	}
				        	memoryfile.Close()
			*/

		case command == "regpool_print":
			chain.Regpool.Regpool_Print()

		case command == "regpool_flush":
			chain.Regpool.Regpool_flush()
		case command == "regpool_delete_tx":

			if len(line_parts) == 2 && len(line_parts[1]) == 64 {
				txid, err := hex.DecodeString(strings.ToLower(line_parts[1]))
				if err != nil {
					logger.Error(err, "err parsing txid")
					continue
				}
				var hash crypto.Hash
				copy(hash[:32], []byte(txid))

				chain.Regpool.Regpool_Delete_TX(hash)
			} else {
				logger.Error(fmt.Errorf("regpool_delete_tx  needs a single transaction id as argument"), "")
			}

		case command == "mempool_print":
			chain.Mempool.Mempool_Print()

		case command == "mempool_flush":
			chain.Mempool.Mempool_flush()
		case command == "mempool_delete_tx":

			if len(line_parts) == 2 && len(line_parts[1]) == 64 {
				txid, err := hex.DecodeString(strings.ToLower(line_parts[1]))
				if err != nil {
					logger.Error(err, "err parsing txid")
					continue
				}
				var hash crypto.Hash
				copy(hash[:32], []byte(txid))

				chain.Mempool.Mempool_Delete_TX(hash)
			} else {
				logger.Error(fmt.Errorf("mempool_delete_tx  needs a single transaction id as argument"), "")
			}

		case command == "version":
			logger.Info("", "OS", runtime.GOOS, "ARCH", runtime.GOARCH, "GOMAXPROCS", runtime.GOMAXPROCS(0))
			logger.Info("", "Version", config.Version.String())

		case command == "print_tree": // prints entire block chain tree
			//WriteBlockChainTree(chain, "/tmp/graph.dot")

		case command == "print_block":

			fmt.Printf("printing block\n")
			if len(line_parts) == 2 && len(line_parts[1]) == 64 {
				bl_raw, err := hex.DecodeString(strings.ToLower(line_parts[1]))

				if err != nil {
					fmt.Printf("err while decoding txid err %s\n", err)
					continue
				}
				var hash crypto.Hash
				copy(hash[:32], []byte(bl_raw))

				bl, err := chain.Load_BL_FROM_ID(hash)
				if err == nil {
					fmt.Printf("Block ID : %s\n", hash)
					fmt.Printf("Block : %x\n", bl.Serialize())
					fmt.Printf("difficulty: %s\n", chain.Load_Block_Difficulty(hash).String())
					//fmt.Printf("Orphan: %v\n",chain.Is_Block_Orphan(hash))

					json_bytes, err := json.Marshal(bl)

					fmt.Printf("%s  err : %s\n", string(prettyprint_json(json_bytes)), err)
				} else {
					fmt.Printf("Err %s\n", err)
				}
			} else if len(line_parts) == 2 {
				if s, err := strconv.ParseInt(line_parts[1], 10, 64); err == nil {
					_ = s
					// first load block id from topo height

					hash, err := chain.Load_Block_Topological_order_at_index(s)
					if err != nil {
						fmt.Printf("Skipping block at topo height %d due to error %s\n", s, err)
						continue
					}
					bl, err := chain.Load_BL_FROM_ID(hash)
					if err == nil {
						fmt.Printf("Block ID : %s\n", hash)
						fmt.Printf("Block : %x\n", bl.Serialize())
						fmt.Printf("difficulty: %s\n", chain.Load_Block_Difficulty(hash).String())
						fmt.Printf("Height: %d\n", chain.Load_Height_for_BL_ID(hash))
						fmt.Printf("TopoHeight: %d\n", s)

						version, err := chain.ReadBlockSnapshotVersion(hash)
						if err != nil {
							panic(err)
						}

						bhash, err := chain.Load_Merkle_Hash(version)

						if err != nil {
							panic(err)
						}

						fmt.Printf("BALANCE_TREE : %s\n", bhash)

						//fmt.Printf("Orphan: %v\n",chain.Is_Block_Orphan(hash))

						json_bytes, err := json.Marshal(bl)

						fmt.Printf("%s  err : %s\n", string(prettyprint_json(json_bytes)), err)
					} else {
						fmt.Printf("Err %s\n", err)
					}

				} else {
					fmt.Printf("print_block  needs a single block id as argument\n")
				}
			}

		case strings.ToLower(line) == "status":

			// fmt.Printf("chain diff %d\n",chain.Get_Difficulty_At_Block(chain.Top_ID))
			//fmt.Printf("chain nw rate %d\n", chain.Get_Network_HashRate())
			inc, out := p2p.Peer_Direction_Count()

			mempool_tx_count := len(chain.Mempool.Mempool_List_TX())
			regpool_tx_count := len(chain.Regpool.Regpool_List_TX())

			//supply := chain.Load_Already_Generated_Coins_for_Topo_Index(nil, chain.Load_TOPO_HEIGHT(nil))

			supply := uint64(0)

			if supply > (1000000 * 1000000000000) {
				supply -= (1000000 * 1000000000000) // remove  premine
			}
			fmt.Printf("Network %s Height %d  NW Hashrate %0.03f MH/sec  TH %s Peers %d inc, %d out  MEMPOOL size %d REGPOOL %d  Total Supply %s DERO \n", globals.Config.Name, chain.Get_Height(), float64(chain.Get_Network_HashRate())/1000000.0, chain.Get_Top_ID(), inc, out, mempool_tx_count, regpool_tx_count, globals.FormatMoney(supply))

			// print hardfork status on second line
			hf_state, _, _, threshold, version, votes, window := chain.Get_HF_info()
			switch hf_state {
			case 0: // voting in progress
				locked := false
				if window == 0 {
					window = 1
				}
				if votes >= (threshold*100)/window {
					locked = true
				}
				fmt.Printf("Hard-Fork v%d in-progress need %d/%d votes to lock in, votes: %d, LOCKED:%+v\n", version, ((threshold * 100) / window), window, votes, locked)
			case 1: // daemon is old and needs updation
				fmt.Printf("Please update this daemon to  support Hard-Fork v%d\n", version)
			case 2: // everything is okay
				fmt.Printf("Hard-Fork v%d\n", version)

			}

		case strings.ToLower(line) == "bye":
			fallthrough
		case strings.ToLower(line) == "exit":
			fallthrough
		case strings.ToLower(line) == "quit":
			close(Exit_In_Progress)
			goto exit

		case line == "sleep":
			logger.Info("console sleeping for 1 second")
			time.Sleep(1 * time.Second)
		case line == "":
		default:
			logger.Info(fmt.Sprintf("you said: %s", strconv.Quote(line)))
		}
	}
exit:

	logger.Info("Exit in Progress, Please wait")
	time.Sleep(100 * time.Millisecond) // give prompt update time to finish

	rpcserver.RPCServer_Stop()
	p2p.P2P_Shutdown() // shutdown p2p subsystem
	chain.Shutdown()   // shutdown chain subsysem
	stop_rpcs()        // stop all wallets rpcs

	for globals.Subsystem_Active > 0 {
		logger.Info("Exit in Progress, Please wait.", "active subsystems", globals.Subsystem_Active)
		time.Sleep(1000 * time.Millisecond)
	}
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return !errors.Is(err, os.ErrNotExist)
}

func trigger_block_creation() {
	fmt.Printf("triggering fie creation\n")
	if globals.Arguments["--noautomine"].(bool) == true {
		err := os.WriteFile(TRIGGER_MINE_BLOCK, []byte("HELLO"), 0666)
		fmt.Printf("triggering fie creation %s err %s\n", TRIGGER_MINE_BLOCK, err)
	}
}

// generate a block as soon as tx appears in blockchain
// or 15 sec pass
func mine_block_auto(chain *blockchain.Blockchain, miner_address rpc.Address) {

	last_block_time := time.Now()
	for {
		bl, _, _, _, err := chain.Create_new_block_template_mining(miner_address)
		if err != nil {
			logger.Error(err, "error while building mining block")
			continue
		}

		if globals.Arguments["--noautomine"].(bool) == true && exists(TRIGGER_MINE_BLOCK) {
			if err = Mine_block_single(chain, miner_address); err == nil {
				last_block_time = time.Now()
				os.Remove(TRIGGER_MINE_BLOCK)
			} else {
				logger.Error(err, "error while mining single block")
			}
		}
		if globals.Arguments["--noautomine"].(bool) == false {
			if time.Now().Sub(last_block_time) > time.Duration(config.BLOCK_TIME)*time.Second || // every X secs generate a block
				len(bl.Tx_hashes) >= 1 { //pools have a tx, try to mine them ASAP
				if err := Mine_block_single(chain, miner_address); err == nil {
					last_block_time = time.Now()
				}
			}
		}
		time.Sleep(900 * time.Millisecond)
	}
}

func prettyprint_json(b []byte) []byte {
	var out bytes.Buffer
	err := json.Indent(&out, b, "", "  ")
	_ = err
	return out.Bytes()
}

func usage(w io.Writer) {
	io.WriteString(w, "commands:\n")
	//io.WriteString(w, completer.Tree("    "))
	io.WriteString(w, "\t\033[1mhelp\033[0m\t\tthis help\n")
	io.WriteString(w, "\t\033[1mdiff\033[0m\t\tShow difficulty\n")
	io.WriteString(w, "\t\033[1mprint_bc\033[0m\tPrint blockchain info in a given blocks range, print_bc <begin_height> <end_height>\n")
	io.WriteString(w, "\t\033[1mprint_block\033[0m\tPrint block, print_block <block_hash> or <block_height>\n")
	io.WriteString(w, "\t\033[1mprint_height\033[0m\tPrint local blockchain height\n")
	io.WriteString(w, "\t\033[1mprint_tx\033[0m\tPrint transaction, print_tx <transaction_hash>\n")
	io.WriteString(w, "\t\033[1mstatus\033[0m\t\tShow general information\n")
	io.WriteString(w, "\t\033[1mstart_mining\033[0m\tStart mining <dero address> <number of threads>\n")
	io.WriteString(w, "\t\033[1mstop_mining\033[0m\tStop daemon mining\n")
	io.WriteString(w, "\t\033[1mpeer_list\033[0m\tPrint peer list\n")
	io.WriteString(w, "\t\033[1msync_info\033[0m\tPrint information about connected peers and their state\n")
	io.WriteString(w, "\t\033[1mbye\033[0m\t\tQuit the daemon\n")
	io.WriteString(w, "\t\033[1mban\033[0m\t\tBan specific ip from making any connections\n")
	io.WriteString(w, "\t\033[1munban\033[0m\t\tRevoke restrictions on previously banned ips\n")
	io.WriteString(w, "\t\033[1mbans\033[0m\t\tPrint current ban list\n")
	io.WriteString(w, "\t\033[1mmempool_print\033[0m\t\tprint mempool contents\n")
	io.WriteString(w, "\t\033[1mmempool_delete_tx\033[0m\t\tDelete specific tx from mempool\n")
	io.WriteString(w, "\t\033[1mmempool_flush\033[0m\t\tFlush regpool\n")
	io.WriteString(w, "\t\033[1mregpool_print\033[0m\t\tprint regpool contents\n")
	io.WriteString(w, "\t\033[1mregpool_delete_tx\033[0m\t\tDelete specific tx from regpool\n")
	io.WriteString(w, "\t\033[1mregpool_flush\033[0m\t\tFlush mempool\n")
	io.WriteString(w, "\t\033[1mversion\033[0m\t\tShow version\n")
	io.WriteString(w, "\t\033[1mexit\033[0m\t\tQuit the daemon\n")
	io.WriteString(w, "\t\033[1mquit\033[0m\t\tQuit the daemon\n")

}

var completer = readline.NewPrefixCompleter(
	readline.PcItem("help"),
	readline.PcItem("diff"),

	readline.PcItem("mempool_flush"),
	readline.PcItem("mempool_delete_tx"),
	readline.PcItem("mempool_print"),
	readline.PcItem("regpool_flush"),
	readline.PcItem("regpool_delete_tx"),
	readline.PcItem("regpool_print"),
	readline.PcItem("status"),
	readline.PcItem("version"),
	readline.PcItem("bye"),
	readline.PcItem("exit"),
	readline.PcItem("quit"),
)

func filterInput(r rune) (rune, bool) {
	switch r {
	// block CtrlZ feature
	case readline.CharCtrlZ:
		return r, false
	}
	return r, true
}
