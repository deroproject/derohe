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

import "time"
import "fmt"

//import "io/ioutil"
import "strings"

//import "path/filepath"
//import "encoding/hex"

import "github.com/chzyer/readline"

import "github.com/deroproject/derohe/rpc"
import "github.com/deroproject/derohe/globals"

//import "github.com/deroproject/derohe/address"

//import "github.com/deroproject/derohe/walletapi"
import "github.com/deroproject/derohe/transaction"

// handle menu if a wallet is currently opened
func display_easymenu_post_open_command(l *readline.Instance) {
	w := l.Stderr()
	io.WriteString(w, "Menu:\n")

	io.WriteString(w, "\t\033[1m1\033[0m\tDisplay account Address \n")
	io.WriteString(w, "\t\033[1m2\033[0m\tDisplay Seed "+color_red+"(Please save seed in safe location)\n\033[0m")

	io.WriteString(w, "\t\033[1m3\033[0m\tDisplay Keys (hex)\n")

	if !wallet.IsRegistered() {
		io.WriteString(w, "\t\033[1m4\033[0m\tAccount registration to blockchain (registration has no fee requirement and is precondition to use the account)\n")
		io.WriteString(w, "\n")
		io.WriteString(w, "\n")
	} else { // hide some commands, if view only wallet
		io.WriteString(w, "\t\033[1m4\033[0m\tDisplay wallet pool\n")
		io.WriteString(w, "\t\033[1m5\033[0m\tTransfer (send  DERO) To Another Wallet\n")
		//io.WriteString(w, "\t\033[1m6\033[0m\tCreate Transaction in offline mode\n")
		io.WriteString(w, "\n")
	}

	io.WriteString(w, "\t\033[1m7\033[0m\tChange wallet password\n")
	io.WriteString(w, "\t\033[1m8\033[0m\tClose Wallet\n")
	if wallet.IsRegistered() {
		io.WriteString(w, "\t\033[1m12\033[0m\tTransfer all balance (send  DERO) To Another Wallet\n")
		io.WriteString(w, "\t\033[1m13\033[0m\tShow transaction history\n")
		io.WriteString(w, "\t\033[1m14\033[0m\tRescan transaction history\n")
	}

	io.WriteString(w, "\n\t\033[1m9\033[0m\tExit menu and start prompt\n")
	io.WriteString(w, "\t\033[1m0\033[0m\tExit Wallet\n")

}

