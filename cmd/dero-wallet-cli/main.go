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

/// this file implements the wallet and rpc wallet

import "io"
import "os"
import "fmt"
import "time"
import "sync"
import "strings"
import "strconv"
import "runtime"
import "encoding/hex"
import "sync/atomic"
import "github.com/deroproject/derohe/transaction"

import "github.com/go-logr/logr"
import "github.com/chzyer/readline"
import "github.com/docopt/docopt-go"

import "github.com/deroproject/derohe/config"
import "github.com/deroproject/derohe/cryptography/bn256"
import "github.com/deroproject/derohe/cryptography/crypto"
import "github.com/deroproject/derohe/globals"
import	"github.com/deroproject/derohe/rpc"
import "github.com/deroproject/derohe/walletapi"
import "github.com/deroproject/derohe/walletapi/mnemonics"

var command_line string = `dero-wallet-cli 
DERO : A secure, private blockchain with smart-contracts

Usage:
  dero-wallet-cli [options] 
  dero-wallet-cli -h | --help
  dero-wallet-cli --version

  Options:
  -h --help     Show this screen.
  --version     Show version.
  --wallet-file=<file>   Use this file to restore or create new wallet
  --password=<password>  Use this password to unlock the wallet
  --offline     Run the wallet in offline (signing) mode. An online (view only) wallet is required to create the transaction & sync to the network
  Full wallet & offline wallet:  
     --viewingkey  Print the viewing key and exit
     --regtx       Generate the registration transaction with required POW and exit
     --remotetx    Process a balance or transaction sign request from a view only wallet and exit
     --prefix=<.>  The path to search for the remotetx requests.
  --prompt      Disable menu and display prompt
  --testnet     Run in testnet mode.
  --debug       Debug mode enabled, print log messages
  --unlocked    Keep wallet unlocked for cli commands (Does not confirm password before commands)
  --generate-new-wallet            Create a new wallet, using a randomly generated seed  
  --restore-viewonly-wallet        Restore a view only wallet. The offline (signing) wallet contains the secret key & can export the view only key  
  --restore-deterministic-wallet   Restore wallet from previously saved recovery seed
  --electrum-seed=<recovery-seed>  Seed to use while restoring wallet
  --socks-proxy=<socks_ip:port>    Use a proxy to connect to Daemon.
  --remote      use hard coded remote daemon https://rwallet.dero.live
  --daemon-address=<host:port>     Use daemon instance at <host>:<port> or https://domain
  --rpc-server  Run rpc server, so wallet is accessible using api
  --rpc-bind=<127.0.0.1:20209>     Wallet binds on this ip address and port
  --rpc-login=<username:password>  RPC server will grant access based on these credentials
  --allow-rpc-password-change      RPC server will change password if you send "Pass" header with new password
  --scan-top-n-blocks=<100000>     Only scan top N blocks
  --save-every-x-seconds=<300>     Save wallet every x seconds
  `
var menu_mode bool = true // default display menu mode
// var account_valid bool = false                        // if an account has been opened, do not allow to create new account in this session
var offline_mode bool             // whether we are in offline mode
var remote_request_prefix string
var sync_in_progress int          //  whether sync is in progress with daemon
var wallet *walletapi.Wallet_Disk //= &walletapi.Account{} // all account  data is available here
// var address string
var sync_time time.Time // used to suitable update  prompt

var logger logr.Logger = logr.Discard() // default discard all logs

var color_black = "\033[30m"
var color_red = "\033[31m"
var color_green = "\033[32m"
var color_yellow = "\033[33m"
var color_blue = "\033[34m"
var color_magenta = "\033[35m"
var color_cyan = "\033[36m"
var color_white = "\033[37m"
var color_extra_white = "\033[1m"
var color_normal = "\033[0m"

var prompt_mutex sync.Mutex // prompt lock
var prompt string = "\033[92mDERO Wallet:\033[32m>>>\033[0m "

var tablock uint32

