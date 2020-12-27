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

import "os"
import "io"
import "fmt"
import "bytes"
import "time"

//import "io/ioutil"
//import "path/filepath"
import "strings"
import "strconv"
import "encoding/hex"

import "github.com/chzyer/readline"

import "github.com/deroproject/derohe/config"
import "github.com/deroproject/derohe/crypto"
import "github.com/deroproject/derohe/globals"
import "github.com/deroproject/derohe/address"
import "github.com/deroproject/derohe/walletapi"

var account walletapi.Account

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
			globals.Logger.Warnf("No wallet available")
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
		fmt.Fprintf(l.Stderr(), "Balance    : "+color_green+"%s"+color_white+"\n\n", globals.FormatMoney(locked_balance+balance_unlocked))

	case "rescan_bc", "rescan_spent": // rescan from 0
		if offline_mode {
			globals.Logger.Warnf("Offline wallet rescanning NOT implemented")
		} else {
			rescan_bc(wallet)
		}

	case "seed": // give user his seed, if password is valid
		if !ValidateCurrentPassword(l, wallet) {
			globals.Logger.Warnf("Invalid password")
			PressAnyKey(l, wallet)
			break
		}
		display_seed(l, wallet) // seed should be given only to authenticated users

	case "spendkey": // give user his spend key
		display_spend_key(l, wallet)

	case "password": // change wallet password
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

	case "get_tx_key":
		if !valid_registration_or_display_error(l, wallet) {
			break
		}
		if len(line_parts) == 2 && len(line_parts[1]) == 64 {
			_, err := hex.DecodeString(line_parts[1])
			if err != nil {
				globals.Logger.Warnf("Error parsing txhash")
				break
			}
			key := wallet.GetTXKey(crypto.HexToHash(line_parts[1]))
			if key != "" {
				globals.Logger.Infof("TX Proof key \"%s\"", key)
			} else {
				globals.Logger.Warnf("TX not found in database")
			}
		} else {
			globals.Logger.Warnf("get_tx_key needs transaction hash as input parameter")
			globals.Logger.Warnf("eg. get_tx_key ea551b02b9f1e8aebe4d7b1b7f6bf173d76ae614cb9a066800773fee9e226fd7")
		}
	case "sweep_all", "transfer_all": // transfer everything
		Transfer_Everything(l)

	case "show_transfers":
		show_transfers(l, wallet, 100)
	case "set": // set/display different settings
		handle_set_command(l, line)
	case "close": // close the account
		if !ValidateCurrentPassword(l, wallet) {
			globals.Logger.Warnf("Invalid password")
			break
		}
		wallet.Close_Encrypted_Wallet() // overwrite previous instance

	case "menu": // enable menu mode
		menu_mode = true
		globals.Logger.Infof("Menu mode enabled")
	case "i8", "integrated_address": // user wants a random integrated address 8 bytes
		a := wallet.GetRandomIAddress8()
		fmt.Fprintf(l.Stderr(), "Wallet integrated address : "+color_green+"%s"+color_white+"\n", a.String())
		fmt.Fprintf(l.Stderr(), "Embedded payment ID : "+color_green+"%x"+color_white+"\n", a.PaymentID)

	case "version":
		globals.Logger.Infof("Version %s\n", config.Version.String())

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
						globals.Logger.Warnf("Error Parsing \"%s\" err %s", line_parts[0], err)
						return
					}
					amount, err := globals.ParseAmount(line_parts[1])
					if err != nil {
						globals.Logger.Warnf("Error Parsing \"%s\" err %s", line_parts[1], err)
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
							globals.Logger.Warnf("Error parsing payment ID, it should be in hex 16 or 64 chars")
							return
						}
						payment_id = line_parts[0]
						line_parts = line_parts[1:]

					} else {
						globals.Logger.Warnf("Invalid payment ID \"%s\"", line_parts[0])
						return
					}

				}
			}

			// check if everything is okay, if yes build the transaction
			if len(addr_list) == 0 {
				globals.Logger.Warnf("Destination address not provided")
				return
			}

			payment_id_integrated := false
			for i := range addr_list {
				if addr_list[i].IsIntegratedAddress() {
					payment_id_integrated = true
					globals.Logger.Infof("Payment ID is integreted in address ID:%x", addr_list[i].PaymentID)
				}

			}

			// if user provided an integrated address donot ask him payment id
			// otherwise confirm whether user wants to send without payment id
			if payment_id_integrated == false && len(payment_id) == 0 {
				payment_id_bytes, err := ReadPaymentID(l)
				payment_id = hex.EncodeToString(payment_id_bytes)
				if err != nil {
					globals.Logger.Warnf("Err :%s", err)
					break
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

	case "": // blank enter key just loop
	default:
		//fmt.Fprintf(l.Stderr(), "you said: %s", strconv.Quote(line))
		globals.Logger.Warnf("No such command")
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
			globals.Logger.Warnf("Wrong number of arguments, see help eg")
			help = true
			break
		}
		s, err := strconv.ParseUint(line_parts[2], 10, 64)
		if err != nil {
			globals.Logger.Warnf("Error parsing ringsize")
			return
		}
		wallet.SetRingSize(int(s))
		globals.Logger.Infof("Ring size =  %d", wallet.GetRingSize())

	case "priority":
		if len(line_parts) != 3 {
			globals.Logger.Warnf("Wrong number of arguments, see help eg")
			help = true
			break
		}
		s, err := strconv.ParseFloat(line_parts[2], 64)
		if err != nil {
			globals.Logger.Warnf("Error parsing priority")
			return
		}
		wallet.SetFeeMultiplier(float32(s))
		globals.Logger.Infof("Transaction priority =  %.02f", wallet.GetFeeMultiplier())

	case "seed": // seed only has 1 setting, lanuage so do it now
		language := choose_seed_language(l)
		globals.Logger.Infof("Setting seed language to  \"%s\"", wallet.SetSeedLanguage(language))

	default:
		help = true
	}

	if help == true || len(line_parts) == 1 { // user type plain set command, give out all settings and help

		fmt.Fprintf(l.Stderr(), color_extra_white+"Current settings"+color_extra_white+"\n")
		fmt.Fprintf(l.Stderr(), color_normal+"Seed Language: "+color_extra_white+"%s\t"+color_normal+"eg. "+color_extra_white+"set seed language\n"+color_normal, wallet.GetSeedLanguage())
		fmt.Fprintf(l.Stderr(), color_normal+"Ringsize: "+color_extra_white+"%d\t"+color_normal+"eg. "+color_extra_white+"set ringsize 16\n"+color_normal, wallet.GetRingSize())
		fmt.Fprintf(l.Stderr(), color_normal+"Priority: "+color_extra_white+"%0.2f\t"+color_normal+"eg. "+color_extra_white+"set priority 4.0\t"+color_normal+"Transaction priority on DERO network \n", wallet.GetFeeMultiplier())
		fmt.Fprintf(l.Stderr(), "\t\tMinimum priority is 1.00. High priority = high fees\n")

	}
}

