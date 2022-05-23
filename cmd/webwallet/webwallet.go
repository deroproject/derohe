// +build js,wasm

package main

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"io"
	//"io/ioutil"
	//"log"
	//"net/http"
	"fmt"
	"net/url"
	"strconv"
	"syscall/js"
	"time"
	// "bytes"
    "runtime"
	"runtime/debug"
	"strings"
)
import "github.com/go-logr/logr"

import "github.com/deroproject/derohe/walletapi"
import "github.com/deroproject/derohe/globals"
import "github.com/deroproject/derohe/config"
import "github.com/deroproject/derohe/rpc"
import "github.com/deroproject/derohe/transaction"
import "github.com/deroproject/derohe/cryptography/crypto"

var miner_tx bool = false

var logger logr.Logger

var Local_wallet_instance *walletapi.Wallet_Memory

func register_wallet_callbacks() {

    // this function is used to ping to keep things working
	js.Global().Set("go_pinger", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
        fmt.Println("go_pinger called")
        return nil
    }))

 
	js_Create_New_Wallet := func(this js.Value, args []js.Value) interface{} {
		error_message := "success"
		//filename := args[0].String()
		password := args[1].String()

		if w, err := walletapi.Create_Encrypted_Wallet_Random_Memory( password); err == nil {
			error_message = "success"
			Local_wallet_instance = w
			Local_wallet_instance.SetDaemonAddress(daemon_address)
			Local_wallet_instance.SetNetwork(globals.IsMainnet()) // set mainnet/testnet
		} else {
			error_message = err.Error()
		}
        return error_message
	}
	js.Global().Set("DERO_JS_Create_New_Wallet", js.FuncOf(js_Create_New_Wallet))

	js_Create_Encrypted_Wallet_From_Recovery_Words := func(this js.Value, args []js.Value) interface{} {
		error_message := "error"

		w, err := walletapi.Create_Encrypted_Wallet_From_Recovery_Words_Memory(args[1].String(), args[2].String())

		if err == nil {
			error_message = "success"
			Local_wallet_instance = w
			Local_wallet_instance.SetDaemonAddress(daemon_address)
            Local_wallet_instance.SetNetwork(globals.IsMainnet()) // set mainnet/testnet
		} else {
			error_message = err.Error()
		}

		js.Global().Set("error_message", error_message)
        return nil
	}
	js.Global().Set("DERO_JS_Create_Encrypted_Wallet_From_Recovery_Words", js.FuncOf(js_Create_Encrypted_Wallet_From_Recovery_Words))

	js_Open_Encrypted_Wallet := func(this js.Value, args []js.Value) interface{} {

		error_message := "error"

		// convert typed array to go array
		// this may be slow and needs to be optimized
		// as optimization we are converting the data in javascript to hex
		// and here we are hex decoding as it is faster than converting each value of typed array
		// TODO: later when this gets fixed by go devs, we can incorporate it


		src := []byte(args[2].String())
		db_array := make([]byte, hex.DecodedLen(len(src)))
		n, err := hex.Decode(db_array, src)
		db_array = db_array[:n]

		if err != nil {

			logger.Error(err,"error decoding hex string")
		}

		logger.Info("passed DB", "size", len(db_array))
		w, err := walletapi.Open_Encrypted_Wallet_Memory( args[1].String(), db_array)
		if err == nil {
			error_message = "success"
			Local_wallet_instance = w
			Local_wallet_instance.SetDaemonAddress(daemon_address)
            Local_wallet_instance.SetNetwork(globals.IsMainnet()) // set mainnet/testnet

			logger.Info("Successfully opened wallet")
		} else {
			error_message = err.Error()

			logger.Error(err,"Error opened wallet")
		}

		js.Global().Set("error_message", error_message)
        return nil
	}
	js.Global().Set("DERO_JS_Open_Encrypted_Wallet", js.FuncOf(js_Open_Encrypted_Wallet))

	js_Create_Wallet := func(this js.Value, args []js.Value) interface{} {

		//filename := args[0].String()
		password := args[1].String()
		seed_hex := args[2].String()
		error_message := "error"

		var seed crypto.Key
		seed_raw, err := hex.DecodeString(strings.TrimSpace(seed_hex))
		if len(seed_raw) != 32 || err != nil {
			err = fmt.Errorf("Recovery Only key must be 64 chars hexadecimal chars")
			logger.Error(err,"recovery failed")
			error_message = err.Error()
		} else {

			copy(seed[:], seed_raw[:32])
			wallet, err := walletapi.Create_Encrypted_Wallet_Memory( password, new(crypto.BNRed).SetBytes(seed[:]))

			if err != nil {
				error_message = err.Error()
			} else {
				error_message = "success"
				Local_wallet_instance = wallet
				Local_wallet_instance.SetDaemonAddress(daemon_address)
                 Local_wallet_instance.SetNetwork(globals.IsMainnet()) // set mainnet/testnet
			}
		}

		js.Global().Set("error_message", error_message)
    return nil
	}
	js.Global().Set("DERO_JS_Create_Wallet", js.FuncOf(js_Create_Wallet))



    // generate integrated address at user demand
    js_GenerateIntegratedAddress :=  func(this js.Value, args []js.Value) interface{} {
		if Local_wallet_instance != nil {
		i8 := Local_wallet_instance.GetRandomIAddress8()
		js.Global().Set("random_i8_address", i8.String())
		//js.Global().Set("random_i8_address_paymentid", fmt.Sprintf("%x", i8.PaymentID))
	}
    return nil
    }


	js.Global().Set("DERO_JS_GenerateIAddress", js.FuncOf(js_GenerateIntegratedAddress))

	js_GetSeedinLanguage :=  func(this js.Value, args []js.Value) interface{} {
		seed := "Some error occurred"
		if Local_wallet_instance != nil && len(args) == 1 {
			seed = Local_wallet_instance.GetSeedinLanguage(args[0].String())
		}
		js.Global().Set("wallet_seed", seed)
        return nil
	}
	js.Global().Set("DERO_JS_GetSeedinLanguage", js.FuncOf(js_GetSeedinLanguage))

	js_TX_history :=  func(this js.Value, args []js.Value) interface{} {
			error_message := "Wallet is Closed"
			var buffer []byte
			var err error

			defer func() {
				js.Global().Set("tx_history", string(buffer))
				js.Global().Set("error_message", error_message)
			}()

			if Local_wallet_instance != nil {

				min_height, _ := strconv.ParseUint(args[6].String(), 0, 64)
				max_height, _ := strconv.ParseUint(args[7].String(), 0, 64)

                var zeroscid crypto.Hash

				entries := Local_wallet_instance.Show_Transfers(zeroscid,args[0].Bool(), args[1].Bool(), args[2].Bool(),0,0, "args[3].Bool()", "args[4].Bool()",  min_height, max_height)

				if len(entries) == 0 {
					return nil
				}
				buffer, err = json.Marshal(entries)
				if err != nil {
					error_message = err.Error()
					return nil
				}}
           return nil
		
	}
	js.Global().Set("DERO_JS_TX_History", js.FuncOf(js_TX_history))


	js_Transfer2 := func( args []js.Value) interface{} {
		transfer_error := "error"
		var transfer_txid, transfer_txhex, transfer_fee, transfer_amount, transfer_inputs_sum, transfer_change string

		defer func() {
			fmt.Printf("setting values of tranfer variables")
			js.Global().Set("transfer_txid", transfer_txid)
			js.Global().Set("transfer_txhex", transfer_txhex)
			js.Global().Set("transfer_amount", transfer_amount)
			js.Global().Set("transfer_fee", transfer_fee)
			js.Global().Set("transfer_inputs_sum", transfer_inputs_sum)
			js.Global().Set("transfer_change", transfer_change)
			js.Global().Set("transfer_error", transfer_error)
			//rlog.Warnf("setting values of tranfesr variables %s ", transfer_error)
		}()

		

		var transfers []rpc.Transfer

		if args[0].Length() != args[1].Length() {
			fmt.Printf("Destination and amount mismatch")
			return nil
		}

		for i := 0; i < args[0].Length(); i++ { // convert string address to our native form
			_, err := globals.ParseValidateAddress(args[0].Index(i).String())
			if err != nil {
				if _, err1 := Local_wallet_instance.NameToAddress(string(strings.TrimSpace(string(args[0].Index(i).String())))); err1 != nil {
					transfer_error = err.Error()
					logger.Error(err,"Parsing address failed",  "addr", args[0].Index(i).String())
					transfer_error = err.Error()
					return nil
				} else {

				}
			}

			amount, err := globals.ParseAmount(args[1].Index(i).String())
			if err != nil {
				logger.Error(err,"Parsing amount failed", "amount", args[0].Index(i).String())
				transfer_error = err.Error()
				return nil
				//return nil, jsonrpc.ErrInvalidParams()
			}

			transfers = append(transfers, rpc.Transfer{Destination:args[0].Index(i).String(), Amount:amount})
		}

	
		fmt.Printf("address parsed, building tx")
		ringsize := uint64(0)

		tx,  err := Local_wallet_instance.TransferPayload0(transfers, ringsize, false, rpc.Arguments{}, 0,false)
	
		if err != nil {
			logger.Error(err,"Error while building Transaction err")
			transfer_error = err.Error()
			return nil
			//return nil, &jsonrpc.Error{Code: -2, Message: fmt.Sprintf("Error while building Transaction err %s", err)}

		}



		
		amount := uint64(0)
		for i := range transfers {
			amount += transfers[i].Amount
		}

		transfer_fee = globals.FormatMoney5(tx.Fees())
		transfer_amount = globals.FormatMoney5(amount)
		transfer_change = "0"
		transfer_inputs_sum = "0"
		transfer_txid = tx.GetHash().String()
		transfer_txhex = hex.EncodeToString(tx.Serialize())
		transfer_error = "success"
		return nil
	}

	js_Transfer := func(this js.Value,args []js.Value) interface{}{

		go js_Transfer2(args)
		return nil
	}
	js.Global().Set("DERO_JS_Transfer", js.FuncOf(js_Transfer))