func main() {
	var err error

	globals.Arguments, err = docopt.Parse(command_line, nil, true, "DERO atlantis wallet : work in progress", false)
	if err != nil {
		fmt.Printf("Error while parsing options err: %s\n", err)
		return
	}

	// init the lookup table one, anyone importing walletapi should init this first, this will take around 1 sec on any recent system
	if os.Getenv("USE_BIG_TABLE") != "" {
		fmt.Printf("Please wait, generating precompute table....")
		walletapi.Initialize_LookupTable(1, 1<<24) // use 8 times more more ram, around 256 MB RAM
		fmt.Printf("done\n")
	} else {
		walletapi.Initialize_LookupTable(1, 1<<21)
	}

	// We need to initialize readline first, so it changes stderr to ansi processor on windows
	l, err := readline.NewEx(&readline.Config{
		//Prompt:          "\033[92mDERO:\033[32m»\033[0m",
		Prompt:          prompt,
		HistoryFile:     "", // wallet never saves any history file anywhere, to prevent any leakage
		AutoComplete:    completer,
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",

		HistorySearchFold:   true,
		FuncFilterInputRune: filterInput,
	})
	if err != nil {
		panic(err)
	}
	defer l.Close()

	// get ready to grab passwords
	setPasswordCfg := l.GenPasswordConfig()
	setPasswordCfg.SetListener(func(line []rune, pos int, key rune) (newLine []rune, newPos int, ok bool) {
		l.SetPrompt(fmt.Sprintf("Enter password(%v): ", len(line)))
		l.Refresh()
		return nil, 0, false
	})
	l.Refresh() // refresh the prompt

	// parse arguments and setup logging , print basic information
	exename, _ := os.Executable()
	f, err := os.Create(exename + ".log")
	if err != nil {
		fmt.Printf("Error while opening log file err: %s filename %s\n", err, exename+".log")
		return
	}
	globals.InitializeLog(l.Stdout(), f)
	logger = globals.Logger.WithName("wallet")

	logger.Info("DERO Wallet  :  It is an alpha version, use it for testing/evaluations purpose only.")
	logger.Info("Copyright 2017-2021 DERO Project. All rights reserved.")
	logger.Info("", "OS", runtime.GOOS, "ARCH", runtime.GOARCH, "GOMAXPROCS", runtime.GOMAXPROCS(0))
	logger.Info("", "Version", config.Version.String())
	logger.V(1).Info("", "Arguments", globals.Arguments)
	globals.Initialize() // setup network and proxy
	logger.V(0).Info("", "MODE", globals.Config.Name)

	// disable menu mode if requested
	if globals.Arguments["--prompt"] != nil && globals.Arguments["--prompt"].(bool) {
		menu_mode = false
	}

	offline_mode = globals.Arguments["--offline"].(bool)

	wallet_file := "wallet.db" //dero.wallet"
	if globals.Arguments["--wallet-file"] != nil {
		wallet_file = globals.Arguments["--wallet-file"].(string) // override with user specified settings
	}

	wallet_password := "" // default
	if globals.Arguments["--password"] != nil {
		wallet_password = globals.Arguments["--password"].(string) // override with user specified settings
	}

	// lets handle the arguments one by one:
	// Mutually exclusive commands:
	//	--restore-deterministic-wallet
	//	--generate-new-wallet
	//
	if globals.Arguments["--restore-deterministic-wallet"].(bool) {
		// user wants to recover wallet, check whether seed is provided on command line, if not prompt now
		seed := ""

		if globals.Arguments["--electrum-seed"] != nil {
			seed = globals.Arguments["--electrum-seed"].(string)
		} else { // prompt user for seed
			seed = read_line_with_prompt(l, "Enter your seed (25 words) : ")
		}

		account, err := walletapi.Generate_Account_From_Recovery_Words(seed)
		if err != nil {
			logger.Error(err, "Error while recovering seed.")
			return
		}

		// ask user a pass, if not provided on command_line
		password := ""
		if wallet_password == "" {
			password = ReadConfirmedPassword(l, "Enter password", "Confirm password")
		} else {
			//Use provided password from the command line
			password = wallet_password
		}

		wallet, err = walletapi.Create_Encrypted_Wallet(wallet_file, password, account)
		if err != nil {
			logger.Error(err, "Error occurred while restoring wallet")
			return
		}

		if (offline_mode==false) {
			logger.V(1).Info("Seed Language", "language", account.SeedLanguage)
			logger.Info("Successfully recovered wallet from seed")
		} else {
			fmt.Printf("Successfully recovered wallet from seed. Your address: %s\n", wallet.GetAddress() )
			globals.Exit_In_Progress=true
		}
	} else if globals.Arguments["--generate-new-wallet"] != nil && globals.Arguments["--generate-new-wallet"].(bool) {
		// generare new random account
		var filename string
		if globals.Arguments["--wallet-file"] != nil && len(globals.Arguments["--wallet-file"].(string)) > 0 {
			filename = globals.Arguments["--wallet-file"].(string)
		} else {
			filename = choose_file_name(l)
		}
		// Check right at the beginning if the file exist
	        if _, err = os.Stat(filename); err == nil {
	                fmt.Printf("File '%s' already exists\n", filename)
        	        return
		}

		// ask user a pass, if not provided on command_line
		password := ""
		if wallet_password == "" {
			password = ReadConfirmedPassword(l, "Enter password", "Confirm password")
                } else {
                        //Use provided password from the command line
                        password = wallet_password
                }		

		seed_language := choose_seed_language(l)
		_ = seed_language
		wallet, err = walletapi.Create_Encrypted_Wallet_Random(filename, password)
		if err != nil {
			logger.Error(err, "Error occured while creating new wallet.")
			wallet = nil
			return
		}
		if wallet==nil {
			logger.Error(err, "Internal error: Could not initialise the wallet.")
			return
		}
		logger.V(1).Info("Seed Language", "language", account.SeedLanguage)
		display_seed(l, wallet)
	} else if globals.Arguments["--restore-viewonly-wallet"]!=nil && globals.Arguments["--restore-viewonly-wallet"].(bool) {
		// Create a 'view only' account using the details obtained from the offline (signing) wallet
	
		filename := choose_file_name(l)
		// Check right at the beginning if the file exist
	        if _, err = os.Stat(filename); err == nil {
	                fmt.Printf("File '%s' already exists\n", filename)
        	        return
		}
		
                // ask user a pass, if not provided on command_line
                password := ""
                if wallet_password == "" {
                        password = ReadConfirmedPassword(l, "Enter password", "Confirm password")
                } else {
                        password = wallet_password
                }

		//Format: [0] - preamble: viewkey
		//        [1] - Address
		//        [2] - Public key              
		//        [3] - Public key internal data
		//        [4] - Checksum
                var sViewOnly = read_line_with_prompt(l, "Enter the view only key (obtained from the offline (signing) wallet): ")

                //Strip off any newlines or extra spaces
                sTmp := strings.ReplaceAll(sViewOnly,"\n","")
                sViewOnly = strings.ReplaceAll(sTmp," ","");
                
                saParts := strings.Split(sViewOnly, ";")
                if (len(saParts)!=2) {
                	fmt.Printf("Invalid number of parts in the input. Expected 2 found %d\n",len(saParts));
                	return	
		}                
		
		sViewKey := saParts[0]
		sProtocolChecksum := saParts[1]
		iTmp,err := strconv.Atoi(sProtocolChecksum)
		if err!=nil {
			fmt.Fprintf(l.Stderr(),"Could not convert the checksum back to an integer: "+sProtocolChecksum+"\n")
			return
		}				
		iProtocolChecksum:=uint16(iTmp)
		
		//Regenerate checksum:
		var iCalculatedChecksum uint16
		iCalculatedChecksum=0x1
		for t := range sViewKey {
			iCalculatedChecksum = iCalculatedChecksum + (uint16)(sViewKey[t])
		}
		
		// Check 1: Checksum
		if (iProtocolChecksum != iCalculatedChecksum) {
			fmt.Printf("Checksum calculation failed. Please check if you've imported the view key correctly\n");
			return
		}                               
		
		saParts = strings.Split(sViewKey,",")
		if (len(saParts) != 4) {
			fmt.Fprintf(l.Stderr(), "Invalid number of parts. Expected 4, found %d\n", len(saParts))
			return
		}
		
		if (saParts[0]!="viewkey") {
			fmt.Fprintf(l.Stderr(), "Input doesn't start with 'viewkey'\n");
			return
		}	
                
		//Send: Public key, public key internals
		account,err   := walletapi.Generate_Account_From_ViewOnly_params(saParts[2],saParts[3], globals.IsMainnet() )
                if err != nil {
                        logger.Error(err, "Error while recovering view only parameters.")
                        return
                }
		                
                wallet, err = walletapi.Create_Encrypted_Wallet(filename, password, account)
                
                if err != nil {
                        logger.Error(err, "Error occurred while restoring wallet")
                        return
                }

		//Double check that the restored public key generates the expected dero address
                sAddress := fmt.Sprintf("%s",wallet.GetAddress().String() )
		if (sAddress != saParts[1]) {
			logger.Error(err, "The addres containted in the viewing key (%s) doesn't match the restored address (%s)\n", saParts[1], sAddress);
			return
		}

                fmt.Printf("Successfully restored an online (view only) wallet\n")	 	
                fmt.Printf("  Address: %s\n",sAddress)
                fmt.Printf("  Public key: %s\n", wallet.Get_Keys().Public.StringHex())
	}

	if globals.Arguments["--rpc-login"] != nil {
		userpass := globals.Arguments["--rpc-login"].(string)
		parts := strings.SplitN(userpass, ":", 2)

		if len(parts) != 2 {
			logger.Error(fmt.Errorf("RPC user name or password invalid"), "cannot set password on wallet rpc")
			return
		}
		logger.Info("Wallet RPC", "username", parts[0], "password", parts[1])
	}

	// if wallet is nil,  check whether the file exists, if yes, request password
	if wallet == nil {
		if _, err = os.Stat(wallet_file); err == nil {

			// if a wallet file and password  has been provide, make sure that the wallet opens in 1st attempt, othwer wise exit

			if globals.Arguments["--password"] != nil {
				wallet, err = walletapi.Open_Encrypted_Wallet(wallet_file, wallet_password)
				if err != nil {
					logger.Error(err, "Error occurred while opening wallet.")
					os.Exit(-1)
				}
			} else { // request user the password

				// ask user a password
				for i := 0; i < 3; i++ {
					wallet, err = walletapi.Open_Encrypted_Wallet(wallet_file, ReadPassword(l, wallet_file))
					if err != nil {
						logger.Error(err, "Error occurred while opening wallet.")
					} else { //  user knows the password and is db is valid
						break
					}
				}
			}
			//globals.Logger.Debugf("Seed Language %s", account.SeedLanguage)
			//globals.Logger.Infof("Successfully recovered wallet from seed")
		}
	}

	if wallet == nil {
		logger.Error(err, "Error occurred while opening wallet.")
		return
	}
	common_processing(wallet)
	
	go walletapi.Keep_Connectivity() // maintain connectivity
	
	bResult := globals.Arguments["--viewingkey"].(bool)
	if (bResult==true) {
		display_viewing_key(wallet)
		return;
	}

	bResult = globals.Arguments["--regtx"].(bool)
	if (bResult==true) {
		if (wallet.ViewOnly() == true) {
			fmt.Fprintf(l.Stderr(), "A view only wallet cannot generate the registration transaction\n");
			return;
		}
		fmt.Fprintf(l.Stderr(), "Generating registration transaction for wallet address : "+color_green+"%s"+color_white+"\n", wallet.GetAddress())

		successful_regs := make(chan *transaction.Transaction)
		counter := 0
		counter2 := 0
		var reg_tx *transaction.Transaction
		for i := 0; i < runtime.GOMAXPROCS(0); i++ {
			go func() {
				for counter == 0 {
					lreg_tx := wallet.GetRegistrationTX()
					hash := lreg_tx.GetHash()

					if hash[0] == 0 && hash[1] == 0 && hash[2] == 0 {
						fmt.Printf("Found transaction:\n");
						successful_regs <- lreg_tx
						counter++
						break
					} else {
						counter2++
						if ((counter2 % 10000) == 0) {
							//Match usually found round about 2 million mark
							fmt.Printf("Searched %d hashes\n",counter2)
						}
					}
				}
			}()
		}

		reg_tx = <-successful_regs

		// Offline wallet: print the prepared transaction, to be used in the online wallet
		fmt.Printf("Found the registration transaction. Import the complete text into the online (view only) wallet:\n");
		sTransaction := fmt.Sprintf("registration,%s,%x,%s",wallet.GetAddress().String(), reg_tx.Serialize(), reg_tx.GetHash())
		
		//Append a simple checksum to the string to detect copy/paste errors
		//during import into the online wallet:
		var iChecksum=1
		for t := range sTransaction {
			iChecksum = iChecksum + (int)(sTransaction[t])
		}
		
		fmt.Printf("%s;%d\n\n",sTransaction, iChecksum)
		return
	} 

	//Any remote request to decrypt/sign?	
	//  The 'remote_request' file must already be present on the filesystem
	remote_request_prefix="."
	if globals.Arguments["--prefix"] != nil {
		remote_request_prefix = globals.Arguments["--prefix"].(string) // override with user specified settings
	}
	
	bResult = globals.Arguments["--remotetx"].(bool)
	if (bResult==true) {
		if (wallet.ViewOnly() == true) {
			fmt.Fprintf(l.Stderr(), "A view only wallet cannot process remote transactions\n");
			return;
		}
		
		err = process_remote_requests(true,remote_request_prefix)
		if err!=nil {
			fmt.Printf("\n\nRemote request error: %s\n",err)
		}
		return
	}
	
	//pipe_reader, pipe_writer = io.Pipe() // create pipes

	// reader ready to parse any data from the file
	//go blockchain_data_consumer()

	// update prompt when required
	prompt_mutex.Lock()
	go update_prompt(l)
	prompt_mutex.Unlock()

	// if wallet has been opened in offline mode by commands supplied at command prompt
	// trigger the offline scan

	//	go trigger_offline_data_scan()

	// start infinite loop processing user commands
	for {

		prompt_mutex.Lock()
		if globals.Exit_In_Progress { // exit if requested so
			prompt_mutex.Unlock()
			break
		}
		prompt_mutex.Unlock()

		if menu_mode { // display menu if requested
			if wallet != nil { // account is opened, display post menu
				display_easymenu_post_open_command(l)
			} else { // account has not been opened display pre open menu
				display_easymenu_pre_open_command(l)
			}
		}

		line, err := l.Readline()
		if err == readline.ErrInterrupt {
			if len(line) == 0 {
				logger.Info("Ctrl-C received, Exit in progress")
				globals.Exit_In_Progress = true
				break
			} else {
				continue
			}
		} else if err == io.EOF {
			//			break
			time.Sleep(time.Second)
		}

		// pass command to suitable handler
		if menu_mode {
			if wallet != nil {
				if !handle_easymenu_post_open_command(l, line) { // if not processed , try processing as command
					handle_prompt_command(l, line)
					PressAnyKey(l, wallet)
				}
			} else {
				handle_easymenu_pre_open_command(l, line)
			}
		} else {
			handle_prompt_command(l, line)
		}

	}
	prompt_mutex.Lock()
	globals.Exit_In_Progress = true
	prompt_mutex.Unlock()

}

