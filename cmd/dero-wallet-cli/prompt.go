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
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/chzyer/readline"
	"github.com/deroproject/derohe/config"
	"github.com/deroproject/derohe/cryptography/crypto"
	"github.com/deroproject/derohe/globals"
	"github.com/deroproject/derohe/rpc"
	"github.com/deroproject/derohe/walletapi"
)

//import "io/ioutil"
//import "path/filepath"

var account walletapi.Account

func isASCII(s string) bool {
	for _, c := range s {
		if c > unicode.MaxASCII {
			return false
		}
	}
	return true
}

// handle all commands while  in prompt mode
func handle_prompt_command(l *readline.Instance, line string) {

	var err error
	line = strings.TrimSpace(line)
	line_parts := strings.Fields(line)

	if len(line_parts) < 1 { // if no command return
		return
	}

	_ = err
	command := ""
	if len(line_parts) >= 1 {
		command = strings.ToLower(line_parts[0])
	}

	// handled closed wallet commands
	switch command {
	case "address", "rescan_bc", "seed", "set", "password", "get_tx_key", "i8", "payment_id":
		fallthrough
	case "spendkey", "transfer", "close":
		fallthrough
	case "transfer_all", "sweep_all", "show_transfers", "balance", "status":
		if wallet == nil {
			logger.Error(err, "No wallet available")
			return
		}
	}

	switch command {
	case "help":
		usage(l.Stderr())
	case "address": // give user his account address

		fmt.Fprintf(l.Stderr(), "Wallet address : "+color_green+"%s"+color_white+"\n", wallet.GetAddress())
	case "status": // show syncronisation status
		fmt.Fprintf(l.Stderr(), "Wallet Version : %s\n", config.Version.String())
		fmt.Fprintf(l.Stderr(), "Wallet Height : %d\t Daemon Height %d \n", wallet.Get_Height(), wallet.Get_Daemon_Height())
		fallthrough
	case "balance": // give user his balance
		balance_unlocked, locked_balance := wallet.Get_Balance_Rescan()
		fmt.Fprintf(l.Stderr(), "DERO Balance: "+color_green+"%s"+color_white+"\n", globals.FormatMoney(locked_balance+balance_unlocked))

		line_parts := line_parts[1:] // remove first part

		switch len(line_parts) {
		case 0:
			addr := wallet.GetAddress().String()
			for scid := range wallet.GetAccount().EntriesNative {
				if !scid.IsZero() {
					balance, _, err := wallet.GetDecryptedBalanceAtTopoHeight(scid, -1, addr)
					if err != nil {
						logger.Error(err, "error during Sc balance", "scid", scid.String())
					} else {
						// TODO digits token standard
						fmt.Fprintf(l.Stderr(), "SCID %s Balance: "+color_green+"%d"+color_white+"\n\n", scid, balance)
					}
				}
			}
			break

		case 1: // scid balance
			scid := crypto.HashHexToHash(line_parts[0])

			//logger.Info("scid1 %s  line_parts %+v", scid, line_parts)
			balance, _, err := wallet.GetDecryptedBalanceAtTopoHeight(scid, -1, wallet.GetAddress().String())

			//logger.Info("scid %s", scid)
			if err != nil {
				logger.Error(err, "error during Sc balance", "scid", scid.String())
			} else {
				fmt.Fprintf(l.Stderr(), "SCID %s Balance: "+color_green+"%s"+color_white+"\n\n", line_parts[0], globals.FormatMoney(balance))
			}

		case 2: // scid balance at topoheight
			logger.Error(err, "not implemented")
			break
		}

	case "token_add":
		line_parts := line_parts[1:] // remove first part

		switch len(line_parts) {
		case 0:
			break
		case 1:
			scid := crypto.HashHexToHash(line_parts[0])
			if err := wallet.TokenAdd(scid); err != nil {
				logger.Error(err, "Token")
			} else {
				wallet.Save_Wallet()
				fmt.Fprintf(l.Stderr(), "SCID "+color_green+"%s"+color_white+" added\n\n", scid.String())
			}
		default:
			logger.Error(err, "not implemented")
		}

	case "rescan_bc", "rescan_spent": // rescan from 0
		if offline_mode {
			logger.Error(err, "Offline wallet rescanning NOT implemented")
		} else {
			rescan_bc(wallet)
		}

	case "seed": // give user his seed, if password is valid
		if !ValidateCurrentPassword(l, wallet) {
			logger.Error(err, "Invalid password")
			PressAnyKey(l, wallet)
			break
		}
		display_seed(l, wallet) // seed should be given only to authenticated users

	case "spendkey": // give user his spend key
		display_spend_key(l, wallet)

	case "filesign": // sign a file contents
		if !ValidateCurrentPassword(l, wallet) {
			logger.Error(err, "Invalid password")
			PressAnyKey(l, wallet)
			break
		}

		filename, err := ReadString(l, "Enter file to sign", "")
		if err != nil {
			logger.Error(err, "Cannot read input file name")
			break
		}

		outputfile := filename + ".sign"

		if filedata, err := os.ReadFile(filename); err != nil {
			logger.Error(err, "Cannot read input file")
		} else if err := os.WriteFile(outputfile, wallet.SignData(filedata), 0600); err != nil {
			logger.Error(err, "Cannot write output file", "file", outputfile)
		} else {
			logger.Info("successfully signed file. please check", "file", outputfile)
		}

	case "fileverify": // verify a file contents
		filename, err := ReadString(l, "Enter file to verify signature", "")
		if err != nil {
			logger.Error(err, "Cannot read input file name")
			break
		}

		outputfile := strings.TrimSuffix(filename, ".sign")

		if filedata, err := os.ReadFile(filename); err != nil {
			logger.Error(err, "Cannot read input file")
		} else if signer, message, err := wallet.CheckSignature(filedata); err != nil {
			logger.Error(err, "Signature verify failed", "file", filename)
		} else {
			logger.Info("Signed by", "address", signer.String())

			if isASCII(string(message)) { // do not spew garbage
				logger.Info("", "message", string(message))
			}

			if os.WriteFile(outputfile, message, 0600); err != nil {
				logger.Error(err, "Cannot write output file", "file", outputfile)
			}
			logger.Info("successfully wrote message to file. please check", "file", outputfile)

		}

	case "filesign_huge": // sign a hugefile contents
		if !ValidateCurrentPassword(l, wallet) {
			logger.Error(err, "Invalid password")
			PressAnyKey(l, wallet)
			break
		}

		filename, err := ReadString(l, "Enter file to sign", "")
		if err != nil {
			logger.Error(err, "Cannot read input file name")
			break
		}

		if err := wallet.SignFile(filename); err != nil {
			logger.Error(err, "Cannot sign", "file", filename)
		} else {
			logger.Info("successfully signed file. please check", "file", filename, "signature", filename+".signed")
		}

	case "fileverify_huge": // verify a file contents
		filename, err := ReadString(l, "Enter file to verify signature", "")
		if err != nil {
			logger.Error(err, "Cannot read input file name")
			break
		}

		if signer, err := wallet.CheckFileSignature(filename); err != nil {
			logger.Error(err, "Signature verify failed", "file", filename)
		} else {
			logger.Info("Signed by", "address", signer.String())

			logger.Info("Signature verified successfully.", "file", filename)
		}

	case "password": // change wallet password
		if ConfirmYesNoDefaultNo(l, "Change wallet password (y/N)") &&
			ValidateCurrentPassword(l, wallet) {

			new_password := ReadConfirmedPassword(l, "Enter new password", "Confirm password")
			err = wallet.Set_Encrypted_Wallet_Password(new_password)
			if err == nil {
				logger.Info("Wallet password successfully changed")
			} else {
				logger.Error(err, "Wallet password could not be changed")
			}
		}

	case "get_tx_key":
		if !valid_registration_or_display_error(l, wallet) {
			break
		}
		if len(line_parts) == 2 && len(line_parts[1]) == 64 {
			_, err := hex.DecodeString(line_parts[1])
			if err != nil {
				logger.Error(err, "Error parsing txhash")
				break
			}
			key := wallet.GetTXKey(line_parts[1])
			if key != "" {
				logger.Info("TX Proof key \"%s\"", key)
			} else {
				logger.Error(err, "TX not found in database")
			}
		} else {
			logger.Info("get_tx_key needs transaction hash as input parameter")
			logger.Info("eg. get_tx_key ea551b02b9f1e8aebe4d7b1b7f6bf173d76ae614cb9a066800773fee9e226fd7")
		}
	case "sweep_all", "transfer_all": // transfer everything
		//Transfer_Everything(l)

	case "show_transfers":

		switch len(line_parts) {
		case 1:
			var zeroscid crypto.Hash
			show_transfers(l, wallet, zeroscid, 100)
			break

		case 2: // scid balance
			scid := crypto.HashHexToHash(line_parts[1])
			show_transfers(l, wallet, scid, 100)

		default:
			logger.Error(err, "unknown parameters or not implemented")
			break
		}

	case "set": // set/display different settings
		handle_set_command(l, line)
	case "close": // close the account
		if !ValidateCurrentPassword(l, wallet) {
			logger.Error(err, "Invalid password")
			break
		}
		wallet.Close_Encrypted_Wallet() // overwrite previous instance

	case "menu": // enable menu mode
		menu_mode = true
		logger.Info("Menu mode enabled")
	case "i8", "integrated_address": // user wants a random integrated address 8 bytes
		a := wallet.GetRandomIAddress8()
		if ConfirmYesNoDefaultNo(l, "Do you want to set a specific SCID ? (y/N)") {
			scid, err := ReadSCID(l)
			if err != nil {
				logger.Error(err, "Error reading SCID")
				break
			}
			a.Arguments = append(a.Arguments, rpc.Argument{Name: rpc.RPC_ASSET, DataType: rpc.DataHash, Value: scid})
		}

		fmt.Fprintf(l.Stderr(), "Wallet integrated address : "+color_green+"%s"+color_white+"\n", a.String())
		fmt.Fprintf(l.Stderr(), "Embedded Arguments : "+color_green+"%s"+color_white+"\n", a.Arguments)

	case "version":
		logger.Info("", "Version", config.Version.String())

	case "burn":
		line_parts := line_parts[1:] // remove first part
		if len(line_parts) < 2 {
			logger.Error(err, "burn needs destination address  and amount as input parameter")
			break
		}
		addr := line_parts[0]
		send_amount := uint64(1)
		burn_amount, err := globals.ParseAmount(line_parts[1])
		if err != nil {
			logger.Error(err, "Error Parsing burn amount", "raw", line_parts[1])
			return
		}
		if ConfirmYesNoDefaultNo(l, "Confirm Transaction (y/N)") {

			//uid, err := wallet.PoolTransferWithBurn(addr, send_amount, burn_amount, data, rpc.Arguments{})

			tx, err := wallet.TransferPayload0([]rpc.Transfer{rpc.Transfer{Amount: send_amount, Burn: burn_amount, Destination: addr}}, 0, false, rpc.Arguments{}, 0, false) // empty SCDATA

			if err != nil {
				logger.Error(err, "Error while building Transaction")
				break
			}

			if err = wallet.SendTransaction(tx); err != nil {
				logger.Error(err, "Error while dispatching Transaction")
				return
			}
			logger.Info("Dispatched tx", "txid", tx.GetHash().String())

			//fmt.Printf("queued tx err %s\n", err)
			//build_relay_transaction(l, uid, err, offline_tx, amount_list)
		}
	case "transfer":
		// parse the address, amount pair
		/*
			line_parts := line_parts[1:] // remove first part

			addr_list := []address.Address{}
			amount_list := []uint64{}
			payment_id := ""

			for i := 0; i < len(line_parts); {

				globals.Logger.Debugf("len %d %+v", len(line_parts), line_parts)
				if len(line_parts) >= 2 { // parse address amount pair
					addr, err := globals.ParseValidateAddress(line_parts[0])
					if err != nil {
						logger.Error(err,"Error Parsing \"%s\" err %s", line_parts[0], err)
						return
					}
					amount, err := globals.ParseAmount(line_parts[1])
					if err != nil {
						logger.Error(err,"Error Parsing \"%s\" err %s", line_parts[1], err)
						return
					}
					line_parts = line_parts[2:] // remove parsed

					addr_list = append(addr_list, *addr)
					amount_list = append(amount_list, amount)

					continue
				}
				if len(line_parts) == 1 { // parse payment_id
					if len(line_parts[0]) == 64 || len(line_parts[0]) == 16 {
						_, err := hex.DecodeString(line_parts[0])
						if err != nil {
							logger.Error(err,"Error parsing payment ID, it should be in hex 16 or 64 chars")
							return
						}
						payment_id = line_parts[0]
						line_parts = line_parts[1:]

					} else {
						logger.Error(err,"Invalid payment ID \"%s\"", line_parts[0])
						return
					}

				}
			}

			// check if everything is okay, if yes build the transaction
			if len(addr_list) == 0 {
				logger.Error(err,"Destination address not provided")
				return
			}

			payment_id_integrated := false
			for i := range addr_list {
				if addr_list[i].IsIntegratedAddress() {
					payment_id_integrated = true
					logger.Info("Payment ID is integrated in address ID:%x", addr_list[i].PaymentID)
				}

			}


			offline := false
			tx, inputs, input_sum, change, err := wallet.Transfer(addr_list, amount_list, 0, payment_id, 0, 0)
			build_relay_transaction(l, tx, inputs, input_sum, change, err, offline, amount_list)
		*/

	case "q", "bye", "exit", "quit":
		globals.Exit_In_Progress = true
		if wallet != nil {
			wallet.Close_Encrypted_Wallet() // overwrite previous instance
		}
	case "flush": // flush wallet pool
		logger.Error(err, "No such command")

	case "": // blank enter key just loop
	default:
		//fmt.Fprintf(l.Stderr(), "you said: %s", strconv.Quote(line))
		logger.Error(err, "No such command")
	}

}