func Transfer_Everything(l *readline.Instance) {
	/*
		if wallet.Is_View_Only() {
			fmt.Fprintf(l.Stderr(), color_yellow+"View Only wallet cannot transfer."+color_white)
		}

		if !ValidateCurrentPassword(l, wallet) {
			globals.Logger.Warnf("Invalid password")
			return
		}

		// a , amount_to_transfer, err := collect_transfer_info(l,wallet)
		addr, err := ReadAddress(l)
		if err != nil {
			globals.Logger.Warnf("Err :%s", err)
			return
		}

		var payment_id []byte
		// if user provided an integrated address donot ask him payment id
		if !addr.IsIntegratedAddress() {
			payment_id, err = ReadPaymentID(l)
			if err != nil {
				globals.Logger.Warnf("Err :%s", err)
				return
			}
		} else {
			globals.Logger.Infof("Payment ID is integreted in address ID:%x", addr.PaymentID)
		}

		fees_per_kb := uint64(0) // fees  must be calculated by walletapi

		tx, inputs, input_sum, err := wallet.Transfer_Everything(*addr, hex.EncodeToString(payment_id), 0, fees_per_kb, 5)

		_ = inputs
		if err != nil {
			globals.Logger.Warnf("Error while building Transaction err %s\n", err)
			return
		}
		globals.Logger.Infof("%d Inputs Selected for %s DERO", len(inputs), globals.FormatMoney12(input_sum))
		globals.Logger.Infof("fees %s DERO", globals.FormatMoneyPrecision(tx.RctSignature.Get_TX_Fee(), 12))
		globals.Logger.Infof("TX Size %0.1f KiB (should be  < 240 KiB)", float32(len(tx.Serialize()))/1024.0)
		offline_tx := false
		if ConfirmYesNoDefaultNo(l, "Confirm Transaction (y/N)") {

			if offline_tx { // if its an offline tx, dump it to a file
				cur_dir, err := os.Getwd()
				if err != nil {
					globals.Logger.Warnf("Cannot obtain current directory to save tx")
					return
				}
				filename := filepath.Join(cur_dir, tx.GetHash().String()+".tx")
				err = ioutil.WriteFile(filename, []byte(hex.EncodeToString(tx.Serialize())), 0600)
				if err == nil {
					if err == nil {
						globals.Logger.Infof("Transaction saved successfully. txid = %s", tx.GetHash())
						globals.Logger.Infof("Saved to %s", filename)
					} else {
						globals.Logger.Warnf("Error saving tx to %s , err %s", filename, err)
					}
				}

			} else {

				err = wallet.SendTransaction(tx) // relay tx to daemon/network
				if err == nil {
					globals.Logger.Infof("Transaction sent successfully. txid = %s", tx.GetHash())
				} else {
					globals.Logger.Warnf("Transaction sending failed txid = %s, err %s", tx.GetHash(), err)
				}

			}
		}
	*/

}