// update prompt as and when necessary
// TODO: make this code simple, with clear direction
func update_prompt(l *readline.Instance) {

	last_wallet_height := uint64(0)
	last_daemon_height := int64(0)
	daemon_online := false
	last_update_time := int64(0)

	// show first 8 bytes of address
	address_trim := ""
	if wallet != nil {
		tmp_addr := wallet.GetAddress().String()
		address_trim = tmp_addr[0:8]
	} else {
		address_trim = "DERO Wallet"
	}	
	
	

	
	for {
		time.Sleep(30 * time.Millisecond) // give user a smooth running number

		prompt_mutex.Lock()
		if globals.Exit_In_Progress {
			prompt_mutex.Unlock()
			return
		}
		prompt_mutex.Unlock()

		if atomic.LoadUint32(&tablock) > 0 { // tab key has been presssed,  stop delivering updates to  prompt
			continue
		}

		prompt_mutex.Lock() // do not update if we can not lock the mutex



		if wallet == nil {
			l.SetPrompt(fmt.Sprintf("\033[1m\033[32m%s \033[0m"+color_green+"0/%d \033[32m>>>\033[0m ", address_trim, walletapi.Get_Daemon_Height()))
			l.Refresh()
			prompt_mutex.Unlock()
			continue
		}

		// only update prompt if needed, or update atleast once every second

		_ = daemon_online

		//fmt.Printf("chekcing if update is required\n")
		// Dero blocktime ~18 seconds. Check for new blocks every 15 seconds
		if last_wallet_height != wallet.Get_Height() || last_daemon_height != walletapi.Get_Daemon_Height() ||
			(time.Now().Unix()-last_update_time) >= 15 {
			// choose color based on urgency
			color := "\033[32m" // default is green color
			if wallet.Get_Height() < wallet.Get_Daemon_Height() {
				color = "\033[33m" // make prompt yellow
			}

			//dheight := walletapi.Get_Daemon_Height()

			/*if wallet.IsDaemonOnlineCached() == false {
				color = "\033[33m" // make prompt yellow
				dheight = 0
			}*/

			balance_string := ""

			//balance_unlocked, locked_balance := wallet.Get_Balance_Rescan()// wallet.Get_Balance()
			balance_unlocked, _ := wallet.Get_Balance()
			balance_string = fmt.Sprintf(color_green+"%s "+color_white, globals.FormatMoney(balance_unlocked))

			if wallet.Error != nil {
				balance_string += fmt.Sprintf(color_red+" %s ", wallet.Error)
			} /*else if wallet.PoolCount() > 0 {
				balance_string += fmt.Sprintf(color_yellow+"(%d tx pending for -%s)", wallet.PoolCount(), globals.FormatMoney(wallet.PoolBalance()))
			}*/

			testnet_string := ""
			if !globals.IsMainnet() {
				testnet_string = "\033[31m TESTNET"
			}

			l.SetPrompt(fmt.Sprintf("\033[1m\033[32m%s \033[0m"+color+"%d/%d %s %s\033[32m>>>\033[0m ", address_trim, wallet.Get_Height(), walletapi.Get_Daemon_Height(), balance_string, testnet_string))
			l.Refresh()
			last_wallet_height = wallet.Get_Height()
			last_daemon_height = walletapi.Get_Daemon_Height()
			last_update_time = time.Now().Unix()
			//daemon_online = wallet.IsDaemonOnlineCached()
			_ = last_update_time

		}

		prompt_mutex.Unlock()
		
		//test for an incomming request to interact with the secret key
		//The online (view only) wallet uses this to reconstruct the account balance & transaction history		
		var err error;
		err=process_remote_requests(false,remote_request_prefix)
		if err!=nil {
			fmt.Printf("Remote request error: %s\n",err)
		}
	}
}