// handle all commands while  in prompt mode
func handle_set_command(l *readline.Instance, line string) {

	//var err error
	line = strings.TrimSpace(line)
	line_parts := strings.Fields(line)

	if len(line_parts) < 1 { // if no command return
		return
	}

	command := ""
	if len(line_parts) >= 2 {
		command = strings.ToLower(line_parts[1])
	}

	help := false
	switch command {
	case "help":
	case "ringsize":
		if len(line_parts) != 3 {
			logger.Info("Wrong number of arguments, see help eg", "")
			help = true
			break
		}
		s, err := strconv.ParseUint(line_parts[2], 10, 64)
		if err != nil {
			logger.Error(err, "Error parsing ringsize")
			return
		}
		wallet.SetRingSize(int(s))
		logger.Info("New Ring size", "ringsize", wallet.GetRingSize())

	case "priority":
		if len(line_parts) != 3 {
			logger.Info("Wrong number of arguments, see help eg")
			help = true
			break
		}
		s, err := strconv.ParseFloat(line_parts[2], 64)
		if err != nil {
			logger.Error(err, "Error parsing priority")
			return
		}
		wallet.SetFeeMultiplier(float32(s))
		logger.Info("Transaction", "priority", wallet.GetFeeMultiplier())

	case "seed": // seed only has 1 setting, lanuage so do it now
		language := choose_seed_language(l)
		logger.Info("Setting seed language", "language", wallet.SetSeedLanguage(language))

	default:
		help = true
	}

	if help == true || len(line_parts) == 1 { // user type plain set command, give out all settings and help

		fmt.Fprintf(l.Stderr(), color_extra_white+"Current settings"+color_extra_white+"\n")
		fmt.Fprintf(l.Stderr(), color_normal+"Seed Language: "+color_extra_white+"%s\t"+color_normal+"eg. "+color_extra_white+"set seed language\n"+color_normal, wallet.GetSeedLanguage())
		fmt.Fprintf(l.Stderr(), color_normal+"Ringsize: "+color_extra_white+"%d\t"+color_normal+"eg. "+color_extra_white+"set ringsize 16\n"+color_normal, wallet.GetRingSize())
		fmt.Fprintf(l.Stderr(), color_normal+"Save Every : "+color_extra_white+"%s \t"+color_normal+"eg. "+color_extra_white+"default value:0 (set using command line)\n"+color_normal, wallet.SetSaveDuration(-1))
		fmt.Fprintf(l.Stderr(), color_normal+"Track Recent Blocks : "+color_extra_white+"%d \t"+color_normal+"eg. "+color_extra_white+"default value:0 means track all blocks (set using command line)\n"+color_normal, wallet.SetTrackRecentBlocks(-1))

		fmt.Fprintf(l.Stderr(), color_normal+"Priority: "+color_extra_white+"%0.2f\t"+color_normal+"eg. "+color_extra_white+"set priority 4.0\t"+color_normal+"Transaction priority on DERO network \n", wallet.GetFeeMultiplier())
		fmt.Fprintf(l.Stderr(), "\t\tMinimum priority is 1.00. High priority = high fees\n")

	}
}