// read an address with all goodies such as color encoding and other things in prompt
func ReadAddress(l *readline.Instance) (a *address.Address, err error) {
	setPasswordCfg := l.GenPasswordConfig()
	setPasswordCfg.EnableMask = false

	prompt_mutex.Lock()
	defer prompt_mutex.Unlock()

	setPasswordCfg.SetListener(func(line []rune, pos int, key rune) (newLine []rune, newPos int, ok bool) {
		error_message := ""
		color := color_green

		if len(line) >= 1 {
			_, err := globals.ParseValidateAddress(string(line))
			if err != nil {
				error_message = " " //err.Error()
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
	a, err = globals.ParseValidateAddress(string(line))
	l.SetPrompt(prompt)
	l.Refresh()
	return
}

/*
// read an payment with all goodies such as color encoding and other things in prompt
func ReadPaymentID(l *readline.Instance) (payment_id []byte, err error) {
	setPasswordCfg := l.GenPasswordConfig()
	setPasswordCfg.EnableMask = false

	// ask user whether he want to enter a payment ID

	if !ConfirmYesNoDefaultNo(l, "Provide Payment ID (y/N)") { // user doesnot want to provide payment it, skip
		return
	}

	prompt_mutex.Lock()
	defer prompt_mutex.Unlock()

	setPasswordCfg.SetListener(func(line []rune, pos int, key rune) (newLine []rune, newPos int, ok bool) {
		error_message := ""
		color := color_green

		if len(line) >= 1 {
			_, err := hex.DecodeString(string(line))
			if (len(line) == 16 || len(line) == 64) && err == nil {
				error_message = ""
			} else {
				error_message = " " //err.Error()
			}
		}

		if error_message != "" {
			color = color_red // Should we display the error message here??
			l.SetPrompt(fmt.Sprintf("%sEnter Payment ID (16/64 hex char): ", color))
		} else {
			l.SetPrompt(fmt.Sprintf("%sEnter Payment ID (16/64 hex char): ", color))

		}

		l.Refresh()
		return nil, 0, false
	})

	line, err := l.ReadPasswordWithConfig(setPasswordCfg)
	if err != nil {
		return
	}
	payment_id, err = hex.DecodeString(string(line))
	if err != nil {
		return
	}
	l.SetPrompt(prompt)
	l.Refresh()

	if len(payment_id) == 8 || len(payment_id) == 32 {
		return
	}

	err = fmt.Errorf("Invalid Payment ID")
	return
}
*/

// confirms whether the user wants to confirm yes
func ConfirmYesNoDefaultYes(l *readline.Instance, prompt_temporary string) bool {
	prompt_mutex.Lock()
	defer prompt_mutex.Unlock()

	l.SetPrompt(prompt_temporary)
	line, err := l.Readline()
	if err == readline.ErrInterrupt {
		if len(line) == 0 {
			globals.Logger.Infof("Ctrl-C received, Exiting\n")
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
			globals.Logger.Infof("Ctrl-C received, Exiting\n")
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

		globals.Logger.Warnf("Passwords mismatch.Retrying.")
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

/*
// if we are in offline, scan default or user provided file
// this function will replay the blockchain data in offline mode
func trigger_offline_data_scan() {
	filename := default_offline_datafile

	if globals.Arguments["--offline_datafile"] != nil {
		filename = globals.Arguments["--offline_datafile"].(string)
	}

	f, err := os.Open(filename)
	if err != nil {
		globals.Logger.Warnf("Cannot read offline data file=\"%s\"  err: %s   ", filename, err)
		return
	}
	w := bufio.NewReader(f)
	gzipreader, err := gzip.NewReader(w)
	if err != nil {
		globals.Logger.Warnf("Error while decompressing offline data file=\"%s\"  err: %s   ", filename, err)
		return
	}
	defer gzipreader.Close()
	io.Copy(pipe_writer, gzipreader)
}
*/

// this completer is used to complete the commands at the prompt
// BUG, this needs to be disabled in menu mode
var completer = readline.NewPrefixCompleter(
	readline.PcItem("help"),
	readline.PcItem("address"),
	readline.PcItem("balance"),
	readline.PcItem("integrated_address"),
	readline.PcItem("get_tx_key"),
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
	readline.PcItem("walletviewkey"),
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
	io.WriteString(w, "\t\033[1mget_tx_key\033[0m\tDisplay tx proof to prove receiver for specific transaction\n")
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
    h := "0000000000000000000000000000000000000000000000"+keys.Secret.Text(16)
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

func is_registered(wallet *walletapi.Wallet_Disk) bool {
	if wallet.Get_Registration_TopoHeight() == -1 {
		return false
	}
	return true
}

func valid_registration_or_display_error(l *readline.Instance, wallet *walletapi.Wallet_Disk) bool {
	if !is_registered(wallet) {
		globals.Logger.Warnf("Your account is not registered.Please register.")
	}
	return true
}

// show the transfers to the user originating from this account
func show_transfers(l *readline.Instance, wallet *walletapi.Wallet_Disk, limit uint64) {

	available := true
	in := true
	out := true
	pool := true    // this is not processed still TODO list
	failed := false // this is not processed still TODO list
	min_height := uint64(0)
	max_height := uint64(0)

	line := ""
	line_parts := strings.Fields(line)
	if len(line_parts) >= 2 {
		switch strings.ToLower(line_parts[1]) {
		case "available":
			available = true
			in = false
			out = false
			pool = false
			failed = false
		case "in":
			available = true
			in = true
			out = false
			pool = false
			failed = false
		case "out":
			available = false
			in = false
			out = true
			pool = false
			failed = false
		case "pool":
			available = false
			in = false
			out = false
			pool = true
			failed = false
		case "failed":
			available = false
			in = false
			out = false
			pool = false
			failed = true

		}
	}

	if len(line_parts) >= 3 { // user supplied min height
		s, err := strconv.ParseUint(line_parts[2], 10, 64)
		if err != nil {
			globals.Logger.Warnf("Error parsing minimum height")
			return
		}
		min_height = s
	}

	if len(line_parts) >= 4 { // user supplied max height
		s, err := strconv.ParseUint(line_parts[2], 10, 64)
		if err != nil {
			globals.Logger.Warnf("Error parsing maximum height")
			return
		}
		max_height = s
	}

	// request payments without payment id
	transfers := wallet.Show_Transfers(available, in, out, pool, failed, false, min_height, max_height) // receives sorted list of transfers

	if len(transfers) == 0 {
		globals.Logger.Warnf("No transfers available")
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

			} else if len(transfers[i].PaymentID) == 0 {
				io.WriteString(l.Stderr(), fmt.Sprintf(color_green+"%s Height %d TopoHeight %d transaction %s received %s DERO"+color_white+"\n", transfers[i].Time.Format(time.RFC822), transfers[i].Height, transfers[i].TopoHeight, transfers[i].TXID, globals.FormatMoney(transfers[i].Amount)))
			} else {
				payment_id := fmt.Sprintf("%x", transfers[i].PaymentID)
				io.WriteString(l.Stderr(), fmt.Sprintf(color_green+"%s Height %d TopoHeight %d transaction %s received %s DERO"+color_white+" PAYMENT ID:%s\n", transfers[i].Time.Format(time.RFC822), transfers[i].Height, transfers[i].TopoHeight, transfers[i].TXID, globals.FormatMoney(transfers[i].Amount), payment_id))
			}

		case 1:
			payment_id := fmt.Sprintf("%x", transfers[i].PaymentID)
			io.WriteString(l.Stderr(), fmt.Sprintf(color_magenta+"%s Height %d TopoHeight %d transaction %s spent %s DERO"+color_white+" PAYMENT ID: %s  Proof:%s\n", transfers[i].Time.Format(time.RFC822), transfers[i].Height, transfers[i].TopoHeight, transfers[i].TXID, globals.FormatMoney(transfers[i].Amount), payment_id, transfers[i].Proof))
		case 2:
			fallthrough
		default:
			globals.Logger.Warnf("Transaction status unknown TXID %s status %d", transfers[i].TXID, transfers[i].Status)

		}

		j := len(transfers) - i
		if j != 0 && j%paging == 0 && (j+1) < len(transfers) { // ask user whether he want to see more till he quits
			if !ConfirmYesNoDefaultNo(l, "Want to see more history (y/N)?") {
				break // break loop
			}

		}

	}

}