// create a new wallet from scratch from random numbers
func Create_New_Wallet(l *readline.Instance) (w *walletapi.Wallet_Disk, err error) {

	// ask user a file name to store the data

	walletpath := read_line_with_prompt(l, "Please enter wallet file name : ")
	walletpassword := ""

	account, _ := walletapi.Generate_Keys_From_Random()
	account.SeedLanguage = choose_seed_language(l)

	w, err = walletapi.Create_Encrypted_Wallet(walletpath, walletpassword, account)

	if err != nil {
		return
	}

	// set wallet seed language

	// a new account has been created, append the seed to user home directory

	//usr, err := user.Current()
	/*if err != nil {
	      globals.Logger.Warnf("Cannot get current username to save recovery key and password")
	  }else{ // we have a user, get his home dir


	  }*/

	return
}

/*

// create a new wallet from hex seed provided
func Create_New_Account_from_seed(l *readline.Instance) *walletapi.Account {

	var account *walletapi.Account
	var seedkey crypto.Key

	seed := read_line_with_prompt(l, "Please enter your seed ( hex 64 chars): ")
	seed = strings.TrimSpace(seed)          // trim any extra space
	seed_raw, err := hex.DecodeString(seed) // hex decode
	if len(seed) != 64 || err != nil {      //sanity check
		globals.Logger.Warnf("Seed must be 64 chars hexadecimal chars")
		return account
	}

	copy(seedkey[:], seed_raw[:32])                            // copy bytes to seed
	account, _ = walletapi.Generate_Account_From_Seed(seedkey) // create a new account
	account.SeedLanguage = choose_seed_language(l)             // ask user his seed preference and set it

	account_valid = true

	return account
}

// create a new wallet from viewable seed provided
// viewable seed consists of public spend key and private view key
func Create_New_Account_from_viewable_key(l *readline.Instance) *walletapi.Account {

	var seedkey crypto.Key
	var privateview crypto.Key

	var account *walletapi.Account
	seed := read_line_with_prompt(l, "Please enter your View Only Key ( hex 128 chars): ")

	seed = strings.TrimSpace(seed) // trim any extra space

	seed_raw, err := hex.DecodeString(seed)
	if len(seed) != 128 || err != nil {
		globals.Logger.Warnf("View Only key must be 128 chars hexadecimal chars")
		return account
	}

	copy(seedkey[:], seed_raw[:32])
	copy(privateview[:], seed_raw[32:64])

	account, _ = walletapi.Generate_Account_View_Only(seedkey, privateview)

	account_valid = true

	return account
}
*/
// helper function to let user to choose a seed in specific lanaguage
func choose_seed_language(l *readline.Instance) string {
	languages := mnemonics.Language_List()
	fmt.Printf("Language list for seeds, please enter a number (default English)\n")
	for i := range languages {
		fmt.Fprintf(l.Stderr(), "\033[1m%2d:\033[0m %s\n", i, languages[i])
	}

	language_number := read_line_with_prompt(l, "Please enter a choice: ")
	choice := 0 // 0 for english

	if s, err := strconv.Atoi(language_number); err == nil {
		choice = s
	}

	for i := range languages { // if user gave any wrong or ot of range choice, choose english
		if choice == i {
			return languages[choice]
		}
	}
	// if no match , return Englisg
	return "English"

}