/*
	js_Transfer_Everything2 := func(this js.Value, args []js.Value) interface{} {
		transfer_error := "error"
		var transfer_txid, transfer_txhex, transfer_fee, transfer_amount, transfer_inputs_sum, transfer_change string

		defer func() {
			rlog.Warnf("setting values of tranfer variables")
			js.Global().Set("transfer_txid", transfer_txid)
			js.Global().Set("transfer_txhex", transfer_txhex)
			js.Global().Set("transfer_amount", transfer_amount)
			js.Global().Set("transfer_fee", transfer_fee)
			js.Global().Set("transfer_inputs_sum", transfer_inputs_sum)
			js.Global().Set("transfer_change", transfer_change)
			js.Global().Set("transfer_error", transfer_error)
			rlog.Warnf("setting values of tranfesr variables %s ", transfer_error)
		}()

		var address_list []address.Address
		var amount_list []uint64

		if params[0].Length() != 1 {
			return
		}

		for i := 0; i < params[0].Length(); i++ { // convert string address to our native form
			a, err := globals.ParseValidateAddress(params[0].Index(i).String())
			if err != nil {
				rlog.Warnf("Parsing address failed %s err %s\n", params[0].Index(i).String(), err)
				transfer_error = err.Error()
				return
				//return nil, jsonrpc.ErrInvalidParams()
			}
			address_list = append(address_list, *a)
		}

		payment_id := params[1].String()

		if len(payment_id) > 0 && !(len(payment_id) == 64 || len(payment_id) == 16) {
			transfer_error = "Invalid payment ID"
			return // we should give invalid payment ID
		}
		if _, err := hex.DecodeString(payment_id); err != nil {
			transfer_error = "Invalid payment ID"
			return // we should give invalid payment ID
		}

		//unlock_time := uint64(0)
		fees_per_kb := uint64(0)
		mixin := uint64(0)

		tx, inputs, input_sum, err := Local_wallet_instance.Transfer_Everything(address_list[0], payment_id, 0, fees_per_kb, mixin)
		_ = inputs
		if err != nil {
			rlog.Warnf("Error while building Everything Transaction err %s\n", err)
			transfer_error = err.Error()
			return
			//return nil, &jsonrpc.Error{Code: -2, Message: fmt.Sprintf("Error while building Transaction err %s", err)}

		}

		rlog.Infof("Inputs Selected for %s \n", globals.FormatMoney(input_sum))
		amount := uint64(0)
		for i := range amount_list {
			amount += amount_list[i]
		}
		amount = uint64(input_sum - tx.RctSignature.Get_TX_Fee())
		change := uint64(0)
		rlog.Infof("Transfering everything total amount %s \n", globals.FormatMoney(amount))
		rlog.Infof("change amount ( will come back ) %s \n", globals.FormatMoney(change))
		rlog.Infof("fees %s \n", globals.FormatMoney(tx.RctSignature.Get_TX_Fee()))

		rlog.Infof(" size of tx %d", len(hex.EncodeToString(tx.Serialize())))

		transfer_fee = globals.FormatMoney12(tx.RctSignature.Get_TX_Fee())
		transfer_amount = globals.FormatMoney12(amount)
		transfer_change = globals.FormatMoney12(change)
		transfer_inputs_sum = globals.FormatMoney12(input_sum)
		transfer_txid = tx.GetHash().String()
		transfer_txhex = hex.EncodeToString(tx.Serialize())
		transfer_error = "success"
	}

	js_Transfer_Everything := func(params []js.Value) {
		go js_Transfer_Everything2(params)
	}
	js.Global().Set("DERO_JS_Transfer_Everything", js.NewCallback(js_Transfer_Everything))
*/
	js_Relay_TX2 := func( args []js.Value)  {
		hex_tx := strings.TrimSpace(args[0].String())
		tx_bytes, err := hex.DecodeString(hex_tx)
		if err != nil {
			js.Global().Set("relay_error", fmt.Sprintf("Transaction Could NOT be hex decoded err %s", err))
			return
		}

		var tx transaction.Transaction

		err = tx.Deserialize(tx_bytes)
		if err != nil {
			js.Global().Set("relay_error", fmt.Sprintf("Transaction Could NOT be deserialized err %s", err))
			return
		}

		err = Local_wallet_instance.SendTransaction(&tx) // relay tx to daemon/network

		if err != nil {
			js.Global().Set("relay_error", fmt.Sprintf("Transaction sending failed txid = %s, err %s", tx.GetHash(), err))
			return
		}
		js.Global().Set("relay_error", "success")
	}

	js_Relay_TX := func(this js.Value,params []js.Value) interface{} {
		go js_Relay_TX2(params)
		return nil
	}
	js.Global().Set("DERO_JS_Relay_TX", js.FuncOf(js_Relay_TX))

	js_Register_2 := func( args []js.Value)  {
		if Local_wallet_instance == nil {
				return
		}
		var reg_tx *transaction.Transaction
			successful_regs := make(chan *transaction.Transaction)
			counter := 0
			fmt.Printf("Beginning wallet registeration")
			for i := 0; i < runtime.GOMAXPROCS(0); i++ {
				go func() {
					for counter == 0 {
						lreg_tx := Local_wallet_instance.GetRegistrationTX()
						hash := lreg_tx.GetHash()
						if hash[0] == 0 && hash[1] == 0 && hash[2] == 0 {
							successful_regs <- lreg_tx
							counter++
							break
						}
					}
				}()
			}

			reg_tx = <-successful_regs
			fmt.Printf("Registration TXID %s\n", reg_tx.GetHash())
			err := Local_wallet_instance.SendTransaction(reg_tx)
			_ = err
	}

	js_Register := func(this js.Value,params []js.Value) interface{} {
		go js_Register_2(params)
		return nil
	}
	js.Global().Set("DERO_JS_Register", js.FuncOf(js_Register))

	js_Close_Encrypted_Wallet :=func(this js.Value, args []js.Value) interface{} {
		if Local_wallet_instance != nil {
			Local_wallet_instance.Close_Encrypted_Wallet()
			Local_wallet_instance = nil

			fmt.Printf("Wallet has been closed\n")
		}
        return nil
	}

	js.Global().Set("DERO_JS_Close_Encrypted_Wallet", js.FuncOf(js_Close_Encrypted_Wallet))

	// these function does NOT report back anything
	js_Rescan_Blockchain := func(this js.Value, args []js.Value) interface{} {
		if Local_wallet_instance != nil {
			Local_wallet_instance.Clean()               // clean existing data from wallet
		}
    return nil
	}
	js.Global().Set("DERO_JS_Rescan_Blockchain", js.FuncOf(js_Rescan_Blockchain))

	// this function does NOT report back anything
	js_SetOnline := func(this js.Value, args []js.Value) interface{} {
		if Local_wallet_instance != nil {
			Local_wallet_instance.SetOnlineMode()
		}
        return nil
	}
	js.Global().Set("DERO_JS_SetOnline", js.FuncOf(js_SetOnline))

	// this function does NOT report back anything
	js_SetOffline := func(this js.Value, args []js.Value) interface{} {
		if Local_wallet_instance != nil {
			Local_wallet_instance.SetOfflineMode()
		}
    return nil
	}
	js.Global().Set("DERO_JS_SetOffline", js.FuncOf(js_SetOffline))

	// this function does NOT report back anything
	js_ChangePassword := func(this js.Value, args []js.Value) interface{} {
		if Local_wallet_instance != nil {
			Local_wallet_instance.Set_Encrypted_Wallet_Password(args[0].String())
		}
        return nil
	}
	js.Global().Set("DERO_JS_ChangePassword", js.FuncOf(js_ChangePassword))

	// this function does NOT report back anything
	js_SetInitialHeight := func(this js.Value, args []js.Value) interface{} {
		return nil
	}
	js.Global().Set("DERO_JS_SetInitialHeight", js.FuncOf(js_SetInitialHeight))

	// this function does NOT report back anything
	js_SetMixin := func(this js.Value, args []js.Value) interface{} {
		if Local_wallet_instance != nil {
			Local_wallet_instance.SetRingSize((args[0].Int()))
		}
        return nil
	}
	js.Global().Set("DERO_JS_SetMixin", js.FuncOf(js_SetMixin))

	// this function does NOT report back anything
	js_SetFeeMultiplier := func(this js.Value, args []js.Value) interface{} {
		if Local_wallet_instance != nil {
			Local_wallet_instance.SetFeeMultiplier(float32(args[0].Float()))
		}
        return nil
	}
	js.Global().Set("DERO_JS_SetFeeMultiplier", js.FuncOf(js_SetFeeMultiplier))
        
        
        // this function does NOT report back anything
	js_SetSyncTime := func(this js.Value, args []js.Value) interface{} {
		if Local_wallet_instance != nil {
			//Local_wallet_instance.SetDelaySync(int64(args[0].Int()))
		}
        return nil
	}
	js.Global().Set("DERO_JS_SetSyncTime", js.FuncOf(js_SetSyncTime))

	// this function does NOT report back anything
	js_SetDaemonAddress := func(this js.Value, args []js.Value) interface{} {
		if Local_wallet_instance != nil {
			Local_wallet_instance.SetDaemonAddress(args[0].String())
		}
        return nil
	}
	js.Global().Set("DERO_JS_SetDaemonAddress", js.FuncOf(js_SetDaemonAddress))


	// some apis to detect  parse validate address
	// this will setup some fields
	js_VerifyAddress := func(this js.Value, args []js.Value) interface{} {

		var address_main, address_paymentid, address_error string
		var address_valid, address_integrated bool

		address_error = "error"
		addr, err := globals.ParseValidateAddress(args[0].String())
		if err == nil {
			address_valid = true
			if addr.IsIntegratedAddress() {
				address_integrated = true
				//address_paymentid = fmt.Sprintf("%x", addr.PaymentID)
			} else {
				address_integrated = false
			}
			address_error = "success"
		} else {
			address_error = err.Error()
			address_valid = false
			address_integrated = false
		}

		js.Global().Set("address_error", address_error)
		js.Global().Set("address_main", address_main)
		js.Global().Set("address_paymentid", address_paymentid)
		js.Global().Set("address_valid", address_valid)
		js.Global().Set("address_integrated", address_integrated)

        return nil
	}

	js.Global().Set("DERO_JS_VerifyAddress", js.FuncOf(js_VerifyAddress))

	js_VerifyAmount := func(this js.Value, args []js.Value) interface{}{
		var amount_valid bool
		lamountstr := strings.TrimSpace(args[0].String())
		_, err := globals.ParseAmount(lamountstr)

		if err != nil {
			js.Global().Set("amount_valid", amount_valid)
			js.Global().Set("amount_error", err.Error())
			return nil
		}
		amount_valid = true
		js.Global().Set("amount_valid", amount_valid)
		js.Global().Set("amount_error", "success")
        return nil
	}
	js.Global().Set("DERO_JS_VerifyAmount", js.FuncOf(js_VerifyAmount))

	js_VerifyPassword := func(this js.Value, args []js.Value) interface{} {
		password_error := "error"
		if Local_wallet_instance != nil {
			valid := Local_wallet_instance.Check_Password(args[0].String())
			if valid {
				password_error = "success"
			}
		}
		js.Global().Set("password_error", password_error)
        return nil
	}
	js.Global().Set("DERO_JS_VerifyPassword", js.FuncOf(js_VerifyPassword))


	js_GetEncryptedCopy := func(this js.Value, args []js.Value) interface{} {
		wallet_encrypted_error := "success"
		var encrypted_bytes []byte
		if Local_wallet_instance != nil {
			encrypted_bytes = Local_wallet_instance.Get_Encrypted_Wallet()
		}

        a := js.Global().Get("Uint8Array").New(len(encrypted_bytes))
		js.CopyBytesToJS(a, encrypted_bytes)
        runtime.KeepAlive(a)
		js.Global().Set("wallet_encrypted_dump", js.Global().Get("Int8Array").New(a.Get("buffer")))
		js.Global().Set("wallet_encrypted_error", wallet_encrypted_error)
        return nil
	}

	js.Global().Set("DERO_JS_GetEncryptedCopy", js.FuncOf(js_GetEncryptedCopy))


}

