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

import "io"
import "os"
import "time"
import "fmt"
import "bytes"

import "bufio"
import "strings"
import "strconv"
import "runtime"
import "runtime/debug"
import "math/big"
import "os/signal"

//import "crypto/sha1"
import "encoding/hex"
import "encoding/json"
import "path/filepath"
import "runtime/pprof"

import "github.com/go-logr/logr"

//import "golang.org/x/crypto/sha3"

import "github.com/chzyer/readline"
import "github.com/docopt/docopt-go"
import "gopkg.in/natefinch/lumberjack.v2"

import "github.com/deroproject/derohe/p2p"
import "github.com/deroproject/derohe/globals"
import "github.com/deroproject/derohe/block"
import "github.com/deroproject/derohe/transaction"
import "github.com/deroproject/derohe/config"
import "github.com/deroproject/derohe/rpc"
import "github.com/deroproject/derohe/blockchain"
import derodrpc "github.com/deroproject/derohe/cmd/derod/rpc"

import "github.com/deroproject/derohe/cryptography/crypto"

var command_line string = `derod 
DERO : A secure, private blockchain with smart-contracts

Usage:
  derod [--help] [--version] [--testnet] [--debug]  [--sync-node] [--timeisinsync] [--fastsync] [--socks-proxy=<socks_ip:port>] [--data-dir=<directory>] [--p2p-bind=<0.0.0.0:18089>] [--add-exclusive-node=<ip:port>]... [--add-priority-node=<ip:port>]... [--min-peers=<11>] [--max-peers=<100>] [--rpc-bind=<127.0.0.1:9999>] [--getwork-bind=<0.0.0.0:18089>] [--node-tag=<unique name>] [--prune-history=<50>] [--integrator-address=<address>] [--clog-level=1] [--flog-level=1]
  derod -h | --help
  derod --version

Options:
  -h --help     Show this screen.
  --version     Show version.
  --testnet  	Run in testnet mode.
  --debug       Debug mode enabled, print more log messages
  --clog-level=1	Set console log level (0 to 127) 
  --flog-level=1	Set file log level (0 to 127)
  --fastsync      Fast sync mode (this option has effect only while bootstrapping)
  --timeisinsync  Confirms to daemon that time is in sync, so daemon doesn't try to sync
  --socks-proxy=<socks_ip:port>  Use a proxy to connect to network.
  --data-dir=<directory>    Store blockchain data at this location
  --rpc-bind=<127.0.0.1:9999>    RPC listens on this ip:port
  --p2p-bind=<0.0.0.0:18089>    p2p server listens on this ip:port, specify port 0 to disable listening server
  --getwork-bind=<0.0.0.0:10100>    getwork server listens on this ip:port, specify port 0 to disable listening server
  --add-exclusive-node=<ip:port>	Connect to specific peer only 
  --add-priority-node=<ip:port>	Maintain persistant connection to specified peer
  --sync-node       Sync node automatically with the seeds nodes. This option is for rare use.
  --node-tag=<unique name>	Unique name of node, visible to everyone
  --integrator-address	if this node mines a block,Integrator rewards will be given to address.default is dev's address.
  --min-peers=<31>	  Node will try to maintain atleast this many connections to peers
  --max-peers=<101>	  Node will maintain maximim this many connections to peers and will stop accepting connections
  --prune-history=<50>	prunes blockchain history until the specific topo_height

  `

var Exit_In_Progress = make(chan bool)

var logger logr.Logger

func dump(filename string) {
	f, err := os.Create(filename)
	if err != nil {
		fmt.Printf("err creating file %s\n", err)
		return
	}

	runtime.GC()
	debug.WriteHeapDump(f.Fd())

	err = f.Close()
	if err != nil {
		fmt.Printf("err closing file %s\n", err)
	}
}