// read an address with all goodies such as color encoding and other things in prompt
func ReadAddress(l *readline.Instance, wallet *walletapi.Wallet_Disk) (a *rpc.Address, err error) {
	setPasswordCfg := l.GenPasswordConfig()
	setPasswordCfg.EnableMask = false

	prompt_mutex.Lock()
	defer prompt_mutex.Unlock()

	var linestr string

	setPasswordCfg.SetListener(func(line []rune, pos int, key rune) (newLine []rune, newPos int, ok bool) {
		error_message := ""
		color := color_green

		if len(line) >= 1 {
			_, err := globals.ParseValidateAddress(string(line))
			if err != nil {
				if linestr, err = wallet.NameToAddress(string(strings.TrimSpace(string(line)))); err != nil {
					error_message = " " //err.Error()
				} else {

				}
			}
		}

		if error_message != "" {
			color = color_red // Should we display the error message here??
			l.SetPrompt(fmt.Sprintf("%sEnter Destination Address: ", color))
		} else {
			l.SetPrompt(fmt.Sprintf("%sEnter Destination Address: ", color))
		}

		l.Refresh()
		return nil, 0, false
	})

	line, err := l.ReadPasswordWithConfig(setPasswordCfg)
	if err != nil {
		return
	}
	if linestr == "" {
		a, err = globals.ParseValidateAddress(string(line))
	} else {
		a, err = globals.ParseValidateAddress(string(linestr))
	}

	l.SetPrompt(prompt)
	l.Refresh()
	return
}

