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
import "fmt"
import "time"
import "strconv"
import "strings"
import "encoding/hex"

import "github.com/chzyer/readline"

import "github.com/deroproject/derohe/cryptography/crypto"
import "github.com/deroproject/derohe/config"
import "github.com/deroproject/derohe/globals"
import "github.com/deroproject/derohe/walletapi"
import "github.com/deroproject/derohe/walletapi/rpcserver"

// display menu before a wallet is opened
func display_easymenu_pre_open_command(l *readline.Instance) {
	w := l.Stderr()
	io.WriteString(w, "Menu:\n")
	io.WriteString(w, "\t\033[1m1\033[0m\tOpen existing Wallet\n")
	io.WriteString(w, "\t\033[1m2\033[0m\tCreate New Wallet\n")
	io.WriteString(w, "\t\033[1m3\033[0m\tRecover Wallet using recovery seed (25 words)\n")
	io.WriteString(w, "\t\033[1m4\033[0m\tRecover Wallet using recovery key (64 char private spend key hex)\n")
	io.WriteString(w, "\n\t\033[1m9\033[0m\tExit menu and start prompt\n")
	io.WriteString(w, "\t\033[1m0\033[0m\tExit Wallet\n")
}

// handle all commands
func handle_easymenu_pre_open_command(l *readline.Instance, line string) {
	var err error

	line = strings.TrimSpace(line)
	line_parts := strings.Fields(line)

	if len(line_parts) < 1 { // if no command return
		return
	}

	command := ""
	if len(line_parts) >= 1 {
		command = strings.ToLower(line_parts[0])
	}

	var wallett *walletapi.Wallet_Disk

	//account_state := account_valid
	switch command {
	case "1": // open existing wallet
		filename := choose_file_name(l)

		// ask user a password
		for i := 0; i < 3; i++ {
			wallett, err = walletapi.Open_Encrypted_Wallet(filename, ReadPassword(l, filename))
			if err != nil {
				logger.Error(err, "Error occurred while opening wallet file", "filename", filename)
				wallet = nil
				break
			} else { //  user knows the password and is db is valid
				break
			}
		}
		if wallett != nil {
			wallet = wallett
			wallett = nil
			logger.Info("Successfully opened wallet")

			common_processing(wallet)
		}

	case "2": // create a new random account

		filename := choose_file_name(l)

		password := ReadConfirmedPassword(l, "Enter password", "Confirm password")

		wallett, err = walletapi.Create_Encrypted_Wallet_Random(filename, password)
		if err != nil {
			logger.Error(err, "Error occurred while creating wallet file", "filename", filename)
			wallet = nil
			break

		}
		err = wallett.Set_Encrypted_Wallet_Password(password)
		if err != nil {
			logger.Error(err, "Error changing password")
		}
		wallet = wallett
		wallett = nil

		seed_language := choose_seed_language(l)
		wallet.SetSeedLanguage(seed_language)
		logger.V(1).Info("Seed", "Language", seed_language)

		display_seed(l, wallet)

		common_processing(wallet)

	case "3": // create wallet from recovery words

		filename := choose_file_name(l)
		password := ReadConfirmedPassword(l, "Enter password", "Confirm password")
		electrum_words := read_line_with_prompt(l, "Enter seed (25 words) : ")

		wallett, err = walletapi.Create_Encrypted_Wallet_From_Recovery_Words(filename, password, electrum_words)
		if err != nil {
			logger.Error(err, "Error while recovering wallet using seed.")
			break
		}
		wallet = wallett
		wallett = nil
		//globals.Logger.Debugf("Seed Language %s", account.SeedLanguage)
		logger.Info("Successfully recovered wallet from seed")
		common_processing(wallet)

	case "4": // create wallet from  hex seed

		filename := choose_file_name(l)
		password := ReadConfirmedPassword(l, "Enter password", "Confirm password")

		seed_key_string := read_line_with_prompt(l, "Please enter your seed ( hex 64 chars): ")

		seed_raw, err := hex.DecodeString(seed_key_string) // hex decode
		if len(seed_key_string) >= 65 || err != nil {      //sanity check
			logger.Error(err, "Seed must be less than 66 chars hexadecimal chars")
			break
		}

		wallett, err = walletapi.Create_Encrypted_Wallet(filename, password, new(crypto.BNRed).SetBytes(seed_raw))
		if err != nil {
			logger.Error(err, "Error while recovering wallet using seed key")
			break
		}
		logger.Info("Successfully recovered wallet from hex seed")
		wallet = wallett
		wallett = nil
		seed_language := choose_seed_language(l)
		wallet.SetSeedLanguage(seed_language)
		logger.V(1).Info("Seed", "Language", seed_language)

		display_seed(l, wallet)
		common_processing(wallet)
		/*
		   	case "5": // create new view only wallet // TODO user providing wrong key is not being validated, do it ASAP

		   		filename := choose_file_name(l)
		   		view_key_string := read_line_with_prompt(l, "Please enter your View Only Key ( hex 128 chars): ")

		   		password := ReadConfirmedPassword(l, "Enter password", "Confirm password")
		   		wallet, err = walletapi.Create_Encrypted_Wallet_ViewOnly(filename, password, view_key_string)

		   		if err != nil {
		   			globals.Logger.Warnf("Error while reconstructing view only wallet using view key err %s\n", err)
		   			break
		   		}

		   		if globals.Arguments["--offline"].(bool) == true {
		   			//offline_mode = true
		   		} else {
		   			wallet.SetOnlineMode()
		   		}
		           case "6": // create non deterministic wallet // TODO user providing wrong key is not being validated, do it ASAP

		   		filename := choose_file_name(l)
		   		spend_key_string := read_line_with_prompt(l, "Please enter your Secret spend key ( hex 64 chars): ")
		                   view_key_string := read_line_with_prompt(l, "Please enter your Secret view key ( hex 64 chars): ")

		   		password := ReadConfirmedPassword(l, "Enter password", "Confirm password")
		   		wallet, err = walletapi.Create_Encrypted_Wallet_NonDeterministic(filename, password, spend_key_string,view_key_string)

		   		if err != nil {
		   			globals.Logger.Warnf("Error while reconstructing view only wallet using view key err %s\n", err)
		   			break
		   		}

		   		if globals.Arguments["--offline"].(bool) == true {
		   			//offline_mode = true
		   		} else {
		   			wallet.SetOnlineMode()
		   		}
		*/
	case "9":
		menu_mode = false
		logger.Info("Prompt mode enabled")
	case "0", "bye", "exit", "quit":
		globals.Exit_In_Progress = true
	default: // just loop

	}
	//_ = account_state

	// NOTE: if we are in online mode, it is handled automatically
	// user opened or created a new account
	// rescan blockchain in offline mode
	//if account_state == false && account_valid && offline_mode {
	//	go trigger_offline_data_scan()
	//}

}