// this handles all the commands if wallet in menu mode  and a wallet is opened
func handle_easymenu_post_open_command(l *readline.Instance, line string) (processed bool) {

	var err error
	_ = err
	line = strings.TrimSpace(line)
	line_parts := strings.Fields(line)
	processed = true

	if len(line_parts) < 1 { // if no command return
		return
	}

	command := ""
	if len(line_parts) >= 1 {
		command = strings.ToLower(line_parts[0])
	}

	offline_tx := false
	_ = offline_tx
	switch command {
	case "1":
		fmt.Fprintf(l.Stderr(), "Wallet address : "+color_green+"%s"+color_white+"\n", wallet.GetAddress())

		if !wallet.IsRegistered() {
			reg_tx := wallet.GetRegistrationTX()
			fmt.Fprintf(l.Stderr(), "Registration TX : "+color_green+"%x"+color_white+"\n", reg_tx.Serialize())
		}
		PressAnyKey(l, wallet)

	case "2": // give user his seed
		if !ValidateCurrentPassword(l, wallet) {
			globals.Logger.Warnf("Invalid password")
			PressAnyKey(l, wallet)
			break
		}
		display_seed(l, wallet) // seed should be given only to authenticated users
		PressAnyKey(l, wallet)

	case "3": // give user his keys in hex form

		if !ValidateCurrentPassword(l, wallet) {
			globals.Logger.Warnf("Invalid password")
			PressAnyKey(l, wallet)
			break
		}

		display_spend_key(l, wallet)
		PressAnyKey(l, wallet)

	case "4": // Registration

		if !wallet.IsRegistered() {

			fmt.Fprintf(l.Stderr(), "Wallet address : "+color_green+"%s"+color_white+" is going to be registered.This is a pre-condition for using the online chain.It will take few seconds to register.\n", wallet.GetAddress())

			reg_tx := wallet.GetRegistrationTX()

			// at this point we must send the registration transaction

			fmt.Fprintf(l.Stderr(), "Wallet address : "+color_green+"%s"+color_white+" is going to be registered.Pls wait till the account is registered.\n", wallet.GetAddress())

			fmt.Printf("sending registration tx err %s\n", wallet.SendTransaction(reg_tx))
		} else {
			pool := wallet.GetPool()
			fmt.Fprintf(l.Stderr(), "Wallet pool has %d pending/in-progress transactions.\n", len(pool))
			fmt.Fprintf(l.Stderr(), "%5s %9s %8s %64s %s  %s\n", "No.", "Amount", "TH", "TXID", "Destination", "Status")
			for i := range pool {
				var txid, status string
				if len(pool[i].Tries) > 0 {
					try := pool[i].Tries[len(pool[i].Tries)-1]
					txid = try.TXID.String()
					status = try.Status
				} else {
					status = "Will Dispatch in next block"
				}
				fmt.Fprintf(l.Stderr(), "%5d %9s %8d  %64s %s %s\n", i, "-"+globals.FormatMoney(pool[i].Amount()), pool[i].Trigger_Height, txid, "Not implemented", status)
			}

		}

	case "6":
		offline_tx = true
		fallthrough
	case "5":
		if !valid_registration_or_display_error(l, wallet) {
			break
		}
		if !ValidateCurrentPassword(l, wallet) {
			globals.Logger.Warnf("Invalid password")
			break
		}

		// a , amount_to_transfer, err := collect_transfer_info(l,wallet)
		a, err := ReadAddress(l)
		if err != nil {
			globals.Logger.Warnf("Err :%s", err)
			break
		}

		var amount_to_transfer uint64

		var arguments = rpc.Arguments{
			// { rpc.RPC_DESTINATION_PORT, rpc.DataUint64,uint64(0x1234567812345678)},
			// { rpc.RPC_VALUE_TRANSFER, rpc.DataUint64,uint64(12345)},
			// { rpc.RPC_EXPIRY , rpc.DataTime, time.Now().Add(time.Hour).UTC()},
			// { rpc.RPC_COMMENT , rpc.DataString, "Purchase XYZ"},
		}
		if a.IsIntegratedAddress() { // read everything from the address

			if a.Arguments.Validate_Arguments() != nil {
				globals.Logger.Warnf("Integrated Address  arguments could not be validated, err: %s", err)
				break
			}

			if !a.Arguments.Has(rpc.RPC_DESTINATION_PORT, rpc.DataUint64) { // but only it is present
				globals.Logger.Warnf("Integrated Address does not contain destination port.")
				break
			}

			arguments = append(arguments, rpc.Argument{rpc.RPC_DESTINATION_PORT, rpc.DataUint64, a.Arguments.Value(rpc.RPC_DESTINATION_PORT, rpc.DataUint64).(uint64)})
			// arguments = append(arguments, rpc.Argument{"Comment", rpc.DataString, "holygrail of all data is now working if you can see this"})

			if a.Arguments.Has(rpc.RPC_EXPIRY, rpc.DataTime) { // but only it is present

				if a.Arguments.Value(rpc.RPC_EXPIRY, rpc.DataTime).(time.Time).Before(time.Now().UTC()) {
					globals.Logger.Warnf("This address has expired on %s", a.Arguments.Value(rpc.RPC_EXPIRY, rpc.DataTime))
					break
				} else {
					globals.Logger.Infof("This address will expire on %s", a.Arguments.Value(rpc.RPC_EXPIRY, rpc.DataTime))
				}
			}

			globals.Logger.Infof("Destination port is integreted in address ID:%016x", a.Arguments.Value(rpc.RPC_DESTINATION_PORT, rpc.DataUint64).(uint64))

			if a.Arguments.Has(rpc.RPC_COMMENT, rpc.DataString) { // but only it is present
				globals.Logger.Infof("Integrated Message:%s", a.Arguments.Value(rpc.RPC_COMMENT, rpc.DataString))
			}
		}

		// arguments have been already validated
		for _, arg := range a.Arguments {
			if !(arg.Name == rpc.RPC_COMMENT || arg.Name == rpc.RPC_EXPIRY || arg.Name == rpc.RPC_DESTINATION_PORT || arg.Name == rpc.RPC_SOURCE_PORT || arg.Name == rpc.RPC_VALUE_TRANSFER) {
				switch arg.DataType {
				case rpc.DataString:
					if v, err := ReadString(l, arg.Name, arg.Value.(string)); err == nil {
						arguments = append(arguments, rpc.Argument{arg.Name, arg.DataType, v})
					} else {
						globals.Logger.Warnf("%s could not be parsed (type %s),", arg.Name, arg.DataType)
						return
					}
				case rpc.DataInt64:
					if v, err := ReadInt64(l, arg.Name, arg.Value.(int64)); err == nil {
						arguments = append(arguments, rpc.Argument{arg.Name, arg.DataType, v})
					} else {
						globals.Logger.Warnf("%s could not be parsed (type %s),", arg.Name, arg.DataType)
						return
					}
				case rpc.DataUint64:
					if v, err := ReadUint64(l, arg.Name, arg.Value.(uint64)); err == nil {
						arguments = append(arguments, rpc.Argument{arg.Name, arg.DataType, v})
					} else {
						globals.Logger.Warnf("%s could not be parsed (type %s),", arg.Name, arg.DataType)
						return
					}
				case rpc.DataFloat64:
					if v, err := ReadFloat64(l, arg.Name, arg.Value.(float64)); err == nil {
						arguments = append(arguments, rpc.Argument{arg.Name, arg.DataType, v})
					} else {
						globals.Logger.Warnf("%s could not be parsed (type %s),", arg.Name, arg.DataType)
						return
					}
				case rpc.DataTime:
					globals.Logger.Warnf("time argument is currently not supported.")
					break

				}
			}
		}

		if a.Arguments.Has(rpc.RPC_VALUE_TRANSFER, rpc.DataUint64) { // but only it is present
			globals.Logger.Infof("Transaction Value: %s", globals.FormatMoney(a.Arguments.Value(rpc.RPC_VALUE_TRANSFER, rpc.DataUint64).(uint64)))
			amount_to_transfer = a.Arguments.Value(rpc.RPC_VALUE_TRANSFER, rpc.DataUint64).(uint64)
		} else {

			amount_str := read_line_with_prompt(l, fmt.Sprintf("Enter amount to transfer in DERO (max TODO): "))

			if amount_str == "" {
				amount_str = ".00009"
			}
			amount_to_transfer, err = globals.ParseAmount(amount_str)
			if err != nil {
				globals.Logger.Warnf("Err :%s", err)
				break // invalid amount provided, bail out
			}
		}

		// if no arguments, use space by embedding a small comment
		if len(arguments) == 0 { // allow user to enter Comment
			if v, err := ReadString(l, "Comment", ""); err == nil {
				arguments = append(arguments, rpc.Argument{"Comment", rpc.DataString, v})
			} else {
				globals.Logger.Warnf("%s could not be parsed (type %s),", "Comment", rpc.DataString)
				return
			}
		}

		if _, err := arguments.CheckPack(transaction.PAYLOAD0_LIMIT); err != nil {
			globals.Logger.Warnf("Arguments packing err: %s,", err)
			return
		}

		if ConfirmYesNoDefaultNo(l, "Confirm Transaction (y/N)") {

			//src_port := uint64(0xffffffffffffffff)

			_, err := wallet.PoolTransfer([]rpc.Transfer{rpc.Transfer{Amount: amount_to_transfer, Destination: a.String(), Payload_RPC: arguments}}, rpc.Arguments{}) // empty SCDATA

			if err != nil {
				globals.Logger.Warnf("Error while building Transaction err %s\n", err)
				break
			}
			//fmt.Printf("queued tx err %s\n")
		}

	case "12":
		if !valid_registration_or_display_error(l, wallet) {
			break
		}
		if !ValidateCurrentPassword(l, wallet) {
			globals.Logger.Warnf("Invalid password")
			break
		}

		globals.Logger.Warnf("Not supported err %s\n", err)

		/*
			// a , amount_to_transfer, err := collect_transfer_info(l,wallet)
			fmt.Printf("dest address %s\n", "deroi1qxqqkmaz8nhv4q07w3cjyt84kmrqnuw4nprpqfl9xmmvtvwa7cdykxq5dph4ufnx5ndq4ltraf  (14686f5e2666a4da)  dero1qxqqkmaz8nhv4q07w3cjyt84kmrqnuw4nprpqfl9xmmvtvwa7cdykxqpfpaes")
			a, err := ReadAddress(l)
			if err != nil {
				globals.Logger.Warnf("Err :%s", err)
				break
			}
			// if user provided an integrated address donot ask him payment id
			if a.IsIntegratedAddress() {
				globals.Logger.Infof("Payment ID is integreted in address ID:%x", a.PaymentID)
			}

			if ConfirmYesNoDefaultNo(l, "Confirm Transaction to send entire balance (y/N)") {

				addr_list := []address.Address{*a}
				amount_list := []uint64{0} // transfer 50 dero, 2 dero
				fees_per_kb := uint64(0)   // fees  must be calculated by walletapi
				uid, err := wallet.PoolTransfer(addr_list, amount_list, fees_per_kb, 0, true)
				_ = uid
				if err != nil {
					globals.Logger.Warnf("Error while building Transaction err %s\n", err)
					break
				}
			}
		*/

		//PressAnyKey(l, wallet) // wait for a key press

	case "7": // change password
		if ConfirmYesNoDefaultNo(l, "Change wallet password (y/N)") &&
			ValidateCurrentPassword(l, wallet) {

			new_password := ReadConfirmedPassword(l, "Enter new password", "Confirm password")
			err = wallet.Set_Encrypted_Wallet_Password(new_password)
			if err == nil {
				globals.Logger.Infof("Wallet password successfully changed")
			} else {
				globals.Logger.Warnf("Wallet password could not be changed err %s", err)
			}
		}

	case "8": // close and discard user key

		wallet.Close_Encrypted_Wallet()
		prompt_mutex.Lock()
		wallet = nil // overwrite previous instance
		prompt_mutex.Unlock()

		fmt.Fprintf(l.Stderr(), color_yellow+"Wallet closed"+color_white)

	case "9": // enable prompt mode
		menu_mode = false
		globals.Logger.Infof("Prompt mode enabled, type \"menu\" command to start menu mode")

	case "0", "bye", "exit", "quit":
		wallet.Close_Encrypted_Wallet() // save the wallet
		prompt_mutex.Lock()
		wallet = nil
		globals.Exit_In_Progress = true
		prompt_mutex.Unlock()
		fmt.Fprintf(l.Stderr(), color_yellow+"Wallet closed"+color_white)
		fmt.Fprintf(l.Stderr(), color_yellow+"Exiting"+color_white)

	case "13":
		show_transfers(l, wallet, 100)

	case "14":
		globals.Logger.Infof("Rescanning wallet history")
		rescan_bc(wallet)

	default:
		processed = false // just loop

	}
	return
}