// read an address with all goodies such as color encoding and other things in prompt
func ReadSCID(l *readline.Instance) (a crypto.Hash, err error) {
	setPasswordCfg := l.GenPasswordConfig()
	setPasswordCfg.EnableMask = false

	prompt_mutex.Lock()
	defer prompt_mutex.Unlock()

	setPasswordCfg.SetListener(func(line []rune, pos int, key rune) (newLine []rune, newPos int, ok bool) {
		error_message := ""
		color := color_green

		if len(line) >= 1 {
			err := a.UnmarshalText([]byte(string(line)))
			if err != nil {
				error_message = " " //err.Error()
			}
		}

		if error_message != "" {
			color = color_red // Should we display the error message here??
			l.SetPrompt(fmt.Sprintf("%sEnter SCID: ", color))
		} else {
			l.SetPrompt(fmt.Sprintf("%sEnter SCID: ", color))
		}

		l.Refresh()
		return nil, 0, false
	})

	line, err := l.ReadPasswordWithConfig(setPasswordCfg)
	if err != nil {
		return
	}
	err = a.UnmarshalText([]byte(string(line)))
	l.SetPrompt(prompt)
	l.Refresh()
	return
}
func ReadFloat64(l *readline.Instance, cprompt string, default_value float64) (a float64, err error) {
	setPasswordCfg := l.GenPasswordConfig()
	setPasswordCfg.EnableMask = false

	prompt_mutex.Lock()
	defer prompt_mutex.Unlock()

	setPasswordCfg.SetListener(func(line []rune, pos int, key rune) (newLine []rune, newPos int, ok bool) {
		error_message := ""
		color := color_green

		if len(line) >= 1 {
			_, err := strconv.ParseFloat(string(line), 64)
			if err != nil {
				error_message = " " //err.Error()
			}
		}

		if error_message != "" {
			color = color_red // Should we display the error message here??
			l.SetPrompt(fmt.Sprintf("%sEnter %s (default %f): ", color, cprompt, default_value))
		} else {
			l.SetPrompt(fmt.Sprintf("%sEnter %s (default %f): ", color, cprompt, default_value))

		}

		l.Refresh()
		return nil, 0, false
	})

	line, err := l.ReadPasswordWithConfig(setPasswordCfg)
	if err != nil {
		return
	}
	a, err = strconv.ParseFloat(string(line), 64)
	l.SetPrompt(cprompt)
	l.Refresh()
	return
}