// sets online mode, starts RPC server etc
func common_processing(wallet *walletapi.Wallet_Disk) {
	if globals.Arguments["--offline"].(bool) == true {
		//offline_mode = true
	} else {
		wallet.SetOnlineMode()
	}

	if globals.Arguments["--scan-top-n-blocks"] != nil && globals.Arguments["--scan-top-n-blocks"].(string) != "" {
		s, err := strconv.ParseInt(globals.Arguments["--scan-top-n-blocks"].(string), 10, 64)
		if err != nil {
			logger.Error(err, "Error parsing number(in numeric form)")
		} else {
			wallet.SetTrackRecentBlocks(s)
			if wallet.SetTrackRecentBlocks(-1) == 0 {
				logger.Info("Wallet will track entire history")
			} else {
				logger.Info("Wallet will track recent blocks", "blocks", wallet.SetTrackRecentBlocks(-1))
			}
		}
	}

	if globals.Arguments["--save-every-x-seconds"] != nil && globals.Arguments["--save-every-x-seconds"].(string) != "" {
		s, err := strconv.ParseUint(globals.Arguments["--save-every-x-seconds"].(string), 10, 64)
		if err != nil {
			logger.Error(err, "Error parsing seconds(in numeric form)")
		} else {
			wallet.SetSaveDuration(time.Duration(s) * time.Second)
			logger.Info("Wallet changes will be saved every", "duration (seconds)", wallet.SetSaveDuration(-1))
		}
	}

	wallet.SetNetwork(!globals.Arguments["--testnet"].(bool))

	// start rpc server if requested
	if globals.Arguments["--rpc-server"].(bool) == true {
		rpc_address := "127.0.0.1:" + fmt.Sprintf("%d", config.Mainnet.Wallet_RPC_Default_Port)

		if !globals.IsMainnet() {
			rpc_address = "127.0.0.1:" + fmt.Sprintf("%d", config.Testnet.Wallet_RPC_Default_Port)
		}

		if globals.Arguments["--rpc-bind"] != nil {
			rpc_address = globals.Arguments["--rpc-bind"].(string)
		}
		logger.Info("Starting RPC server", "address", rpc_address)

		if _, err := rpcserver.RPCServer_Start(wallet, "walletrpc"); err != nil {
			logger.Error(err, "Error starting rpc server")

		}
	}
	time.Sleep(time.Second)

	// init_script_engine(wallet) // init script engine
	// init_plugins_engine(wallet) // init script engine

}