// if this remain empty, default 127.0.0.1:20206 is used
var daemon_address = "" // this is setup below at runtime

// this wasm module exports necessary wallet apis to javascript
func main() {

	fmt.Printf("running go now\n")
	globals.Arguments = map[string]interface{}{}

	globals.InitializeLog(os.Stdout,io.Discard)
	logger = globals.Logger


	globals.Config = config.Mainnet
	//globals.Initialize()

	debug.SetGCPercent(40) // start GC at 40%

	href := js.Global().Get("location").Get("href")
	u, err := url.Parse(href.String())
	if err == nil {
		r := strings.NewReplacer("0", "",
			"1", "",
			"2", "",
			"3", "",
			"4", "",
			"5", "",
			"6", "",
			"7", "",
			"8", "",
			"9", "",
			".", "",
			":", "",
		)
		fmt.Printf("u %+v\n", u)
		fmt.Printf("scheme %+v\n", u.Scheme)
		fmt.Printf("Host %+v\n", u.Host)
		if u.Scheme == "http" || u.Scheme == "" { // we do not support DNS names for http, for security reasons
			if len(r.Replace(u.Host)) == 0 { // number is an ipadress
				if strings.Contains(u.Host, ":") {
					daemon_address = u.Host // set the daemon address
				} else {
					daemon_address = u.Host + ":80" // set the daemon address
				}
			}
		} else if u.Scheme == "https" {
			if strings.Contains(u.Host, ":") {
				daemon_address = u.Scheme + "://" + u.Host // set the daemon address
			} else {
				daemon_address = u.Scheme + "://" + u.Host + ":443" // set the daemon address
			}
		}

		if len(daemon_address) == 0 {
			if globals.IsMainnet() {
				daemon_address = "127.0.0.1:10102"
			} else {
				daemon_address = "127.0.0.1:40403"
			}
		}

	}else{
		fmt.Printf("error parsing url",err)
	}

	walletapi.SetDaemonAddress(daemon_address) // set daemon address
	fmt.Printf("daemon_address %s\n",daemon_address)

	fmt.Printf("registering callbacks")
	register_wallet_callbacks()
	fmt.Printf("registered callbacks")

    go walletapi.Keep_Connectivity() // maintain connectivity
    // init the lookup table one, anyone importing walletapi should init this first
	//go walletapi.Initialize_LookupTable(1, 1<<18)
	go update_balance()

	select {} // if this return, program will exit
}

