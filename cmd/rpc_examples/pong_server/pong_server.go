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

import "os"
import "fmt"
import "time"
import "crypto/sha1"

import "go.etcd.io/bbolt"

import "github.com/go-logr/logr"
import "gopkg.in/natefinch/lumberjack.v2"
import "github.com/deroproject/derohe/globals" // needed for logs

import "github.com/deroproject/derohe/rpc"
import "github.com/deroproject/derohe/walletapi"
import "github.com/ybbus/jsonrpc"

var logger logr.Logger = logr.Discard() // default discard all logs

const PLUGIN_NAME = "pong_server"

const DEST_PORT = uint64(0x1234567812345678)

var expected_arguments = rpc.Arguments{
	{Name: rpc.RPC_DESTINATION_PORT, DataType: rpc.DataUint64, Value: DEST_PORT},
	// { Name:rpc.RPC_EXPIRY , DataType:rpc.DataTime, Value:time.Now().Add(time.Hour).UTC()},
	{Name: rpc.RPC_COMMENT, DataType: rpc.DataString, Value: "Purchase PONG"},
	//{"float64", rpc.DataFloat64, float64(0.12345)},          // in atomic units
	//	{Name:rpc.RPC_NEEDS_REPLYBACK_ADDRESS,DataType:rpc.DataUint64,Value:uint64(0)},  // this service will reply to incoming request,so needs the senders address
	{Name: rpc.RPC_VALUE_TRANSFER, DataType: rpc.DataUint64, Value: uint64(12345)}, // in atomic units

}

// currently the interpreter seems to have a glitch if this gets initialized within the code
// see limitations github.com/traefik/yaegi
var response = rpc.Arguments{
	{Name: rpc.RPC_DESTINATION_PORT, DataType: rpc.DataUint64, Value: uint64(0)},
	{Name: rpc.RPC_SOURCE_PORT, DataType: rpc.DataUint64, Value: DEST_PORT},
	{Name: rpc.RPC_COMMENT, DataType: rpc.DataString, Value: "Successfully purchased pong (this could be serial/license key or download link or further)"},
}

var rpcClient = jsonrpc.NewClient("http://127.0.0.1:40403/json_rpc")

// empty place holder

func main() {
	var err error
	fmt.Printf("Pong Server to demonstrate RPC over dero chain.\n")

	// parse arguments and setup logging , print basic information
	globals.Arguments["--debug"] = true
	exename, _ := os.Executable()
	globals.InitializeLog(os.Stdout, &lumberjack.Logger{
		Filename:   exename + ".log",
		MaxSize:    100, // megabytes
		MaxBackups: 2,
	})
	logger = globals.Logger

	var addr *rpc.Address
	var addr_result rpc.GetAddress_Result
	err = rpcClient.CallFor(&addr_result, "GetAddress")
	if err != nil || addr_result.Address == "" {
		fmt.Printf("Could not obtain address from wallet err %s\n", err)
		return
	}

	if addr, err = rpc.NewAddress(addr_result.Address); err != nil {
		fmt.Printf("address could not be parsed: addr:%s err:%s\n", addr_result.Address, err)
		return
	}

	shasum := fmt.Sprintf("%x", sha1.Sum([]byte(addr.String())))

	db_name := fmt.Sprintf("%s_%s.bbolt.db", PLUGIN_NAME, shasum)
	db, err := bbolt.Open(db_name, 0600, nil)
	if err != nil {
		fmt.Printf("could not open db err:%s\n", err)
		return
	}
	//defer db.Close()

	err = db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("SALE"))
		return err
	})
	if err != nil {
		fmt.Printf("err creating bucket. err %s\n", err)
	}

	fmt.Printf("Persistant store created in '%s'\n", db_name)

	fmt.Printf("Wallet Address: %s\n", addr)
	service_address_without_amount := addr.Clone()
	service_address_without_amount.Arguments = expected_arguments[:len(expected_arguments)-1]
	fmt.Printf("Integrated address to activate '%s', (without hardcoded amount) service: \n%s\n", PLUGIN_NAME, service_address_without_amount.String())

	// service address can be created client side for now
	service_address := addr.Clone()
	service_address.Arguments = expected_arguments
	fmt.Printf("Integrated address to activate '%s', service: \n%s\n", PLUGIN_NAME, service_address.String())

	processing_thread(db) // rkeep processing

	//time.Sleep(time.Second)
	//return
}