func ReadUint64(l *readline.Instance, cprompt string, default_value uint64) (a uint64, err error) {
	setPasswordCfg := l.GenPasswordConfig()
	setPasswordCfg.EnableMask = false

	prompt_mutex.Lock()
	defer prompt_mutex.Unlock()

	setPasswordCfg.SetListener(func(line []rune, pos int, key rune) (newLine []rune, newPos int, ok bool) {
		error_message := ""
		color := color_green

		if len(line) == 0 {
			line = []rune(fmt.Sprintf("%d", default_value))
		}

		if len(line) >= 1 {
			_, err := strconv.ParseUint(string(line), 0, 64)
			if err != nil {
				error_message = " " //err.Error()
			}
		}

		if error_message != "" {
			color = color_red // Should we display the error message here??
			l.SetPrompt(fmt.Sprintf("%sEnter %s (default %d): ", color, cprompt, default_value))
		} else {
			l.SetPrompt(fmt.Sprintf("%sEnter %s (default %d): ", color, cprompt, default_value))

		}

		l.Refresh()
		return nil, 0, false
	})

	line, err := l.ReadPasswordWithConfig(setPasswordCfg)
	if err != nil {
		return
	}
	if len(line) == 0 {
		line = []byte(fmt.Sprintf("%d", default_value))
	}
	a, err = strconv.ParseUint(string(line), 0, 64)
	l.SetPrompt(cprompt)
	l.Refresh()
	return
}

func ReadInt64(l *readline.Instance, cprompt string, default_value int64) (a int64, err error) {
	setPasswordCfg := l.GenPasswordConfig()
	setPasswordCfg.EnableMask = false

	prompt_mutex.Lock()
	defer prompt_mutex.Unlock()

	setPasswordCfg.SetListener(func(line []rune, pos int, key rune) (newLine []rune, newPos int, ok bool) {
		error_message := ""
		color := color_green

		if len(line) >= 1 {
			_, err := strconv.ParseInt(string(line), 0, 64)
			if err != nil {
				error_message = " " //err.Error()
			}
		}

		if error_message != "" {
			color = color_red // Should we display the error message here??
			l.SetPrompt(fmt.Sprintf("%sEnter %s (default %d): ", color, cprompt, default_value))
		} else {
			l.SetPrompt(fmt.Sprintf("%sEnter %s (default %d): ", color, cprompt, default_value))

		}

		l.Refresh()
		return nil, 0, false
	})

	line, err := l.ReadPasswordWithConfig(setPasswordCfg)
	if err != nil {
		return
	}
	a, err = strconv.ParseInt(string(line), 0, 64)
	l.SetPrompt(cprompt)
	l.Refresh()
	return
}

func ReadString(l *readline.Instance, cprompt string, default_value string) (a string, err error) {
	setPasswordCfg := l.GenPasswordConfig()
	setPasswordCfg.EnableMask = false

	prompt_mutex.Lock()
	defer prompt_mutex.Unlock()

	setPasswordCfg.SetListener(func(line []rune, pos int, key rune) (newLine []rune, newPos int, ok bool) {
		error_message := ""
		color := color_green

		if len(line) < 1 {
			error_message = " " //err.Error()
		}

		if error_message != "" {
			color = color_red // Should we display the error message here??
			l.SetPrompt(fmt.Sprintf("%sEnter %s (default '%s'): ", color, cprompt, default_value))
		} else {
			l.SetPrompt(fmt.Sprintf("%sEnter %s (default '%s'): ", color, cprompt, default_value))

		}

		l.Refresh()
		return nil, 0, false
	})

	line, err := l.ReadPasswordWithConfig(setPasswordCfg)
	if err != nil {
		return
	}
	a = string(line)
	l.SetPrompt(cprompt)
	l.Refresh()
	return
}

// confirms whether the user wants to confirm yes
func ConfirmYesNoDefaultYes(l *readline.Instance, prompt_temporary string) bool {
	prompt_mutex.Lock()
	defer prompt_mutex.Unlock()

	l.SetPrompt(prompt_temporary)
	line, err := l.Readline()
	if err == readline.ErrInterrupt {
		if len(line) == 0 {
			logger.Info("Ctrl-C received, Exiting")
			os.Exit(0)
		}
	} else if err == io.EOF {
		os.Exit(0)
	}
	l.SetPrompt(prompt)
	l.Refresh()

	if strings.TrimSpace(line) == "n" || strings.TrimSpace(line) == "N" {
		return false
	}
	return true
}