func main() {
	runtime.MemProfileRate = 0
	var err error
	globals.Arguments, err = docopt.Parse(command_line, nil, true, config.Version.String(), false)

	if err != nil {
		fmt.Printf("Error while parsing options err: %s\n", err)
		return
	}

	// We need to initialize readline first, so it changes stderr to ansi processor on windows

	l, err := readline.NewEx(&readline.Config{
		//Prompt:          "\033[92mDERO:\033[32m»\033[0m",
		Prompt:          "\033[92mDERO:\033[32m>>>\033[0m ",
		HistoryFile:     filepath.Join(os.TempDir(), "derod_readline.tmp"),
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

	logger.Info("DERO HE daemon :  It is an alpha version, use it for testing/evaluations purpose only.")
	logger.Info("Copyright 2017-2021 DERO Project. All rights reserved.")
	logger.Info("", "OS", runtime.GOOS, "ARCH", runtime.GOARCH, "GOMAXPROCS", runtime.GOMAXPROCS(0))
	logger.Info("", "Version", config.Version.String())

	logger.V(1).Info("", "Arguments", globals.Arguments)

	globals.Initialize() // setup network and proxy

	logger.V(0).Info("", "MODE", globals.Config.Name)
	logger.V(0).Info("", "Daemon data directory", globals.GetDataDirectory())

	//go check_update_loop ()

	params := map[string]interface{}{}

	// check  whether we are pruning, if requested do so
	prune_topo := int64(50)
	if _, ok := globals.Arguments["--prune-history"]; ok && globals.Arguments["--prune-history"] != nil { // user specified a limit, use it if possible
		i, err := strconv.ParseInt(globals.Arguments["--prune-history"].(string), 10, 64)
		if err != nil {
			logger.Error(err, "error Parsing --prune-history ")
			return
		} else {
			if i <= 1 {
				logger.Error(fmt.Errorf("--prune-history should be positive and more than 1"), "invalid argument")
				return
			} else {
				prune_topo = i
			}
		}
		logger.Info("will prune history till", "topo_height", prune_topo)

		if err := blockchain.Prune_Blockchain(prune_topo); err != nil {
			logger.Error(err, "Error pruning blockchain ")
			return
		} else {
			logger.Info("blockchain pruning successful")

		}
	}

	if _, ok := globals.Arguments["--timeisinsync"]; ok {
		globals.TimeIsInSync = globals.Arguments["--timeisinsync"].(bool)
	}

	if _, ok := globals.Arguments["--integrator-address"]; ok {
		params["--integrator-address"] = globals.Arguments["--integrator-address"]
	}

	chain, err := blockchain.Blockchain_Start(params)
	if err != nil {
		logger.Error(err, "Error starting blockchain")
		return
	}

	params["chain"] = chain

	// since user is using a proxy, he definitely does not want to give out his IP
	if globals.Arguments["--socks-proxy"] != nil {
		globals.Arguments["--p2p-bind"] = ":0"
		logger.Info("Disabling P2P server since we are using socks proxy")
	}

	p2p.P2P_Init(params)
	rpcserver, _ := derodrpc.RPCServer_Start(params)

	go derodrpc.Getwork_server()

	// setup function pointers
	chain.P2P_Block_Relayer = func(cbl *block.Complete_Block, peerid uint64) {
		p2p.Broadcast_Block(cbl, peerid)
	}

	chain.P2P_MiniBlock_Relayer = func(mbl block.MiniBlock, peerid uint64) {
		p2p.Broadcast_MiniBlock(mbl, peerid)
	}

	{
		current_blid, err := chain.Load_Block_Topological_order_at_index(17600)
		if err == nil {

			current_blid := current_blid
			for {
				height := chain.Load_Height_for_BL_ID(current_blid)

				if height < 17500 {
					break
				}

				r, err := chain.Store.Topo_store.Read(int64(height))
				if err != nil {
					panic(err)
				}
				if r.BLOCK_ID != current_blid {
					fmt.Printf("Fixing corruption r %+v  , current_blid %s current_blid_height %d\n", r, current_blid, height)

					fix_commit_version, err := chain.ReadBlockSnapshotVersion(current_blid)
					if err != nil {
						panic(err)
					}

					chain.Store.Topo_store.Write(int64(height), current_blid, fix_commit_version, int64(height))

				}

				fix_bl, err := chain.Load_BL_FROM_ID(current_blid)
				if err != nil {
					panic(err)
				}
				current_blid = fix_bl.Tips[0]
			}
		}
	}
	globals.Cron.Start() // start cron jobs

	// This tiny goroutine continuously updates status as required
	go func() {
		last_our_height := int64(0)
		last_best_height := int64(0)
		last_peer_count := uint64(0)
		last_topo_height := int64(0)
		last_mempool_tx_count := 0
		last_regpool_tx_count := 0
		last_second := int64(0)
		for {
			select {
			case <-Exit_In_Progress:
				return
			default:
			}

			func() {
				defer globals.Recover(0) // a panic might occur, due to some rare file system issues, so skip them
				our_height := chain.Get_Height()
				best_height, best_topo_height := p2p.Best_Peer_Height()
				peer_count := p2p.Peer_Count()
				topo_height := chain.Load_TOPO_HEIGHT()

				mempool_tx_count := len(chain.Mempool.Mempool_List_TX())
				regpool_tx_count := len(chain.Regpool.Regpool_List_TX())

				// only update prompt if needed
				if last_second != time.Now().Unix() || last_our_height != our_height || last_best_height != best_height || last_peer_count != peer_count || last_topo_height != topo_height || last_mempool_tx_count != mempool_tx_count || last_regpool_tx_count != regpool_tx_count {
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

					hash_rate_string := hashratetostring(chain.Get_Network_HashRate())

					testnet_string := ""
					if globals.IsMainnet() {
						testnet_string = "\033[31m MAINNET"
					} else {
						testnet_string = "\033[31m TESTNET"
					}

					testnet_string += " " + strconv.Itoa(chain.MiniBlocks.Count()) + " " + globals.GetOffset().Round(time.Millisecond).String() + "|" + globals.GetOffsetNTP().Round(time.Millisecond).String() + "|" + globals.GetOffsetP2P().Round(time.Millisecond).String()

					miner_count := derodrpc.CountMiners()
					l.SetPrompt(fmt.Sprintf("\033[1m\033[32mDERO HE: \033[0m"+color+"%d/%d [%d/%d] "+pcolor+"P %d TXp %d:%d \033[32mNW %s >MN %d %s>>\033[0m ", our_height, topo_height, best_height, best_topo_height, peer_count, mempool_tx_count, regpool_tx_count, hash_rate_string, miner_count, testnet_string))
					l.Refresh()
					last_second = time.Now().Unix()
					last_our_height = our_height
					last_best_height = best_height
					last_peer_count = peer_count
					last_mempool_tx_count = mempool_tx_count
					last_regpool_tx_count = regpool_tx_count
					last_topo_height = best_topo_height
				}
			}()
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
				return
			}
		}
	}()

	for {
		if err = readline_loop(l, chain, logger); err == nil {
			break
		}
	}

	logger.Info("Exit in Progress, Please wait")
	time.Sleep(100 * time.Millisecond) // give prompt update time to finish

	rpcserver.RPCServer_Stop()
	p2p.P2P_Shutdown() // shutdown p2p subsystem
	chain.Shutdown()   // shutdown chain subsysem

	for globals.Subsystem_Active > 0 {
		logger.Info("Exit in Progress, Please wait.", "active subsystems", globals.Subsystem_Active)
		time.Sleep(1000 * time.Millisecond)
	}
}

func readline_loop(l *readline.Instance, chain *blockchain.Blockchain, logger logr.Logger) (err error) {

	defer func() {
		if r := recover(); r != nil {
			logger.V(0).Error(nil, "Recovered ", "error", r)
			err = fmt.Errorf("crashed")
		}

	}()

restart_loop:
	for {
		line, err := l.Readline()
		if err == io.EOF {
			<-Exit_In_Progress
			return nil
		}

		if err == readline.ErrInterrupt {
			if len(line) == 0 {
				logger.Info("Ctrl-C received, Exit in progress")
				return nil
			} else {
				continue
			}
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
				continue
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

		case command == "setintegratoraddress":
			if len(line_parts) != 2 {
				logger.Error(fmt.Errorf("This function requires 1 parameters, dero address"), "")
				continue
			}
			if addr, err := rpc.NewAddress(line_parts[1]); err != nil {
				logger.Error(err, "invalid address")
				continue
			} else {
				chain.SetIntegratorAddress(*addr)
				logger.Info("will use", "integrator_address", chain.IntegratorAddress().String())
			}

		case command == "print_bc":

			logger.Info("printing block chain")
			// first is starting point, second is ending point
			start := int64(0)
			stop := int64(0)

			if len(line_parts) != 3 {
				logger.Error(fmt.Errorf("This function requires 2 parameters, start and endpoint"), "")
				continue
			}
			if s, err := strconv.ParseInt(line_parts[1], 10, 64); err == nil {
				start = s
			} else {
				logger.Error(err, "Invalid start value", "value", line_parts[1])
				continue
			}

			if s, err := strconv.ParseInt(line_parts[2], 10, 64); err == nil {
				stop = s
			} else {
				logger.Error(err, "Invalid stop value", "value", line_parts[1])
				continue
			}

			if start < 0 || start > int64(chain.Load_TOPO_HEIGHT()) {
				logger.Error(fmt.Errorf("Start value should be be between 0 and current height"), "")
				continue
			}
			if start > stop || stop > int64(chain.Load_TOPO_HEIGHT()) {
				logger.Error(fmt.Errorf("Stop value should be > start and current height"), "")
				continue
			}

			logger.Info("Printing block chain", "start", start, "stop", stop)

			for i := start; i <= stop; i++ {
				// get block id at height
				current_block_id, err := chain.Load_Block_Topological_order_at_index(i)
				if err != nil {
					logger.Error(err, "Skipping block at height due to error \n", "height", i)
					continue
				}
				var timestamp uint64
				diff := new(big.Int)
				if chain.Block_Exists(current_block_id) {
					timestamp = chain.Load_Block_Timestamp(current_block_id)
					diff = chain.Load_Block_Difficulty(current_block_id)
				}

				version, err := chain.ReadBlockSnapshotVersion(current_block_id)
				if err != nil {
					panic(err)
				}

				balance_hash, err := chain.Load_Merkle_Hash(version)

				if err != nil {
					panic(err)
				}

				logger.Info("", "topo height", i, "height", chain.Load_Height_for_BL_ID(current_block_id), "timestamp", timestamp, "difficulty", diff.String())
				logger.Info("", "Block Id", current_block_id.String(), "balance_tree hash", balance_hash.String())
				logger.Info("\n")

			}
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

		case command == "mempool_dump": // dump mempool to directory
			tx_hash_list_sorted := chain.Mempool.Mempool_List_TX_SortedInfo() // hash of all tx expected to be included within this block , sorted by fees

			os.Mkdir(filepath.Join(globals.GetDataDirectory(), "mempool"), 0755)
			count := 0
			for _, txi := range tx_hash_list_sorted {
				if tx := chain.Mempool.Mempool_Get_TX(txi.Hash); tx != nil {
					os.WriteFile(filepath.Join(globals.GetDataDirectory(), "mempool", txi.Hash.String()), tx.Serialize(), 0755)
					count++
				}
			}
			logger.Info("flushed mempool to driectory", "count", count, "dir", filepath.Join(globals.GetDataDirectory(), "mempool"))

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

		case command == "block_export":
			var hash crypto.Hash

			if len(line_parts) == 2 && len(line_parts[1]) == 64 {
				bl_raw, err := hex.DecodeString(strings.ToLower(line_parts[1]))
				if err != nil {
					fmt.Printf("err while decoding blid err %s\n", err)
					continue
				}
				copy(hash[:32], []byte(bl_raw))
			} else {
				fmt.Printf("block_export  needs a single block id as argument\n")
				continue
			}

			var cbl *block.Complete_Block

			bl, err := chain.Load_BL_FROM_ID(hash)
			if err != nil {
				fmt.Printf("Err %s\n", err)
				continue
			}
			cbl = &block.Complete_Block{Bl: bl}
			for _, txid := range bl.Tx_hashes {

				var tx transaction.Transaction
				if tx_bytes, err := chain.Store.Block_tx_store.ReadTX(txid); err != nil {
					fmt.Printf("err while reading txid err %s\n", err)
					continue restart_loop
				} else if err = tx.Deserialize(tx_bytes); err != nil {
					fmt.Printf("err deserializing tx err %s\n", err)
					continue restart_loop
				}
				cbl.Txs = append(cbl.Txs, &tx)

			}

			cbl_bytes := p2p.Convert_CBL_TO_P2PCBL(cbl, true)

			if err := os.WriteFile(fmt.Sprintf("/tmp/%s.block", hash), cbl_bytes, 0755); err != nil {
				fmt.Printf("err writing block %s\n", err)
				continue
			}

			fmt.Printf("successfully exported block to %s\n", fmt.Sprintf("/tmp/%s.block", hash))

		case command == "block_import":
			var hash crypto.Hash

			if len(line_parts) == 2 && len(line_parts[1]) == 64 {
				bl_raw, err := hex.DecodeString(strings.ToLower(line_parts[1]))
				if err != nil {
					fmt.Printf("err while decoding blid err %s\n", err)
					continue
				}
				copy(hash[:32], []byte(bl_raw))
			} else {
				fmt.Printf("install_block  needs a single block id as argument\n")
				continue
			}

			var cbl *block.Complete_Block

			if block_data, err := os.ReadFile(fmt.Sprintf("/tmp/%s.block", hash)); err == nil {

				cbl = p2p.Convert_P2PCBL_TO_CBL(block_data)
			} else {
				fmt.Printf("err reading block %s\n", err)
				continue
			}

			err, _ = chain.Add_Complete_Block(cbl)
			fmt.Printf("err adding block %s\n", err)

		case command == "fix":
			tips := chain.Get_TIPS()

			current_blid := tips[0]
			for {
				height := chain.Load_Height_for_BL_ID(current_blid)

				//fmt.Printf("checking height %d\n", height)

				if height < 1 {
					break
				}

				r, err := chain.Store.Topo_store.Read(int64(height))
				if err != nil {
					panic(err)
				}
				if r.BLOCK_ID != current_blid {
					fmt.Printf("corruption due to XYZ r %+v  , current_blid %s current_blid_height %d\n", r, current_blid, height)

					fix_commit_version, err := chain.ReadBlockSnapshotVersion(current_blid)
					if err != nil {
						panic(err)
					}

					chain.Store.Topo_store.Write(int64(height), current_blid, fix_commit_version, int64(height))

				}

				fix_bl, err := chain.Load_BL_FROM_ID(current_blid)
				if err != nil {
					panic(err)
				}

				current_blid = fix_bl.Tips[0]

				/*		fix_commit_version, err = chain.ReadBlockSnapshotVersion(current_block_id)
						if err != nil {
							panic(err)
						}
				*/

			}

		case command == "print_block":

			fmt.Printf("printing block\n")
			var hash crypto.Hash

			if len(line_parts) == 2 && len(line_parts[1]) == 64 {
				bl_raw, err := hex.DecodeString(strings.ToLower(line_parts[1]))
				if err != nil {
					fmt.Printf("err while decoding blid err %s\n", err)
					continue
				}
				copy(hash[:32], []byte(bl_raw))
			} else if len(line_parts) == 2 {
				if s, err := strconv.ParseInt(line_parts[1], 10, 64); err == nil {
					_ = s
					// first load block id from topo height

					hash, err = chain.Load_Block_Topological_order_at_index(s)
					if err != nil {
						fmt.Printf("Skipping block at topo height %d due to error %s\n", s, err)
						continue
					}
				}
			} else {
				fmt.Printf("print_block  needs a single block id as argument\n")
				continue
			}
			bl, err := chain.Load_BL_FROM_ID(hash)
			if err != nil {
				fmt.Printf("Err %s\n", err)
				continue
			}

			header, _ := derodrpc.GetBlockHeader(chain, hash)
			fmt.Fprintf(os.Stdout, "BLID:%s\n", bl.GetHash())
			fmt.Fprintf(os.Stdout, "Major version:%d Minor version: %d ", bl.Major_Version, bl.Minor_Version)
			fmt.Fprintf(os.Stdout, "Height:%d ", bl.Height)
			fmt.Fprintf(os.Stdout, "Timestamp:%d  (%s)\n", bl.Timestamp, bl.GetTimestamp())
			for i := range bl.Tips {
				fmt.Fprintf(os.Stdout, "Past %d:%s\n", i, bl.Tips[i])
			}
			for i, mbl := range bl.MiniBlocks {
				fmt.Fprintf(os.Stdout, "Mini %d:%s %s\n", i, mbl, header.Miners[i])
			}
			for i, txid := range bl.Tx_hashes {
				fmt.Fprintf(os.Stdout, "tx %d:%s\n", i, txid)
			}

			fmt.Printf("difficulty: %s\n", chain.Load_Block_Difficulty(hash).String())
			fmt.Printf("TopoHeight: %d\n", chain.Load_Block_Topological_order(hash))

			version, err := chain.ReadBlockSnapshotVersion(hash)
			if err != nil {
				panic(err)
			}

			bhash, err := chain.Load_Merkle_Hash(version)
			if err != nil {
				panic(err)
			}

			fmt.Printf("BALANCE_TREE : %s\n", bhash)
			fmt.Printf("MINING REWARD : %s\n", globals.FormatMoney(blockchain.CalcBlockReward(bl.Height)))

			//fmt.Printf("Orphan: %v\n",chain.Is_Block_Orphan(hash))

			//json_bytes, err := json.Marshal(bl)

			//fmt.Printf("%s  err : %s\n", string(prettyprint_json(json_bytes)), err)

		// can be used to debug/deserialize blocks
		// it can be used for blocks not in chain
		case command == "parse_block":

			if len(line_parts) != 2 {
				logger.Info("parse_block needs a block in hex format")
				continue
			}

			block_raw, err := hex.DecodeString(strings.ToLower(line_parts[1]))
			if err != nil {
				fmt.Printf("err while hex decoding block err %s\n", err)
				continue
			}

			var bl block.Block
			err = bl.Deserialize(block_raw)
			if err != nil {
				logger.Error(err, "Error deserializing block")
				continue
			}

			// decode and print block as much as possible
			fmt.Printf("%s\n", bl.String())
			fmt.Printf("Height: %d\n", bl.Height)
			tips_found := true
			for i := range bl.Tips {
				_, err := chain.Load_BL_FROM_ID(bl.Tips[i])
				if err != nil {
					fmt.Printf("Tips %s not in our DB", bl.Tips[i])
					tips_found = false
					continue
				}
			}

			expected_difficulty := new(big.Int).SetUint64(0)
			if tips_found { // we can solve diffculty
				expected_difficulty = chain.Get_Difficulty_At_Tips(bl.Tips)
				fmt.Printf("Difficulty:  %s\n", expected_difficulty.String())
			} else { // difficulty cann not solved

			}

		case command == "print_tx":
			if len(line_parts) == 2 && len(line_parts[1]) == 64 {
				txid, err := hex.DecodeString(strings.ToLower(line_parts[1]))

				if err != nil {
					fmt.Printf("err while decoding txid err %s\n", err)
					continue
				}
				var hash crypto.Hash
				copy(hash[:32], []byte(txid))

				var tx transaction.Transaction
				if tx_bytes, err := chain.Store.Block_tx_store.ReadTX(hash); err != nil {
					fmt.Printf("err while reading txid err %s\n", err)
					continue
				} else if err = tx.Deserialize(tx_bytes); err != nil {
					fmt.Printf("err deserializing tx err %s\n", err)
					continue
				}

				if valid_blid, invalid, valid := chain.IS_TX_Valid(hash); valid {
					fmt.Printf("TX is valid in block %s\n", valid_blid)
				} else if len(invalid) == 0 {
					fmt.Printf("TX is mined in a side chain\n")
				} else {
					fmt.Printf("TX is mined in blocks %+v\n", invalid)
				}
				if tx.IsRegistration() {
					fmt.Printf("Registration TX validity could not be detected\n")
				}

			} else {
				fmt.Printf("print_tx  needs a single transaction id as arugument\n")
			}

		case strings.ToLower(line) == "status":
			inc, out := p2p.Peer_Direction_Count()

			mempool_tx_count := len(chain.Mempool.Mempool_List_TX())
			regpool_tx_count := len(chain.Regpool.Regpool_List_TX())

			supply := uint64(0)

			supply = (config.PREMINE + blockchain.CalcBlockReward(uint64(chain.Get_Height()))*uint64(chain.Get_Height())) // valid for few years

			hostname, _ := os.Hostname()
			fmt.Printf("STATUS MENU for DERO HE Node - Hostname: %s\n\n", hostname)
			fmt.Printf("Hostname: %s - Uptime: %s\n", hostname, time.Now().Sub(globals.StartTime).Round(time.Second).String())
			fmt.Printf("Uptime Since: %s\n\n", globals.StartTime.Format(time.RFC1123))

			fmt.Printf("Network %s Height %d  NW Hashrate %0.03f MH/sec  Peers %d inc, %d out  MEMPOOL size %d REGPOOL %d  Total Supply %s DERO \n", globals.Config.Name, chain.Get_Height(), float64(chain.Get_Network_HashRate())/1000000.0, inc, out, mempool_tx_count, regpool_tx_count, globals.FormatMoney(supply))

			tips := chain.Get_TIPS()
			fmt.Printf("Tips ")
			for _, tip := range tips {
				fmt.Printf(" %s(%d)", tip, chain.Load_Height_for_BL_ID(tip))
			}

			if chain.LocatePruneTopo() >= 1 {
				fmt.Printf("\nChain is pruned till %d\n", chain.LocatePruneTopo())
			} else {
				fmt.Printf("\nChain is in full mode.\n")
			}
			fmt.Printf("Integrator address %s\n", chain.IntegratorAddress().String())
			fmt.Printf("UTC time %s  (as per system clock) \n", time.Now().UTC())
			fmt.Printf("UTC time %s  (offset %s) (as per daemon) should be close to 0\n", globals.Time().UTC(), time.Now().Sub(globals.Time()))
			fmt.Printf("Local time %s  (as per system clock) \n", time.Now())
			fmt.Printf("Local time %s  (offset %s) (as per daemon) should be close to 0\n", globals.Time(), time.Now().Sub(globals.Time()))

			fmt.Print("\nPeer Stats:\n")
			fmt.Printf("\tPeer ID: %d\n", p2p.GetPeerID())

			blocksMined := (derodrpc.CountMinisAccepted + derodrpc.CountBlocks) - derodrpc.CountMinisRejected
			fmt.Print("\nMining Stats:\n")
			fmt.Printf("\tBlock Minted: %d (MB+IB)\n", (derodrpc.CountMinisAccepted+derodrpc.CountBlocks)-derodrpc.CountMinisRejected)
			if blocksMined > 0 {
				fmt.Printf("\tMinting Velocity: %.4f MB/h\t%.4f MB/d (since uptime)\n", float64(float64(derodrpc.CountMinisAccepted)/time.Now().Sub(globals.StartTime).Seconds())*3600,
					float64(float64(derodrpc.CountMinisAccepted)/time.Now().Sub(globals.StartTime).Seconds())*3600*24)
			} else {
				fmt.Print("\tMinting Velocity: 0.0000 MB/h\t0.0000MB/d (since uptime)\n")
			}
			//if derodrpc.CountMiners() > 0 { // only give info if we have a miner connected
			fmt.Printf("\tMB:%d MBR:%d IB:%d\n", derodrpc.CountMinisAccepted, derodrpc.CountMinisRejected, derodrpc.CountBlocks)
			fmt.Printf("\tMB %.02f%%(1hr)\t%.05f%%(1d)\t%.06f%%(7d)\t(Moving average %%, will be 0 if no miniblock found)\n", derodrpc.HashrateEstimatePercent_1hr(), derodrpc.HashrateEstimatePercent_1day(), derodrpc.HashrateEstimatePercent_7day())
			mh_1hr := uint64((float64(chain.Get_Network_HashRate()) * derodrpc.HashrateEstimatePercent_1hr()) / 100)
			mh_1d := uint64((float64(chain.Get_Network_HashRate()) * derodrpc.HashrateEstimatePercent_1day()) / 100)
			mh_7d := uint64((float64(chain.Get_Network_HashRate()) * derodrpc.HashrateEstimatePercent_7day()) / 100)
			fmt.Printf("\tAvg Mining HR %s(1hr)\t%s(1d)\t%s(7d)\n", hashratetostring(mh_1hr), hashratetostring(mh_1d), hashratetostring(mh_7d))
			//}

			fmt.Printf("\n")
			fmt.Printf("Current Block Reward: %s\n", globals.FormatMoney(blockchain.CalcBlockReward(uint64(chain.Get_Height()))))
			fmt.Printf("\n")

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

		case strings.ToLower(line) == "peer_list": // print peer list
			p2p.PeerList_Print()
		case strings.ToLower(line) == "syncinfo", strings.ToLower(line) == "sync_info": // print active connections
			p2p.Connection_Print()
		case strings.ToLower(line) == "bye":
			fallthrough
		case strings.ToLower(line) == "exit":
			fallthrough
		case strings.ToLower(line) == "quit":
			close(Exit_In_Progress)
			return nil

		case command == "graph":
			start := int64(0)
			stop := int64(0)

			if len(line_parts) != 3 {
				logger.Info("This function requires 2 parameters, start height and end height\n")
				continue
			}
			if s, err := strconv.ParseInt(line_parts[1], 10, 64); err == nil {
				start = s
			} else {
				logger.Error(err, "Invalid start value")
				continue
			}

			if s, err := strconv.ParseInt(line_parts[2], 10, 64); err == nil {
				stop = s
			} else {
				logger.Error(err, "Invalid stop value")
				continue
			}

			if start < 0 || start > int64(chain.Load_TOPO_HEIGHT()) {
				logger.Info("Start value should be be between 0 and current height")
				continue
			}
			if start > stop || stop > int64(chain.Load_TOPO_HEIGHT()) {
				logger.Info("Stop value should be > start and current height\n")
				continue
			}

			logger.Info("Writing block chain graph dot format  /tmp/graph.dot\n", "start", start, "stop", stop)
			WriteBlockChainTree(chain, "/tmp/graph.dot", start, stop)

		case command == "pop":
			switch len(line_parts) {
			case 1:
				chain.Rewind_Chain(1)
			case 2:
				pop_count := 0
				if s, err := strconv.Atoi(line_parts[1]); err == nil {
					pop_count = s

					if chain.Rewind_Chain(int(pop_count)) {
						logger.Info("Rewind successful")
					} else {
						logger.Error(fmt.Errorf("Rewind failed"), "")
					}

				} else {
					logger.Error(fmt.Errorf("POP needs argument n to pop this many blocks from the top"), "")
				}

			default:
				logger.Error(fmt.Errorf("POP needs argument n to pop this many blocks from the top"), "")
			}

		case command == "gc":
			runtime.GC()
		case command == "heap":
			if len(line_parts) == 1 {
				fmt.Printf("heap needs a filename to write\n")
				break
			}
			dump(line_parts[1])

		case command == "ban":

			if len(line_parts) >= 4 || len(line_parts) == 1 {
				fmt.Printf("IP address required to ban\n")
				break
			}

			if len(line_parts) == 3 { // process ban time if provided
				// if user provided a time, apply ban for specific time
				if s, err := strconv.ParseInt(line_parts[2], 10, 64); err == nil && s >= 0 {
					p2p.Ban_Address(line_parts[1], uint64(s))
					break
				} else {
					fmt.Printf("err parsing ban time (only positive number) %s", err)
					break
				}
			}

			err := p2p.Ban_Address(line_parts[1], 10*60) // default ban is 10 minutes
			if err != nil {
				fmt.Printf("err parsing address %s", err)
				break
			}

		case command == "unban":

			if len(line_parts) >= 3 || len(line_parts) == 1 {
				fmt.Printf("IP address required to unban\n")
				break
			}

			err := p2p.UnBan_Address(line_parts[1])
			if err != nil {
				fmt.Printf("err unbanning %s, err = %s", line_parts[1], err)
			} else {
				fmt.Printf("unbann %s successful", line_parts[1])
			}

		case command == "bans":
			p2p.BanList_Print() // print ban list

		case line == "sleep":
			logger.Info("console sleeping for 1 second")
			time.Sleep(1 * time.Second)
		case line == "":
		default:
			logger.Info(fmt.Sprintf("you said: %s", strconv.Quote(line)))
		}
	}

	return fmt.Errorf("can never reach here")

}

func writenode(chain *blockchain.Blockchain, w *bufio.Writer, blid crypto.Hash, start_height int64) { // process a node, recursively

	w.WriteString(fmt.Sprintf("node [ fontsize=12 style=filled ]\n{\n"))

	color := "white"

	if chain.Isblock_SideBlock(blid) {
		color = "yellow"
	}
	if chain.IsBlockSyncBlockHeight(blid) {
		color = "green"
	}

	// now dump the interconnections
	bl, err := chain.Load_BL_FROM_ID(blid)

	var acckey crypto.Point
	if err := acckey.DecodeCompressed(bl.Miner_TX.MinerAddress[:]); err != nil {
		panic(err)
	}

	addr := rpc.NewAddressFromKeys(&acckey)
	addr.Mainnet = globals.IsMainnet()

	w.WriteString(fmt.Sprintf("L%s  [ fillcolor=%s label = \"%s %d height %d score %d  order %d\nminer %s\"  ];\n", blid.String(), color, blid.String(), 0, chain.Load_Height_for_BL_ID(blid), 0, chain.Load_Block_Topological_order(blid), addr.String()))
	w.WriteString(fmt.Sprintf("}\n"))

	if err != nil {
		fmt.Printf("err loading block %s err %s\n", blid, err)
		return
	}
	if int64(bl.Height) > start_height {
		for i := range bl.Tips {
			w.WriteString(fmt.Sprintf("L%s -> L%s ;\n", bl.Tips[i].String(), blid.String()))
		}
	}

}

func hashratetostring(hash_rate uint64) string {
	hash_rate_string := ""

	switch {
	case hash_rate > 1000000000000:
		hash_rate_string = fmt.Sprintf("%.3f TH/s", float64(hash_rate)/1000000000000.0)
	case hash_rate > 1000000000:
		hash_rate_string = fmt.Sprintf("%.3f GH/s", float64(hash_rate)/1000000000.0)
	case hash_rate > 1000000:
		hash_rate_string = fmt.Sprintf("%.3f MH/s", float64(hash_rate)/1000000.0)
	case hash_rate > 1000:
		hash_rate_string = fmt.Sprintf("%.3f KH/s", float64(hash_rate)/1000.0)
	case hash_rate > 0:
		hash_rate_string = fmt.Sprintf("%d H/s", hash_rate)
	}
	return hash_rate_string
}

func WriteBlockChainTree(chain *blockchain.Blockchain, filename string, start_height, stop_height int64) (err error) {

	var node_map = map[crypto.Hash]bool{}

	for i := start_height; i < stop_height; i++ {
		blids := chain.Get_Blocks_At_Height(i)

		for _, blid := range blids {
			if _, ok := node_map[blid]; ok {
				panic("duplicate block should not be there")
			} else {
				node_map[blid] = true
			}
		}
	}

	f, err := os.Create(filename)
	if err != nil {
		return
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	defer w.Flush()
	w.WriteString("digraph dero_blockchain_graph { \n")

	for blid := range node_map {
		writenode(chain, w, blid, start_height)
	}
	//g := Generate_Genesis_Block()
	//writenode(chain, dbtx, w, g.GetHash())

	w.WriteString("}\n")

	return
}

func prettyprint_json(b []byte) []byte {
	var out bytes.Buffer
	err := json.Indent(&out, b, "", "  ")
	_ = err
	return out.Bytes()
}

func usage(w io.Writer) {
	io.WriteString(w, "commands:\n")
	io.WriteString(w, "\t\033[1mhelp\033[0m\t\tthis help\n")
	io.WriteString(w, "\t\033[1mdiff\033[0m\t\tShow difficulty\n")
	io.WriteString(w, "\t\033[1mprint_bc\033[0m\tPrint blockchain info in a given blocks range, print_bc <begin_height> <end_height>\n")
	io.WriteString(w, "\t\033[1mprint_block\033[0m\tPrint block, print_block <block_hash> or <block_height>\n")
	io.WriteString(w, "\t\033[1mprint_tx\033[0m\tPrint transaction, print_tx <transaction_hash>\n")
	io.WriteString(w, "\t\033[1mstatus\033[0m\t\tShow general information\n")
	io.WriteString(w, "\t\033[1mpeer_list\033[0m\tPrint peer list\n")
	io.WriteString(w, "\t\033[1msyncinfo\033[0m\tPrint information about connected peers and their state\n")
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
	io.WriteString(w, "\t\033[1msetintegratoraddress\033[0m\t\tChange current integrated address\n")

	io.WriteString(w, "\t\033[1mversion\033[0m\t\tShow version\n")
	io.WriteString(w, "\t\033[1mexit\033[0m\t\tQuit the daemon\n")
	io.WriteString(w, "\t\033[1mquit\033[0m\t\tQuit the daemon\n")

}

var completer = readline.NewPrefixCompleter(
	readline.PcItem("help"),
	readline.PcItem("diff"),
	readline.PcItem("gc"),
	readline.PcItem("mempool_dump"),
	readline.PcItem("mempool_flush"),
	readline.PcItem("mempool_delete_tx"),
	readline.PcItem("mempool_print"),
	readline.PcItem("regpool_flush"),
	readline.PcItem("regpool_delete_tx"),
	readline.PcItem("regpool_print"),
	readline.PcItem("peer_list"),
	readline.PcItem("print_bc"),
	readline.PcItem("print_block"),
	readline.PcItem("block_export"),
	readline.PcItem("block_import"),
	//	readline.PcItem("print_tx"),
	readline.PcItem("setintegratoraddress"),
	readline.PcItem("status"),
	readline.PcItem("syncinfo"),
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