func processing_thread(db *bbolt.DB) {

	var err error

	for { // currently we traverse entire history

		time.Sleep(time.Second)

		var transfers rpc.Get_Transfers_Result
		err = rpcClient.CallFor(&transfers, "GetTransfers", rpc.Get_Transfers_Params{In: true, DestinationPort: DEST_PORT})
		if err != nil {
			logger.Error(err, "Could not obtain gettransfers from wallet")
			continue
		}

		for _, e := range transfers.Entries {
			if e.Coinbase || !e.Incoming { // skip coinbase or outgoing, self generated transactions
				continue
			}

			// check whether the entry has been processed before, if yes skip it
			var already_processed bool
			db.View(func(tx *bbolt.Tx) error {
				if b := tx.Bucket([]byte("SALE")); b != nil {
					if ok := b.Get([]byte(e.TXID)); ok != nil { // if existing in bucket
						already_processed = true
					}
				}
				return nil
			})

			if already_processed { // if already processed skip it
				continue
			}

			// check whether this service should handle the transfer
			if !e.Payload_RPC.Has(rpc.RPC_DESTINATION_PORT, rpc.DataUint64) ||
				DEST_PORT != e.Payload_RPC.Value(rpc.RPC_DESTINATION_PORT, rpc.DataUint64).(uint64) { // this service is expecting value to be specfic
				continue

			}

			logger.V(1).Info("tx should be processed", "txid", e.TXID)

			if expected_arguments.Has(rpc.RPC_VALUE_TRANSFER, rpc.DataUint64) { // this service is expecting value to be specfic
				value_expected := expected_arguments.Value(rpc.RPC_VALUE_TRANSFER, rpc.DataUint64).(uint64)
				if e.Amount != value_expected { // TODO we should mark it as faulty
					logger.Error(nil, fmt.Sprintf("user transferred %d, we were expecting %d. so we will not do anything", e.Amount, value_expected)) // this is an unexpected situation
					continue
				}

				/*	if !e.Payload_RPC.Has(rpc.RPC_REPLYBACK_ADDRESS, rpc.DataAddress){
						logger.Error(nil, fmt.Sprintf("user has not give his address so we cannot replyback")) // this is an unexpected situation
						continue
					}

					destination_expected := e.Payload_RPC.Value(rpc.RPC_REPLYBACK_ADDRESS, rpc.DataAddress).(rpc.Address).String()

					logger.V(1).Info("tx should be replied", "txid", e.TXID,"replyback_address",destination_expected)
				*/
				destination_expected := e.Sender

				// value received is what we are expecting, so time for response
				response[0].Value = e.SourcePort // source port now becomes destination port, similar to TCP
				response[2].Value = fmt.Sprintf("Sucessfully purchased pong (could be serial, license or download link or anything).You sent %s at height %d", walletapi.FormatMoney(e.Amount), e.Height)

				//_, err :=  response.CheckPack(transaction.PAYLOAD0_LIMIT)) //  we only have 144 bytes for RPC

				// sender of ping now becomes destination
				var result rpc.Transfer_Result
				tparams := rpc.Transfer_Params{Transfers: []rpc.Transfer{{Destination: destination_expected, Amount: uint64(1), Payload_RPC: response}}}
				err = rpcClient.CallFor(&result, "Transfer", tparams)
				if err != nil {
					logger.Error(err, "err while transfer")
					continue
				}

				err = db.Update(func(tx *bbolt.Tx) error {
					b := tx.Bucket([]byte("SALE"))
					return b.Put([]byte(e.TXID), []byte("done"))
				})
				if err != nil {
					logger.Error(err, "err updating db")
				} else {
					logger.Info("ping replied successfully with pong ", "result", result)
				}

			}
		}

	}
}