func update_balance() {

	wallet_version_string := config.Version.String()
	for {
		unlocked_balance := uint64(0)
		locked_balance := uint64(0)
		total_balance := uint64(0)

		wallet_height := uint64(0)
		daemon_height := uint64(0)
                wallet_topo_height := uint64(0)
                daemon_topo_height := uint64(0)

		wallet_initial_height := int64(0)

		wallet_address := ""

		wallet_available := false
		wallet_complete := true
		wallet_online := false
		wallet_mixin := 16
		wallet_fees_multiplier := float64(1.5)
		wallet_daemon_address := ""
                wallet_sync_time := int64(0)
                wallet_minimum_topo_height := int64(-1)
        wallet_error := ""

		if Local_wallet_instance != nil {
			unlocked_balance, locked_balance = Local_wallet_instance.Get_Balance()


			total_balance = unlocked_balance + locked_balance

			wallet_height = Local_wallet_instance.Get_Height()
			daemon_height = Local_wallet_instance.Get_Daemon_Height()
            wallet_topo_height = uint64(Local_wallet_instance.Get_TopoHeight())
            daemon_topo_height = uint64(Local_wallet_instance.Get_Daemon_TopoHeight())

			wallet_address = Local_wallet_instance.GetAddress().String()
			wallet_available = true

			//wallet_initial_height = Local_wallet_instance.GetInitialHeight()

			wallet_online = Local_wallet_instance.GetMode()

			wallet_mixin = Local_wallet_instance.GetRingSize()

			wallet_fees_multiplier = float64(Local_wallet_instance.GetFeeMultiplier())
			wallet_daemon_address = Local_wallet_instance.Daemon_Endpoint
			if Local_wallet_instance.Error != nil {
				wallet_error = Local_wallet_instance.Error.Error()
			}
			
			
			//wallet_sync_time = Local_wallet_instance.SetDelaySync(0)
              //          wallet_minimum_topo_height = Local_wallet_instance.GetMinimumTopoHeight()


		}
		js.Global().Set("wallet_address", wallet_address)
		js.Global().Set("total_balance", globals.FormatMoney(total_balance))
		js.Global().Set("locked_balance", globals.FormatMoney(locked_balance))
		js.Global().Set("unlocked_balance", globals.FormatMoney(unlocked_balance))
		js.Global().Set("wallet_height", wallet_height)
		js.Global().Set("daemon_height", daemon_height)

		js.Global().Set("wallet_topo_height", wallet_topo_height)
		js.Global().Set("daemon_topo_height", daemon_topo_height)
		js.Global().Set("wallet_available", wallet_available)
		js.Global().Set("wallet_complete", wallet_complete)
		js.Global().Set("wallet_initial_height", wallet_initial_height)

		js.Global().Set("wallet_online", wallet_online)
		js.Global().Set("wallet_mixin", wallet_mixin)
		js.Global().Set("wallet_fees_multiplier", wallet_fees_multiplier)
		js.Global().Set("wallet_daemon_address", wallet_daemon_address)
		js.Global().Set("wallet_version_string", wallet_version_string)
        js.Global().Set("wallet_sync_time", wallet_sync_time)
        js.Global().Set("wallet_minimum_topo_height", wallet_minimum_topo_height)
        js.Global().Set("wallet_error", wallet_error)
                
              
                

		time.Sleep(100 * time.Millisecond) // update max 10 times per second

	}
}


var i8_address, i8_address_paymentid string

// generate integrated address at user demand
func generate_integrated_address() {
	if Local_wallet_instance != nil {
		i8 := Local_wallet_instance.GetRandomIAddress8()
		js.Global().Set("random_i8_address", i8.String())
		//js.Global().Set("random_i8_address_paymentid", fmt.Sprintf("%x", i8.PaymentID))
	}
}