// confirms whether the user wants to confirm NO
func ConfirmYesNoDefaultNo(l *readline.Instance, prompt_temporary string) bool {
	prompt_mutex.Lock()
	defer prompt_mutex.Unlock()

	l.SetPrompt(prompt_temporary)
	line, err := l.Readline()
	if err == readline.ErrInterrupt {
		if len(line) == 0 {
			logger.Info("Ctrl-C received, Exiting")
			os.Exit(0)
		}
	} else if err == io.EOF {
		os.Exit(0)
	}
	l.SetPrompt(prompt)

	if strings.TrimSpace(line) == "y" || strings.TrimSpace(line) == "Y" {
		return true
	}
	return false
}

// confirms whether user knows the current password for the wallet
// this is triggerred while transferring  amount, changing settings and so on
func ValidateCurrentPassword(l *readline.Instance, wallet *walletapi.Wallet_Disk) bool {
	prompt_mutex.Lock()
	defer prompt_mutex.Unlock()

	// if user requested wallet to be open/unlocked, keep it open
	if globals.Arguments["--unlocked"].(bool) == true {
		return true
	}

	setPasswordCfg := l.GenPasswordConfig()
	setPasswordCfg.SetListener(func(line []rune, pos int, key rune) (newLine []rune, newPos int, ok bool) {
		l.SetPrompt(fmt.Sprintf("Enter current wallet password(%v): ", len(line)))
		l.Refresh()
		return nil, 0, false
	})

	//pswd, err := l.ReadPassword("please enter your password: ")
	pswd, err := l.ReadPasswordWithConfig(setPasswordCfg)
	if err != nil {
		return false
	}

	// something was read, check whether it's the password setup in the wallet
	return wallet.Check_Password(string(pswd))
}

// reads a password to open the wallet
func ReadPassword(l *readline.Instance, filename string) string {
	prompt_mutex.Lock()
	defer prompt_mutex.Unlock()

try_again:
	setPasswordCfg := l.GenPasswordConfig()
	setPasswordCfg.SetListener(func(line []rune, pos int, key rune) (newLine []rune, newPos int, ok bool) {
		l.SetPrompt(fmt.Sprintf("Enter wallet password for %s (%v): ", filename, len(line)))
		l.Refresh()
		return nil, 0, false
	})

	//pswd, err := l.ReadPassword("please enter your password: ")
	pswd, err := l.ReadPasswordWithConfig(setPasswordCfg)
	if err != nil {
		goto try_again
	}

	// something was read, check whether it's the password setup in the wallet
	return string(pswd)
}
func ReadConfirmedPassword(l *readline.Instance, first_prompt string, second_prompt string) (password string) {
	prompt_mutex.Lock()
	defer prompt_mutex.Unlock()

	for {
		setPasswordCfg := l.GenPasswordConfig()
		setPasswordCfg.SetListener(func(line []rune, pos int, key rune) (newLine []rune, newPos int, ok bool) {
			l.SetPrompt(fmt.Sprintf("%s(%v): ", first_prompt, len(line)))
			l.Refresh()
			return nil, 0, false
		})

		password_bytes, err := l.ReadPasswordWithConfig(setPasswordCfg)
		if err != nil {
			//return
			continue
		}

		setPasswordCfg = l.GenPasswordConfig()
		setPasswordCfg.SetListener(func(line []rune, pos int, key rune) (newLine []rune, newPos int, ok bool) {
			l.SetPrompt(fmt.Sprintf("%s(%v): ", second_prompt, len(line)))
			l.Refresh()
			return nil, 0, false
		})

		confirmed_bytes, err := l.ReadPasswordWithConfig(setPasswordCfg)
		if err != nil {
			//return
			continue
		}

		if bytes.Equal(password_bytes, confirmed_bytes) {
			password = string(password_bytes)
			err = nil
			return
		}

		logger.Error(fmt.Errorf("Passwords mismatch.Retrying."), "")
	}

}

// confirms  user to press a key
// this is triggerred while transferring  amount, changing settings and so on
func PressAnyKey(l *readline.Instance, wallet *walletapi.Wallet_Disk) {

	prompt_mutex.Lock()
	defer prompt_mutex.Unlock()

	setPasswordCfg := l.GenPasswordConfig()
	setPasswordCfg.SetListener(func(line []rune, pos int, key rune) (newLine []rune, newPos int, ok bool) {

		l.SetPrompt(fmt.Sprintf("Press ENTER key to continue..."))
		l.Refresh()

		return nil, 0, false
	})

	// any error or any key is the same
	l.ReadPasswordWithConfig(setPasswordCfg)

	return
}

// this completer is used to complete the commands at the prompt
// BUG, this needs to be disabled in menu mode
var completer = readline.NewPrefixCompleter(
	readline.PcItem("help"),
	readline.PcItem("address"),
	readline.PcItem("balance"),
	readline.PcItem("token_add"),
	readline.PcItem("integrated_address"),
	readline.PcItem("get_tx_key"),
	readline.PcItem("filesign"),
	readline.PcItem("fileverify"),
	readline.PcItem("filesign_huge"),
	readline.PcItem("fileverify_huge"),
	readline.PcItem("menu"),
	readline.PcItem("rescan_bc"),
	readline.PcItem("payment_id"),
	readline.PcItem("print_height"),
	readline.PcItem("seed"),

	readline.PcItem("set",
		readline.PcItem("mixin"),
		readline.PcItem("seed"),
		readline.PcItem("priority"),
	),
	readline.PcItem("show_transfers"),
	readline.PcItem("spendkey"),
	readline.PcItem("status"),
	readline.PcItem("version"),
	readline.PcItem("transfer"),
	readline.PcItem("transfer_all"),
	readline.PcItem("bye"),
	readline.PcItem("exit"),
	readline.PcItem("quit"),
)