// lets the user choose a filename or use default
func choose_file_name(l *readline.Instance) (filename string) {

	default_filename := "wallet.db"
	if globals.Arguments["--wallet-file"] != nil {
		default_filename = globals.Arguments["--wallet-file"].(string) // override with user specified settings
	}

	filename = read_line_with_prompt(l, fmt.Sprintf("Enter wallet filename (default %s): ", default_filename))

	if len(filename) < 1 {
		filename = default_filename
	}

	return
}

// read a line from the prompt
// since we cannot query existing, we can get away by using password mode with
func read_line_with_prompt(l *readline.Instance, prompt_temporary string) string {
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
	return line

}

// filter out specfic inputs from input processing
// currently we only skip CtrlZ background key
func filterInput(r rune) (rune, bool) {
	switch r {
	// block CtrlZ feature
	case readline.CharCtrlZ:
		return r, false
	case readline.CharTab:
		atomic.StoreUint32(&tablock, 1) // lock prompt update
	case readline.CharEnter:
		atomic.StoreUint32(&tablock, 0) // enable prompt update
	}
	return r, true
}

//Look for a request file (./offline_request) from the online wallet. The wallet with 
//the secret key (full wallet or offline wallet) can process the request.
//The online (view only) wallet uses the response to reconstruct the account balance,
//transaction history and to broadcast signed transactions
//Input: bSignTransactions - If starting app with --remotetx then balance requests and 
//                           transaction sign requests will be processed.
//                         - If running interactively (without --remotetx) then only 
//                           balance requests will be processed automatically. The user 
//                           has to choose explicitly from the menu to sign a transaction
func process_remote_requests(bSignTransactions bool,sPrefix string) (err error) {
	if (wallet.ViewOnly() == false) {
		sFileRequest:=sPrefix+"/offline_request"

		if _, err := os.Stat(sFileRequest); err != nil {
			return nil
		}
		
		fmt.Printf("Found %s/offline_request -- new decryption request\n",sPrefix)
		
		baData, err := os.ReadFile(sFileRequest)
		if err!=nil {
			err = fmt.Errorf("Could not read from %s. Check the file permissions.\n",sFileRequest);
			return err
		}  
		
		_ = os.Remove(sFileRequest)
		if _, err = os.Stat(sFileRequest); err == nil {
			err = fmt.Errorf("Could not delete %s\n",sFileRequest)
			return err		                    
		}

		//Parameter   [0]: Project - 'dero'
		//            [1]: Version - Layout of the command fields
		//            [2]: header: scalar_mult, shared_secret, sign_offline
		// Version 1: 
		//           scalar_mult & shared_secret:
		//            [3] el.Right
		//           sign_offline
		//            [3]..[11]
		//             ;  Checksum of all the characters in the data stream
		sInput := string(baData[:])
		sInput  = strings.TrimSpace(sInput)
		saParts := strings.Split(sInput,";")
		if (len(saParts) != 2) {
			err = fmt.Errorf("Invalid number of parts in the transaction. Expected 2, found %d\n", len(saParts))
			return err
		}
		
		var iCalculatedChecksum uint16
		iCalculatedChecksum=0x01;
		for t := range saParts[0] {
			iVal := uint16(saParts[0][t])
			iCalculatedChecksum = iCalculatedChecksum + iVal;
		}

		sProtocolChecksum := saParts[1]		
		iTmp,err := strconv.Atoi(sProtocolChecksum)
	        if err!=nil {
        	        err = fmt.Errorf("Could not convert the checksum back to an integer: "+saParts[1]+" ; "+sProtocolChecksum+"\n")
        	        return err
	        }
	        iProtocolChecksum:=uint16(iTmp)

		
		if (iProtocolChecksum!=iCalculatedChecksum) {
			err = fmt.Errorf("The checksum of the request data is invalid. Protocol: %u, Calculates: %u\n", iProtocolChecksum, iCalculatedChecksum)
			return err
		}			
			
		saFields := strings.Split(saParts[0]," ")
		if (len(saFields) < 4) {
			err = fmt.Errorf("Invalid number of parts in the transaction. Expected at least 4, found %d\n", len(saFields))
			return err
		}

		if  (saFields[0] != "dero") {
			err = fmt.Errorf("Expected a Dero transaction, Found %s\n",saFields[0]);
			return err
		}

		if (saFields[1] != "1") {
			err = fmt.Errorf("Only transaction version 1 supported. Found %s\n",saFields[1])
			return err
		}

		if ((saFields[2] != "scalar_mult") && 
						(saFields[2] != "shared_secret") &&
						(saFields[2] != "sign_offline")) {
			err = fmt.Errorf("Transaction doesn't start with 'scalar_mult', 'shared_secret' or 'sign_offline'\n")
			return err
		}
				
		keys := wallet.Get_Keys()
		if (saFields[2]=="scalar_mult") {
			if (len(saFields) != 4) {
				err = fmt.Errorf("Invalid number of parts in the transaction. Expected 4, found %d\n", len(saFields))
				return err
			}
		
			baData,err = hex.DecodeString(saFields[3])
			if err!=nil {
				err = fmt.Errorf("Could not hex decode the data portion\n");
				return err		                	
			}
		
			var elRight    *bn256.G1                    
			elRight = new(bn256.G1)       
			elRight.Unmarshal(baData)

			scalarMultResult := new(bn256.G1).ScalarMult(elRight, keys.Secret.BigInt())
			baData = scalarMultResult.Marshal()
			
			sOutput := fmt.Sprintf("dero 1 scalar_mult_result %x",baData)
			var iCalculatedChecksum uint16
			iCalculatedChecksum=0x01
			for t := range sOutput {
				iCalculatedChecksum = iCalculatedChecksum + (uint16)(sOutput[t])
			}
			sOutput = fmt.Sprintf("%s;%d",sOutput, iCalculatedChecksum)
			baData = []byte(sOutput)
		} else if (saFields[2]=="shared_secret") {
			if (len(saFields) != 4) {
				err = fmt.Errorf("Invalid number of parts in the transaction. Expected 4, found %d\n", len(saFields))
				return err
			}

			baData,err = hex.DecodeString(saFields[3])
			if err!=nil {
				err = fmt.Errorf("Could not hex decode the data portion\n");
				return err		                	
			}

			fmt.Printf("processing shared_secret request\n");
			var peer_publickey    *bn256.G1                    
			peer_publickey = new(bn256.G1)       
			peer_publickey.Unmarshal(baData)
			
			shared_key := crypto.GenerateSharedSecret(keys.Secret.BigInt(), peer_publickey)
			
			sOutput := fmt.Sprintf("dero 1 shared_secret_result %x",shared_key)
			var iCalculatedChecksum uint16
			iCalculatedChecksum=0x01
			for t := range sOutput {
				iCalculatedChecksum = iCalculatedChecksum + (uint16)(sOutput[t])
			}
			sOutput = fmt.Sprintf("%s;%d",sOutput, iCalculatedChecksum)
			baData = []byte(sOutput)
		} else if (saFields[2]=="sign_offline") {
			if (len(saFields) < 12) {
				err = fmt.Errorf("Invalid number of parts in the transaction. Expected 12, found %d\n", len(saFields))
				return err
			}
			
			if (bSignTransactions==false) {
				//Process sign transaction request. If running in interactive mode,
				//the user must select 'sign' from the menu to authorise the activity
				//During interactive mode, the input filename is 'transaction'
				err = os.WriteFile(sPrefix+"/transaction", baData, 0644)
				if err!=nil {
					err = fmt.Errorf("Error saving file: %s\n",err)
					return err
				}
				
				fmt.Printf("Detected new transaction sign request. Authorise the request with menu option '5: Sign'.\n");
				//User must authorise the signature through the menu system:
				//  5. Sign (DERO) transaction, prepared by the online (view only) wallet
				return nil
			} else {
				//Sign the transaction when launching app with --remotetx flag:
				sOutput:=string(baData[:])
				baData,err = sign_remote_transaction(sOutput)
				if err!=nil {
					err = fmt.Errorf("Error signing transaction: %s\n",err)
					return err;
				}
			}
		} else { 
			err = fmt.Errorf("Unknown type request. Only scalar_mult and shared_secret supported\n");
			return err
		}
		
		err = os.WriteFile(sPrefix+"/offline_response", baData, 0644)
		if err!=nil {
			err = fmt.Errorf("Error saving file. %s\n",err)
			return err
		}
		fmt.Printf("Saved result in %s/offline_response\n",sPrefix)
	}

	return nil
}