// help command screen
func usage(w io.Writer) {
	io.WriteString(w, "commands:\n")
	io.WriteString(w, "\t\033[1mhelp\033[0m\t\tthis help\n")
	io.WriteString(w, "\t\033[1maddress\033[0m\t\tDisplay user address\n")
	io.WriteString(w, "\t\033[1mbalance\033[0m\t\tDisplay user balance\n")
	io.WriteString(w, "\t\033[1mtoken_add\033[0m\t\tAdd token\n")
	io.WriteString(w, "\t\033[1mintegrated_address\033[0m\tDisplay random integrated address (with encrypted payment ID)\n")
	io.WriteString(w, "\t\033[1mmenu\033[0m\t\tEnable menu mode\n")
	io.WriteString(w, "\t\033[1mrescan_bc\033[0m\tRescan blockchain to re-obtain transaction history \n")
	io.WriteString(w, "\t\033[1mpassword\033[0m\tChange wallet password\n")
	io.WriteString(w, "\t\033[1mpayment_id\033[0m\tPrint random Payment ID (for encrypted version see integrated_address)\n")
	io.WriteString(w, "\t\033[1mseed\033[0m\t\tDisplay seed\n")
	io.WriteString(w, "\t\033[1mshow_transfers\033[0m\tShow all transactions to/from current wallet\n")
	io.WriteString(w, "\t\033[1mset\033[0m\t\tSet/get various settings\n")
	io.WriteString(w, "\t\033[1mstatus\033[0m\t\tShow general information and balance\n")
	io.WriteString(w, "\t\033[1mspendkey\033[0m\tView secret key\n")
	io.WriteString(w, "\t\033[1mtransfer\033[0m\tTransfer/Send DERO to another address\n")
	io.WriteString(w, "\t\t\tEg. transfer <address> <amount>\n")
	io.WriteString(w, "\t\033[1mtransfer_all\033[0m\tTransfer everything to another address\n")
	io.WriteString(w, "\t\033[1mversion\033[0m\t\tShow version\n")
	io.WriteString(w, "\t\033[1mbye\033[0m\t\tQuit wallet\n")
	io.WriteString(w, "\t\033[1mexit\033[0m\t\tQuit wallet\n")
	io.WriteString(w, "\t\033[1mquit\033[0m\t\tQuit wallet\n")

}

// display seed to the user in his preferred language
func display_seed(l *readline.Instance, wallet *walletapi.Wallet_Disk) {
	seed := wallet.GetSeed()
	fmt.Fprintf(l.Stderr(), color_green+"PLEASE NOTE: the following 25 words can be used to recover access to your wallet. Please write them down and store them somewhere safe and secure. Please do not store them in your email or on file storage services outside of your immediate control."+color_white+"\n")
	fmt.Fprintf(os.Stderr, color_red+"%s"+color_white+"\n", seed)

}

// display spend key
// viewable wallet do not have spend secret key
// TODO wee need to give user a warning if we are printing secret
func display_spend_key(l *readline.Instance, wallet *walletapi.Wallet_Disk) {

	keys := wallet.Get_Keys()
	h := "0000000000000000000000000000000000000000000000" + keys.Secret.Text(16)
	fmt.Fprintf(os.Stderr, "secret key: "+color_red+"%s"+color_white+"\n", h[len(h)-64:])

	fmt.Fprintf(os.Stderr, "public key: %s\n", keys.Public.StringHex())
}

// start a rescan from block 0
func rescan_bc(wallet *walletapi.Wallet_Disk) {
	if wallet.GetMode() { // trigger rescan we the wallet is online
		wallet.Clean() // clean existing data from wallet
		//wallet.Rescan_From_Height(0)
	}

}

func valid_registration_or_display_error(l *readline.Instance, wallet *walletapi.Wallet_Disk) bool {
	if !wallet.IsRegistered() {
		logger.Error(fmt.Errorf("Your account is not registered.Please register."), "")
	}
	return true
}

// show the transfers to the user originating from this account
func show_transfers(l *readline.Instance, wallet *walletapi.Wallet_Disk, scid crypto.Hash, limit uint64) {

	if wallet.GetMode() && walletapi.IsDaemonOnline() { // if wallet is in offline mode , we cannot do anything
		if err := wallet.Sync_Wallet_Memory_With_Daemon_internal(scid); err != nil {
			logger.Error(err, "Error syncing wallet", "scid", scid.String())
			return
		}
	}

	in := true
	out := true
	coinbase := true
	min_height := uint64(0)
	max_height := uint64(0)

	line := ""
	line_parts := strings.Fields(line)
	if len(line_parts) >= 2 {
		switch strings.ToLower(line_parts[1]) {
		case "coinbase":
			out = false
			in = false

		case "in":
			coinbase = false
			in = true
			out = false
		case "out":
			coinbase = false
			in = false
			out = true
		}
	}

	if len(line_parts) >= 3 { // user supplied min height
		s, err := strconv.ParseUint(line_parts[2], 10, 64)
		if err != nil {
			logger.Error(err, "Error parsing minimum height")
			return
		}
		min_height = s
	}

	if len(line_parts) >= 4 { // user supplied max height
		s, err := strconv.ParseUint(line_parts[2], 10, 64)
		if err != nil {
			logger.Error(err, "Error parsing maximum height")
			return
		}
		max_height = s
	}

	// request payments without payment id
	transfers := wallet.Show_Transfers(scid, coinbase, in, out, min_height, max_height, "", "", 0, 0) // receives sorted list of transfers

	if len(transfers) == 0 {
		logger.Error(nil, "No transfers available")
		return
	}
	// we need to paginate on say 20 transactions

	paging := 20

	//if limit != 0 && uint64(len(transfers)) > limit {
	//   transfers = transfers[uint64(len(transfers))-limit:]
	//}
	for i := len(transfers) - 1; i >= 0; i-- {

		switch transfers[i].Status {
		case 0:

			if transfers[i].Coinbase {
				io.WriteString(l.Stderr(), fmt.Sprintf(color_green+"%s Height %d TopoHeight %d  Coinbase (miner reward) received %s DERO"+color_white+"\n", transfers[i].Time.Format(time.RFC822), transfers[i].Height, transfers[i].TopoHeight, globals.FormatMoney(transfers[i].Amount)))

			} else {

				args, err := transfers[i].ProcessPayload()
				if err != nil {
					io.WriteString(l.Stderr(), fmt.Sprintf(color_green+"%s Height %d TopoHeight %d transaction %s received %s DERO Proof: %s"+color_white+"\n", transfers[i].Time.Format(time.RFC822), transfers[i].Height, transfers[i].TopoHeight, transfers[i].TXID, globals.FormatMoney(transfers[i].Amount), transfers[i].Proof))

					io.WriteString(l.Stderr(), fmt.Sprintf("Full Entry %+v\n", transfers[i])) // dump entire entry for debugging purposes

				} else if len(args) == 0 { // no rpc

					io.WriteString(l.Stderr(), fmt.Sprintf(color_green+"%s Height %d TopoHeight %d transaction %s received %s DERO Proof: %s NO RPC CALL"+color_white+"\n", transfers[i].Time.Format(time.RFC822), transfers[i].Height, transfers[i].TopoHeight, transfers[i].TXID, globals.FormatMoney(transfers[i].Amount), transfers[i].Proof))

				} else { // yes, its rpc
					io.WriteString(l.Stderr(), fmt.Sprintf(color_green+"%s Height %d TopoHeight %d transaction %s received %s DERO Proof: %s RPC CALL arguments %s "+color_white+"\n", transfers[i].Time.Format(time.RFC822), transfers[i].Height, transfers[i].TopoHeight, transfers[i].TXID, globals.FormatMoney(transfers[i].Amount), transfers[i].Proof, args))

				}

			}

		case 1:

			args, err := transfers[i].ProcessPayload()
			if err != nil {
				io.WriteString(l.Stderr(), fmt.Sprintf(color_yellow+"%s Height %d TopoHeight %d transaction %s spent %s DERO Destination: %s Proof: %s\n"+color_white+"\n", transfers[i].Time.Format(time.RFC822), transfers[i].Height, transfers[i].TopoHeight, transfers[i].TXID, globals.FormatMoney(transfers[i].Amount), transfers[i].Destination, transfers[i].Proof))

				io.WriteString(l.Stderr(), fmt.Sprintf("Err decoding entry %s\nFull Entry %+v\n", err, transfers[i])) // dump entire entry for debugging purposes

			} else if len(args) == 0 { // no rpc

				io.WriteString(l.Stderr(), fmt.Sprintf(color_yellow+"%s Height %d TopoHeight %d transaction %s spent %s DERO Destination: %s Proof: %s  NO RPC CALL"+color_white+"\n", transfers[i].Time.Format(time.RFC822), transfers[i].Height, transfers[i].TopoHeight, transfers[i].TXID, globals.FormatMoney(transfers[i].Amount), transfers[i].Destination, transfers[i].Proof))

			} else { // yes, its rpc
				io.WriteString(l.Stderr(), fmt.Sprintf(color_yellow+"%s Height %d TopoHeight %d transaction %s spent %s DERO Destination: %s Proof: %s RPC CALL arguments %s "+color_white+"\n", transfers[i].Time.Format(time.RFC822), transfers[i].Height, transfers[i].TopoHeight, transfers[i].TXID, globals.FormatMoney(transfers[i].Amount), transfers[i].Destination, transfers[i].Proof, args))

			}

		case 2:
			fallthrough
		default:
			logger.Error(nil, "Transaction status unknown TXID %s status %d", transfers[i].TXID, transfers[i].Status)

		}

		j := len(transfers) - i
		if j != 0 && j%paging == 0 && (j+1) < len(transfers) { // ask user whether he want to see more till he quits
			if !ConfirmYesNoDefaultNo(l, "Want to see more history (y/N)?") {
				break // break loop
			}

		}

	}

}