func sign_remote_transaction(sTransaction string) (baData []byte, err error){
	// Transaction structure in the file:
	//Parameter   [0] Project - 'dero'
	//            [1] Version - Layout of the command fields
	//            [2] Command: sign_offline
	// Version 1: [3] Array of transfers (outputs)
	//            [4] Array of ring balances
	//            [5] Array of rings
	//            [6] block_hash
	//            [7] height
	//            [8] Array of scdata
	//            [9] treehash
	//            [10] max_bits
	//            [11] gasstorage
	//            [12] account balance
	//             ;   Checksum of all the characters in the command.

	//Split string on ';'
	saParts := strings.Split(sTransaction,";")
	if (len(saParts) != 2) {
		err = fmt.Errorf("Invalid number of parts. Expected 2, found %d\n\n", len(saParts))
		return nil,err
	}

	sTransaction = saParts[0]
	sProtocolChecksum := saParts[1]
	iTmp,err := strconv.Atoi(sProtocolChecksum)	
	if err!=nil {
		err = fmt.Errorf("Could not convert the checksum back to an integer: "+sProtocolChecksum+"\n")
		return nil,err
	}
	iProtocolChecksum := uint16(iTmp)

	//Regenerate checksum:
	var iCalculatedChecksum uint16
	iCalculatedChecksum=0x01
	for t := range sTransaction {
		iCalculatedChecksum = iCalculatedChecksum + (uint16)(sTransaction[t])
	}

	//fmt.Printf("Checksum input\n'%s'\n\n",sTransaction);
	// Check 1: Checksum
	if (iProtocolChecksum != iCalculatedChecksum) {
		err = fmt.Errorf("Checksum calculation failed. Please check if you've imported the transaction correctly. Protocol: %u, calculated: %u\n\n", iProtocolChecksum, iCalculatedChecksum)
		return nil,err
	}                               

	saParts = strings.Split(sTransaction," ")
	if (len(saParts)!=13) {
		err = fmt.Errorf("Invalid number of parts in the transaction. Expected 13, found %d\n", len(saParts))
		return nil,err
	}
	
	if  (saParts[0] != "dero") {
		err = fmt.Errorf("Expected a Dero transaction, Found %s\n",saParts[1]);
		return nil,err
	}

	if (saParts[1] != "1") {
		err = fmt.Errorf("Only transaction version 1 supported. Found %s\n",saParts[2])
		return nil,err
	}
														
	if (saParts[2] != "sign_offline") {
		err = fmt.Errorf("Transaction doesn't start with 'sign_offline'\n")
		return nil,err
	}

	//sChecksumInput:="dero 1 sign_offline ";
	//---------------------------------------------------------------------------------------------------------------------         
	var transfers []rpc.Transfer
	
	sTmp := strings.ReplaceAll(saParts[3],"'","")
	sTmp1:= strings.ReplaceAll(sTmp, "[","")                
	sTmp  = strings.ReplaceAll(sTmp1,"]","")

	saTransfers := strings.SplitAfter(sTmp, "}")
	//fmt.Printf("transfers:%d\n",len(saTransfers)-1);
	//sChecksumInput+="transfers:";
	for t:=range saTransfers {
		if len(saTransfers[t])>0 {
			var transfer rpc.Transfer
		
			sTmp1 = strings.ReplaceAll(saTransfers[t], ",{","")             
			sTmp  = strings.ReplaceAll(sTmp1, "{","")                                               
			sTransfer := strings.ReplaceAll(sTmp,"}","")
			
			saParts := strings.Split(sTransfer,",")
			if (len(saParts)!=5) {
				err = fmt.Errorf("Parse error. Invalid number of parts in transfers. Expected 5, found %d\n", len(saParts));
				return nil,err
			}
			
			sTmp:=strings.ReplaceAll(saParts[0],"\"","")
			saItemValue:=strings.Split(sTmp,":")
			if (saItemValue[0]!="SCID") {
				err = fmt.Errorf("Parse error. Could not find SCID in transfers\n");
				return nil,err
			}
			err = transfer.SCID.UnmarshalText([]byte(string(saItemValue[1])))
			if err != nil {
				err = fmt.Errorf("Parse error. Could not assign SCID to transfers\n");
				return nil,err
			}
			//sChecksumInput+=" \""+saItemValue[1]+"\""
			
			
			sTmp=strings.ReplaceAll(saParts[1],"\"","")
			saItemValue=strings.Split(sTmp,":")
			if (saItemValue[0]!="Destination") {
				err = fmt.Errorf("Parse error. Could not find destination in transfers\n");
				return nil,err
			}
			transfer.Destination = saItemValue[1]					
			//sChecksumInput+=" \""+saItemValue[1]+"\""
			
			sTmp=strings.ReplaceAll(saParts[2],"\"","")
			saItemValue=strings.Split(sTmp,":")
			if (saItemValue[0]!="Amount") {
				err = fmt.Errorf("Parse error. Could not find amount in transfers\n");
				return nil,err
			}
			transfer.Amount, err = strconv.ParseUint(saItemValue[1],10,64)					
			//sChecksumInput+=" "+saItemValue[1]
				
			sTmp=strings.ReplaceAll(saParts[3],"\"","")
			saItemValue=strings.Split(sTmp,":")
			if (saItemValue[0]!="Burn") {
				err = fmt.Errorf("Parse error. Could not find burn in transfers\n");
				return nil,err
			}
			transfer.Burn, err = strconv.ParseUint(saItemValue[1],10,64)					
			//sChecksumInput+=" "+saItemValue[1]
			
			sTmp=strings.ReplaceAll(saParts[4],"\"","")
			saItemValue=strings.Split(sTmp,":")
			if (saItemValue[0]!="RPC") {
				err = fmt.Errorf("Parse error. Could not find rpc in transfers\n");
				return nil,err
			}
			hexRPC, err2 := hex.DecodeString(saItemValue[1])
			if err2 != nil {
				err = fmt.Errorf("Parse error. Could not hex decode the rpc field of the transfers\n");
				return nil,err
			} 
			err2 = transfer.Payload_RPC.UnmarshalBinary(hexRPC)
			if err2 != nil {
				err = fmt.Errorf("Parse error. Could not assign the rpc field of the transfers\n");
				return nil,err
			}
			//sChecksumInput+=" \""+saItemValue[1]+"\""
			
			transfers = append(transfers, transfer)
		}
	}

	//---------------------------------------------------------------------------------------------------------------------         
	var rings_balances [][][]byte //initialize all maps

	sTmp  = strings.ReplaceAll(saParts[4],"'","")
	sTmp1 = strings.ReplaceAll(sTmp, "[","")                
	sTmp  = strings.ReplaceAll(sTmp1,"]","")

	saRingBalances := strings.SplitAfter(sTmp, "}")

	//sChecksumInput+=" rings_balances:"
	for t:=range saRingBalances {
		if len(saRingBalances[t])>0 {
			var ring_balances  [][]byte
		
			sTmp1 = strings.ReplaceAll(saRingBalances[t], ",{","")          
			sTmp  = strings.ReplaceAll(sTmp1, "{","")
			sRingBalance := strings.ReplaceAll(sTmp,"}","")
			
			saRingBalances:=strings.Split(sRingBalance,",")
			if ( len(saRingBalances)<16) {
				err = fmt.Errorf("Expected at least 16 ring balances. Found %d\n", len(saRingBalances))
				return nil,err
			}
			
			for t := range saRingBalances {
				baData,_ := hex.DecodeString( saRingBalances[t] )
				ring_balances = append(ring_balances,baData)
				//sChecksumInput+=" "+saRingBalances[t]
			}
			rings_balances = append(rings_balances, ring_balances)
		}
	}
	/*				
	fmt.Printf("rings_balances:{%d} entries\n",len(rings_balances))
	var counter1=0
	var counter2=0
	for t := range rings_balances {         
		fmt.Printf("rings_balances{%d}:{%d} entries\n",counter1,len(rings_balances[t]))
		counter2=0
		for u := range rings_balances[t] {
			fmt.Printf("  ring balance[%d]=\"%x\"\n",counter2,rings_balances[t][u])
			counter2++
		}
		counter1++
	} 
	*/
	//---------------------------------------------------------------------------------------------------------------------
	var rings [][]*bn256.G1
	//sChecksumInput+=" rings:";

	sTmp  = strings.ReplaceAll(saParts[5],"'","")
	sTmp1 = strings.ReplaceAll(sTmp, "[","")                
	sTmp  = strings.ReplaceAll(sTmp1,"]","")

	saRings := strings.SplitAfter(sTmp, "}")
	//fmt.Printf("Rings:%d\n",len(saRings)-1);                

	for t:=range saRings {
		if len(saRings[t])>0 {
			var ring []*bn256.G1
			var oG1    *bn256.G1                    
		
			sTmp1 = strings.ReplaceAll(saRings[t], ",{","")         
			sTmp  = strings.ReplaceAll(sTmp1, "{","")
			sRing := strings.ReplaceAll(sTmp,"}","")
			
			saRing:=strings.Split(sRing,",")
			//fmt.Printf("saRing=%d\n",len(saRing))
			if ( len(saRing)<16) {
				err = fmt.Errorf("Expected at least 16 rings. Found %d\n\n", len(saRing))
				return nil,err
			}
			
			for t := range saRing {
				//fmt.Printf("Processing: '%s'\n", saRing[t])
				baData,err := hex.DecodeString( saRing[t] )
				if err!=nil {
					err = fmt.Errorf("Could not decode ring entry: %s\n\n", saRing[t])
					return nil,err
				}           
				oG1 = new(bn256.G1);                                      
				_,err = oG1.Unmarshal(baData);      
				if err != nil { 
					err = fmt.Errorf("Could not assign ring data\n\n");
					return nil,err
				}; 
				ring = append(ring,oG1)
							
				//sChecksumInput+=" "+saRing[t]
			}
			rings = append(rings, ring)
		}
	}
	/*	
	fmt.Printf("rings:{%d} entries\n",len(rings))
	counter1=0
	counter2=0
	for t := range rings {
		fmt.Printf("ring{%d}:{%d} entries\n",counter1,len(rings[t]))
		counter2=0
		for u := range rings[t] {
			fmt.Printf("  ring[%d]=\"%x\"\n",counter2,rings[t][u].Marshal() )
			counter2++
		}
		counter1++
	}       
	*/
	//---------------------------------------------------------------------------------------------------------------------
	//[6] block_hash
	sTmp  = strings.ReplaceAll(saParts[6],"\"","")
	block_hash := crypto.HashHexToHash(sTmp)
	//sChecksumInput+=" "+saParts[6]
	
	//[7] height
	var height uint64
	height, err = strconv.ParseUint(saParts[7],10,64)
	if err!=nil {
		err = fmt.Errorf("Parse error. Could not assign height to transfers\n\n");
		return nil,err
	}
	//sChecksumInput+=" "+saParts[7]
	
	//[8] Array of scdata
	var scdata rpc.Arguments
	sTmp  = strings.ReplaceAll(saParts[8],"'","")
	sTmp1 = strings.ReplaceAll(sTmp, "[","")                
	sTmp  = strings.ReplaceAll(sTmp1,"]","")			
	if (len(sTmp) > 0) {
		hexSCData, err2 := hex.DecodeString(sTmp)
		if err2!=nil {
			err = fmt.Errorf("Parse error. Could not decode the SCData\n\n");
			return nil,err
		}                       
		if (len(hexSCData)>0) {
			err = scdata.UnmarshalBinary(hexSCData)
			if err!=nil {
				err = fmt.Errorf("Parse error. Could not decode the SCData\n\n");
				return nil,err
			}
		}       
	}
	//sChecksumInput+=" "+saParts[8]
	
	//[9] treehash
	sTmp  = strings.ReplaceAll(saParts[9],"\"","")
	treehash_raw, err := hex.DecodeString(sTmp)
	if err != nil {
		err = fmt.Errorf("Parse error. Could not decode treehash_raw\n\n")
		return nil,err
	}
	//sChecksumInput+=" "+saParts[9]
	
	//[10] max_bits
	var max_bits int
	max_bits, err = strconv.Atoi(saParts[10])
        if err != nil {
                err = fmt.Errorf("Parse error. Could not decode max_bits\n\n")
                return nil,err
        }
	
	//[11] gasstorage
	var gasstorage uint64
	gasstorage, err = strconv.ParseUint(saParts[11],16,64)
        if err != nil {
                err = fmt.Errorf("Parse error. Could not decode gasstorage\n\n")
                return nil,err
        }	
	
	//[12] current balance
	var current_balance uint64
	current_balance, err = strconv.ParseUint(saParts[12],16,64)
        if err != nil {
                err = fmt.Errorf("Parse error. Could not decode current_balancew\n\n")
                return nil,err
        }	
        fmt.Printf("Account balance: %s\n", globals.FormatMoney(current_balance) )
	
	tx := wallet.BuildTransaction(transfers, rings_balances, rings, block_hash, height, scdata, treehash_raw, max_bits, gasstorage)
	if tx == nil {
		err = fmt.Errorf("The transaction could not be reconstructed, please retry\n\n")
		return nil,err
	}
	
	sTxSerialized := tx.Serialize()
	sOutput := fmt.Sprintf("dero 1 signed %x",sTxSerialized)
	
	iCalculatedChecksum=0x01;
	for t := range sOutput {
		iVal := uint16(sOutput[t])
		iCalculatedChecksum = iCalculatedChecksum + iVal;
	}
	sCalculatedChecksum := fmt.Sprintf("%d",iCalculatedChecksum)
	sOutput+=";"+sCalculatedChecksum	
	
	baData = []byte( sOutput )	
	
	return baData,nil
}
