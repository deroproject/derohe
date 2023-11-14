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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"strconv"
	"encoding/hex"
	
	"github.com/chzyer/readline"
	"github.com/deroproject/derohe/cryptography/bn256"	
	"github.com/deroproject/derohe/cryptography/crypto"
	"github.com/deroproject/derohe/globals"
	"github.com/deroproject/derohe/rpc"
	"github.com/deroproject/derohe/transaction"
)

//import "github.com/deroproject/derohe/address"

// handle menu if a wallet is currently opened
func display_easymenu_post_open_command(l *readline.Instance) {
	w := l.Stderr()
	
	bViewOnly := wallet.ViewOnly()
	bOffline := globals.Arguments["--offline"].(bool)
	
        if (bViewOnly==true) {
                io.WriteString(w, "Online (view only) wallet:\n");
		io.WriteString(w, " 1. Register you account, using registration transaction from the offline (signing) wallet.\n")
                io.WriteString(w, " 2. View your account balance & transaction history\n")
                io.WriteString(w, " 3. Generate transactions for the offline wallet to sign.\n")
                io.WriteString(w, " 4. Submit the signed transactions to the network.\n\n")
        }
        if (bOffline==true) {
        	io.WriteString(w, "Offline (signing) wallet:\n")
        	io.WriteString(w, " 1. Exported public key to setup the online (view only) wallet\n")
        	io.WriteString(w, " 2. Generate a registration transaction for the online wallet\n");
        	io.WriteString(w, " 3. Sign spend transactions for the online wallet\n");
        }

	
	io.WriteString(w, "Menu:\n")

	io.WriteString(w, "\t\033[1m1\033[0m\tDisplay account Address \n")
	if ( bViewOnly==false ) {
		io.WriteString(w, "\t\033[1m2\033[0m\tDisplay Seed "+color_red+"(Please save seed in safe location)\n\033[0m")
	}

	io.WriteString(w, "\t\033[1m3\033[0m\tDisplay Keys (hex)\n")

	if (bOffline==true) {
		io.WriteString(w, "\t\033[1m4\033[0m\tGenerate registration transaction for the online (view only) wallet\n")	
		io.WriteString(w, "\t\033[1m5\033[0m\tSign (DERO) transaction, prepared by the online (view only) wallet\n")
	} else {
		if !wallet.IsRegistered() {
			io.WriteString(w, "\t\033[1m4\033[0m\tAccount registration to blockchain (registration has no fee requirement and is precondition to use the account)\n")			
		} else {
			io.WriteString(w, "\t\033[1m4\033[0m\tDisplay wallet pool\n")
			if ( bViewOnly==false ) {
				io.WriteString(w, "\t\033[1m5\033[0m\tTransfer (send  DERO) to Another Wallet\n")
				io.WriteString(w, "\t\033[1m6\033[0m\tToken transfer to another wallet\n")
			} else {
				io.WriteString(w, "\t\033[1m5\033[0m\tPrepare (DERO) transaction (for the offline wallet to sign)\n")
// Not yet implemented/tested	io.WriteString(w, "\t\033[1m6\033[0m\tPrepare token transaction (for the offline wallet to sign)\n")		
			}
			io.WriteString(w, "\n")
		}
	}

	io.WriteString(w, "\t\033[1m7\033[0m\tChange wallet password\n")
	io.WriteString(w, "\t\033[1m8\033[0m\tClose Wallet\n")
	if wallet.IsRegistered() && (bOffline==false) {
		// Commands applicable only to online wallets
		if (bViewOnly==false) {
			io.WriteString(w, "\t\033[1m12\033[0m\tTransfer all balance (send  DERO) To Another Wallet\n")
		}
		io.WriteString(w, "\t\033[1m13\033[0m\tShow transaction history\n")
		io.WriteString(w, "\t\033[1m14\033[0m\tRescan transaction history\n")
		io.WriteString(w, "\t\033[1m15\033[0m\tExport all transaction history in json format\n")
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

		if !wallet.IsRegistered() && globals.Arguments["--offline"].(bool)==false {
			// The registration transaction to 'remote' requires some POW: First 3 bytes must be 0. 
			// The view only wallet doesn't generate its own registratio transaction
			if (globals.Arguments["--remote"]==true) || (wallet.ViewOnly()==true)  {			
				fmt.Fprintf(l.Stderr(), "Register your account in order to synchronise with the network\n");
			} else {
				reg_tx := wallet.GetRegistrationTX()
				fmt.Fprintf(l.Stderr(), "Registration TX : "+color_green+"%x"+color_white+"\n", reg_tx.Serialize())
			}
			
		}
		PressAnyKey(l, wallet)

	case "2": // give user his seed
	        if (wallet.ViewOnly() == false) {
			if !ValidateCurrentPassword(l, wallet) {
				logger.Error(fmt.Errorf("Invalid password"), "")
				PressAnyKey(l, wallet)
				break
			}
			display_seed(l, wallet) // seed should be given only to authenticated users
		} else {
			fmt.Printf("This is a view only wallet. It doens't contain the seed phrase\n")
		}
		PressAnyKey(l, wallet)

	case "3": // give user his keys in hex form

		if !ValidateCurrentPassword(l, wallet) {
			logger.Error(fmt.Errorf("Invalid password"), "")
			PressAnyKey(l, wallet)
			break
		}

		display_spend_key(l, wallet)
		PressAnyKey(l, wallet)

	case "4": // Registration
		// If the wallet is performing the 'offline' (sign) role, the transaction must be printed in a data format
		// which can be imported in the ViewOnly 'online' wallet.
		var IsOffline = globals.Arguments["--offline"].(bool)
		
		if (!wallet.IsRegistered()) || (IsOffline==true) {
			var reg_tx *transaction.Transaction
			
			if (wallet.ViewOnly()==true) {
				// Format: [0] - preamble: registration			
				//         [1] Address
				//         [2] Registration address
				//         [3] Hash of the registartion address
				//         [4] Checksum of the string
				
				//The view only online wallet received the registration transaction from the offline wallet.
				sRegistration := read_line_with_prompt(l, fmt.Sprintf("Enter the registration transaction (obtained from the offline (signing) wallet): "))
				
				if (len(sRegistration)==0) {
					//No input provided
					break;
				}
				
				//Strip off any newlines or extra spaces
				sTmp := strings.ReplaceAll(sRegistration,"\n","")
				sRegistration = strings.ReplaceAll(sTmp," ","");
				
				//Split string on ';'
				saParts := strings.Split(sRegistration,";")
				if (len(saParts) != 2) {
					fmt.Fprintf(l.Stderr(), "Invalid number of parts. Expected 2, found %d\n\n", len(saParts))
					break;
				}
				
				sTransaction := saParts[0]
				sProtocolChecksum := saParts[1]
				iProtocolChecksum,err := strconv.Atoi(sProtocolChecksum)
				if err!=nil {
					fmt.Fprintf(l.Stderr(), "Could not convert the checksum back to an integer\n\n")
					break
				}
								
				//Regenerate checksum:
                                var iCalculatedChecksum=0
                                for t := range sTransaction {
                                        iCalculatedChecksum = iCalculatedChecksum + (int)(sTransaction[t])
                                }
                                
                                // Check 1: Checksum
                                if (iProtocolChecksum != iCalculatedChecksum) {
                                	fmt.Fprintf(l.Stderr(), "Checksum calculation failed. Please check if you've imported the transaction correctly\n\n");
                                	break
				}				
				
				saParts = strings.Split(sTransaction,",")
				if (len(saParts) != 4) {
					fmt.Fprintf(l.Stderr(), "Invalid number of parts. Expected 4, found %d\n\n", len(saParts))
					break
				}
				
				if (saParts[0]!="registration") {
					fmt.Fprintf(l.Stderr(), "Input doesn't start with 'registration'\n\n")
					break;			
				}
				
				//Check 2: Is this transaction for our address?
				sAddress := wallet.GetAddress().String()
				if (sAddress != saParts[1]) {
					fmt.Fprintf(l.Stderr(), "Mismatch: Our address is %s, the registration transaction contains a different address:%s\n", sAddress, saParts[1])
					break
				}
				
				
				baRegistrationTx, err1 := hex.DecodeString(saParts[2])
				if err1 != nil {
				  fmt.Fprintf(l.Stderr(), "Could not convert the transaction back to binary data.\n\n")
				  break
				}
				baHash, err1 := hex.DecodeString(saParts[3])
				if err1 != nil {
				  fmt.Fprintf(l.Stderr(), "Could not convert the hash back to binary data.\n\n")
				  break
				}
				if (baHash[0]!=0 || baHash[1]!=0 || baHash[2]!=0) {
				  fmt.Fprintf(l.Stderr(), "For a valid registration transaction the first 3 bytes must be 0\n\n")
				  break
				}

				var tx2 transaction.Transaction
				tx2.Deserialize( baRegistrationTx )
				
				//var hash2 string
				calculated_hash := fmt.Sprintf("%s", tx2.GetHash())
				PublicKey2 := fmt.Sprintf("%x", tx2.MinerAddress )
				
				keys := wallet.Get_Keys()
				PublicKey  := fmt.Sprintf("%s", keys.Public.StringHex())
				
				// Check 3
				if (calculated_hash != saParts[3]) {
					fmt.Fprintf(l.Stderr(), "Mismatch: the calculated hash (of the registration transaction) and the hash provided in the input differs\n\n")
					break
				}
				
				// Check 4
				if (PublicKey != PublicKey2) {
					fmt.Fprintf(l.Stderr(), "Mismatch: the registration transaction is for public key: %s, but our public key is %s\n\n", PublicKey2, PublicKey)
					break
				}	
				
				//At this point the address & public key in the transaction matchs our online wallet values.
				reg_tx = & tx2
			} else {
				// The offline wallet generates the registration and provide the transaction text to be used 
				// by the online wallet to complete the registration.
				// The full function wallet (view+sign) generates the registration transaction and submits it
				// directy to the network to complete the registration
				if IsOffline==true {
					fmt.Fprintf(l.Stderr(), "Generating registration transaction for wallet address : "+color_green+"%s"+color_white+"\n", wallet.GetAddress())
				} else {
					fmt.Fprintf(l.Stderr(), "Wallet address : "+color_green+"%s"+color_white+" is going to be registered. Please wait till the account is registered. ", wallet.GetAddress())
				}
				fmt.Fprintf(l.Stderr(), "This is a pre-condition POW for using the online chain. ")
				fmt.Fprintf(l.Stderr(), "This will take a couple of minutes. A match is usually found between 2-5 million hashes. Please wait....\n")


				successful_regs := make(chan *transaction.Transaction)

				counter := 0
				counter2 := 0

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
								if ((counter2 % 100000) == 0) {
									//Match usually found round about 2 million mark
									fmt.Printf("Searched %d hashes\n",counter2)
									
									//FIxIT quick search
//									successful_regs <- lreg_tx
//									counter++
//									break;
								}
							}
						}
					}()
				}

				reg_tx = <-successful_regs
			}

			if (IsOffline==true) {
				// Offline wallet prints the prepared transaction, to be used in the online wallet
				fmt.Printf("Found the registration transaction. Import the complete text into the online (view only) wallet:\n");
				sTransaction := fmt.Sprintf("registration,%s,%x,%s",wallet.GetAddress().String(), reg_tx.Serialize(), reg_tx.GetHash())
				
				//Append a simple checksum to the string to detect copy/paste errors
				//during import into the online wallet:
				var iChecksum=0
				for t := range sTransaction {
					iChecksum = iChecksum + (int)(sTransaction[t])
				}
				
				fmt.Printf("%s;%d\n\n",sTransaction, iChecksum)
				
			} else {
				// View only online wallet & full feature wallet submits the transaction to the network
				fmt.Fprintf(l.Stderr(), "Registration TXID %s\n", reg_tx.GetHash())
				err := wallet.SendTransaction(reg_tx)
				if err != nil {
					fmt.Fprintf(l.Stderr(), "sending registration tx err %s\n\n", err)
				} else {
					fmt.Fprintf(l.Stderr(), "registration tx dispatched successfully\n\n")
				}
			}
		}

	case "6":
		if globals.Arguments["--offline"].(bool) {
			//Offline wallet can´t send tokens
			break;
		}
	
		if !valid_registration_or_display_error(l, wallet) {
			break
		}
		if !ValidateCurrentPassword(l, wallet) {
			logger.Error(fmt.Errorf("Invalid password"), "")
			break
		}

		a, err := ReadAddress(l, wallet)
		if err != nil {
			logger.Error(err, "error reading address")
			break
		}

		// Request SCID from integrated address or from input
		var scid crypto.Hash
		if a.Arguments != nil && a.Arguments.HasValue(rpc.RPC_ASSET, rpc.DataHash) {
			scid = a.Arguments.Value(rpc.RPC_ASSET, rpc.DataHash).(crypto.Hash)
			logger.Info("Address has a integrated SCID", "scid", scid)
		} else {
			scid, err = ReadSCID(l)
			if err != nil {
				logger.Error(err, "error reading SCID")
				break
			}
		}

		var amount_to_transfer uint64
		max_balance, _ := wallet.Get_Balance_scid(scid)
		max_str := fmt.Sprintf("%d", max_balance)
		if scid.IsZero() {
			max_str = globals.FormatMoney(max_balance)
		} // TODO else digits based on token standard

		amount_str := read_line_with_prompt(l, fmt.Sprintf("Enter token amount to transfer (max %s): ", max_str))
		// TODO digits based on token standard
		if amount_str == "" {
			amount_str = ".00001"
		}
		amount_to_transfer, err = globals.ParseAmount(amount_str)
		if err != nil {
			logger.Error(err, "Err parsing amount")
			break // invalid amount provided, bail out
		}

		if ConfirmYesNoDefaultNo(l, color_white+"Confirm Transaction (y/N)") {
			tx, err := wallet.TransferPayload0([]rpc.Transfer{{SCID: scid, Amount: amount_to_transfer, Destination: a.String()}}, 0, false, rpc.Arguments{}, 0, false) // empty SCDATA

			if err != nil {
				logger.Error(err, "Error while building Transaction")
				break
			}
			if err = wallet.SendTransaction(tx); err != nil {
				logger.Error(err, "Error while dispatching Transaction")
				break
			}
			logger.Info("Dispatched tx", "txid", tx.GetHash().String())
		}

	case "5":
		if globals.Arguments["--offline"].(bool) {
			// Offline wallet: Sign transaction
			if !ValidateCurrentPassword(l, wallet) {
				logger.Error(fmt.Errorf("Invalid password"), "")
				break
			}			

			_ = os.Remove("./signed")

			baData, err := os.ReadFile("./transaction")
			if err!=nil {
				fmt.Printf("Cant read from the transaction file: ./transaction\n\n");
				break;
			}
			fmt.Printf("Read %d bytes from ./transaction\n",len(baData))
			sTransaction := string(baData[:])
			
			_ = os.Remove("./transaction")
	    
			
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
			//            [12] Checksum of all the characters in the command.

                        //Split string on ';'
                        saParts := strings.Split(sTransaction,";")
                        if (len(saParts) != 2) {
                          fmt.Fprintf(l.Stderr(), "Invalid number of parts. Expected 2, found %d\n\n", len(saParts))
                          break;
                        }

                        sTransaction = saParts[0]
                        sProtocolChecksum := saParts[1]
                        iProtocolChecksum,err := strconv.Atoi(sProtocolChecksum)
                        if err!=nil {
                          fmt.Fprintf(l.Stderr(), "Could not convert the checksum back to an integer\n\n")
                          break
                        }
/*                                                                
                        //Regenerate checksum:
                        var iCalculatedChecksum=0x01
                        for t := range sTransaction {
                          iCalculatedChecksum = iCalculatedChecksum + (int)(sTransaction[t])
                        }
                                
			fmt.Printf("Checksum input\n'%s'\n\n",sTransaction);
                        // Check 1: Checksum
                        if (iProtocolChecksum != iCalculatedChecksum) {
                          fmt.Fprintf(l.Stderr(), "Checksum calculation failed. Please check if you've imported the transaction correctly. Protocol: %d, calculated: %d\n\n", iProtocolChecksum, iCalculatedChecksum)
                          break
                        }                               
*/                                
                        saParts = strings.Split(sTransaction," ")
			if (len(saParts)!=12) {
				fmt.Printf("Invalid number of parts in the transaction. Expected 13, found %d\n", len(saParts))
				break;
			}
			
			if  (saParts[0] != "dero") {
				fmt.Printf("Expected a Dero transaction, Found %s\n",saParts[1]);
				break;
			}

			if (saParts[1] != "1") {
				fmt.Printf("Only transaction version 1 supported. Found %s\n",saParts[2])
				break;
			}
                
			if (saParts[2] != "sign_offline") {
				fmt.Printf("Transaction doesn't start with 'sign_offline'\n")
				break;
			}
			
                	sChecksumInput:="dero 1 sign_offline ";
			//---------------------------------------------------------------------------------------------------------------------         
			var transfers []rpc.Transfer
			
			sTmp := strings.ReplaceAll(saParts[3],"'","")
			sTmp1:= strings.ReplaceAll(sTmp, "[","")                
			sTmp  = strings.ReplaceAll(sTmp1,"]","")

			saTransfers := strings.SplitAfter(sTmp, "}")
			//fmt.Printf("transfers:%d\n",len(saTransfers)-1);
			sChecksumInput+="transfers:";
			for t:=range saTransfers {
				if len(saTransfers[t])>0 {
					var transfer rpc.Transfer
				
					sTmp1 = strings.ReplaceAll(saTransfers[t], ",{","")             
					sTmp  = strings.ReplaceAll(sTmp1, "{","")                                               
					sTransfer := strings.ReplaceAll(sTmp,"}","")
					
					saParts := strings.Split(sTransfer,",")
					if (len(saParts)!=5) {
						fmt.Printf("Parse error. Invalid number of parts in transfers. Expected 5, found %d\n", len(saParts));
						break;
					}
					
					sTmp:=strings.ReplaceAll(saParts[0],"\"","")
					saItemValue:=strings.Split(sTmp,":")
					if (saItemValue[0]!="SCID") {
						fmt.Printf("Parse error. Could not find SCID in transfers\n");
						break;
					}
					err = transfer.SCID.UnmarshalText([]byte(string(saItemValue[1])))
					if err != nil {
						fmt.Printf("Parse error. Could not assign SCID to transfers\n");
						break;
					}
		                        sChecksumInput+=" \""+saItemValue[1]+"\""
					
					
					sTmp=strings.ReplaceAll(saParts[1],"\"","")
					saItemValue=strings.Split(sTmp,":")
					if (saItemValue[0]!="Destination") {
						fmt.Printf("Parse error. Could not find destination in transfers\n");
						break;
					}
					transfer.Destination = saItemValue[1]					
					sChecksumInput+=" \""+saItemValue[1]+"\""
					
					sTmp=strings.ReplaceAll(saParts[2],"\"","")
					saItemValue=strings.Split(sTmp,":")
					if (saItemValue[0]!="Amount") {
						fmt.Printf("Parse error. Could not find amount in transfers\n");
						break;
					}
					transfer.Amount, err = strconv.ParseUint(saItemValue[1],10,64)					
					sChecksumInput+=" "+saItemValue[1]
						
					sTmp=strings.ReplaceAll(saParts[3],"\"","")
					saItemValue=strings.Split(sTmp,":")
					if (saItemValue[0]!="Burn") {
						fmt.Printf("Parse error. Could not find burn in transfers\n");
						break;
					}
					transfer.Burn, err = strconv.ParseUint(saItemValue[1],10,64)					
					sChecksumInput+=" "+saItemValue[1]
					
					sTmp=strings.ReplaceAll(saParts[4],"\"","")
					saItemValue=strings.Split(sTmp,":")
					if (saItemValue[0]!="RPC") {
						fmt.Printf("Parse error. Could not find rpc in transfers\n");
						break;
					}
					hexRPC, err2 := hex.DecodeString(saItemValue[1])
					if err2 != nil {
						fmt.Printf("Parse error. Could not hex decode the rpc field of the transfers\n");
						break;
					} 
					err2 = transfer.Payload_RPC.UnmarshalBinary(hexRPC)
					if err2 != nil {
						fmt.Printf("Parse error. Could not assign the rpc field of the transfers\n");
						break;                          
					}
					sChecksumInput+=" \""+saItemValue[1]+"\""
					
					transfers = append(transfers, transfer)
				}
			}

			//---------------------------------------------------------------------------------------------------------------------         
			var rings_balances [][][]byte //initialize all maps

			sTmp  = strings.ReplaceAll(saParts[4],"'","")
			sTmp1 = strings.ReplaceAll(sTmp, "[","")                
			sTmp  = strings.ReplaceAll(sTmp1,"]","")

			saRingBalances := strings.SplitAfter(sTmp, "}")
			//fmt.Printf(":%d\n",len(saRingBalances)-1);              

			sChecksumInput+=" rings_balances:"
			for t:=range saRingBalances {
				if len(saRingBalances[t])>0 {
					var ring_balances  [][]byte
				
					sTmp1 = strings.ReplaceAll(saRingBalances[t], ",{","")          
					sTmp  = strings.ReplaceAll(sTmp1, "{","")
					sRingBalance := strings.ReplaceAll(sTmp,"}","")
					
					saRingBalances:=strings.Split(sRingBalance,",")
					if ( len(saRingBalances)<16) {
						fmt.Printf("Expected at least 16 ring balances. Found %d\n", len(saRingBalances))
						break;
					}
					
					for t := range saRingBalances {
					      baData,_ := hex.DecodeString( saRingBalances[t] )
					      ring_balances = append(ring_balances,baData)
					      
					      sChecksumInput+=" "+saRingBalances[t]
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
			sChecksumInput+=" rings:";

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
						fmt.Printf("Expected at least 16 rings. Found %d\n\n", len(saRing))
						break
					}
					
					for t := range saRing {
					    //fmt.Printf("Processing: '%s'\n", saRing[t])
					    baData,err := hex.DecodeString( saRing[t] )
					    if err!=nil {
						fmt.Printf("Could not decode ring entry: %s\n\n", saRing[t])
						break
					    }           
					    oG1 = new(bn256.G1);                                      
					    _,err = oG1.Unmarshal(baData);      
					    if err != nil { 
						fmt.Printf("Could not assign ring data\n\n");
						break
					    }; 
					    ring = append(ring,oG1)
					    
					    sChecksumInput+=" "+saRing[t]
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
	                sChecksumInput+=" "+saParts[6]
			
			//[7] height
			var height uint64
			height, err = strconv.ParseUint(saParts[7],10,64)
			if err!=nil {
				fmt.Printf("Parse error. Could not assign height to transfers\n\n");
			}
	                sChecksumInput+=" "+saParts[7]
			
			//[8] Array of scdata
			var scdata rpc.Arguments
			sTmp  = strings.ReplaceAll(saParts[8],"'","")
			sTmp1 = strings.ReplaceAll(sTmp, "[","")                
			sTmp  = strings.ReplaceAll(sTmp1,"]","")			
			if (len(sTmp) > 0) {
				hexSCData, err2 := hex.DecodeString(sTmp)
				if err2!=nil {
					fmt.Printf("Parse error. Could not decode the SCData\n\n");
					break;
				}                       
				if (len(hexSCData)>0) {
				      err = scdata.UnmarshalBinary(hexSCData)
				      if err!=nil {
					fmt.Printf("Parse error. Could not decode the SCData\n\n");
					break;
				      }
				}       
			}
	                sChecksumInput+=" "+saParts[8]
			
			//[9] treehash
			sTmp  = strings.ReplaceAll(saParts[9],"\"","")
			treehash_raw, err := hex.DecodeString(sTmp)
			if err != nil {
				fmt.Printf("Parse error. Could not decode treehash_raw\n\n")
				break;
			}
	                sChecksumInput+=" "+saParts[9]
			
			//[10] max_bits
			var max_bits int
			max_bits, err = strconv.Atoi(saParts[10])
	                sChecksumInput+=" "+saParts[10]
			
			//[11] gasstorage
			var gasstorage uint64
			gasstorage, err = strconv.ParseUint(saParts[11],10,64)
	                sChecksumInput+=" "+saParts[11]
			
	                iCalculatedChecksum:=0x01;
        	        for t := range sChecksumInput {
                	        iVal := int(sChecksumInput[t])
	                        iCalculatedChecksum = iCalculatedChecksum + iVal;
        	        }
        	        
			if (iCalculatedChecksum != iProtocolChecksum) {
				fmt.Printf("Checksum input\n'%s'\n\n",sChecksumInput)
				fmt.Printf("The checksum for the transaction data is invalid. Please provide the transaction again.\n\n")
				break
			}
			
			tx := wallet.BuildTransaction(transfers, rings_balances, rings, block_hash, height, scdata, treehash_raw, max_bits, gasstorage)
			if tx == nil {
				fmt.Printf("somehow the tx could not be built, please retry\n\n")
				break
			}
			
			sTxSerialized := tx.Serialize()
			sOutput := fmt.Sprintf("dero 1 signed %x",sTxSerialized)
			
			iCalculatedChecksum=0x01;
        	        for t := range sOutput {
                	        iVal := int(sOutput[t])
	                        iCalculatedChecksum = iCalculatedChecksum + iVal;
        	        }
        	        sChecksum := fmt.Sprintf("%d",iCalculatedChecksum)
        	        sOutput+=";"+sChecksum	
			
			baData = []byte( sOutput )
			err = os.WriteFile("./signed", baData, 0666)
			if err!=nil {
			    fmt.Printf("Error saving file to ./signed: %s\n",err)
			    break
			}			
			fmt.Printf("Saved signed transaction to ./signed. Return it to the online (view only) wallet to complete the transaction.\n\n")
			break;
		}


		
		if !valid_registration_or_display_error(l, wallet) {
			break
		}
		if !ValidateCurrentPassword(l, wallet) {
			logger.Error(fmt.Errorf("Invalid password"), "")
			break
		}

		// a , amount_to_transfer, err := collect_transfer_info(l,wallet)
		a, err := ReadAddress(l, wallet)
		if err != nil {
			logger.Error(err, "error reading address")
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
				logger.Error(err, "Integrated Address  arguments could not be validated.")
				break
			}

			if !a.Arguments.Has(rpc.RPC_DESTINATION_PORT, rpc.DataUint64) { // but only it is present
				logger.Error(fmt.Errorf("Integrated Address does not contain destination port."), "")
				break
			}

			arguments = append(arguments, rpc.Argument{Name: rpc.RPC_DESTINATION_PORT, DataType: rpc.DataUint64, Value: a.Arguments.Value(rpc.RPC_DESTINATION_PORT, rpc.DataUint64).(uint64)})
			// arguments = append(arguments, rpc.Argument{"Comment", rpc.DataString, "holygrail of all data is now working if you can see this"})

			if a.Arguments.Has(rpc.RPC_EXPIRY, rpc.DataTime) { // but only it is present

				if a.Arguments.Value(rpc.RPC_EXPIRY, rpc.DataTime).(time.Time).Before(time.Now().UTC()) {
					logger.Error(nil, "This address has expired.", "expiry time", a.Arguments.Value(rpc.RPC_EXPIRY, rpc.DataTime))
					break
				} else {
					logger.Info("This address will expire ", "expiry time", a.Arguments.Value(rpc.RPC_EXPIRY, rpc.DataTime))
				}
			}

			logger.Info("Destination port is integrated in address.", "dst port", a.Arguments.Value(rpc.RPC_DESTINATION_PORT, rpc.DataUint64).(uint64))

			if a.Arguments.Has(rpc.RPC_COMMENT, rpc.DataString) { // but only it is present
				logger.Info("Integrated Message", "comment", a.Arguments.Value(rpc.RPC_COMMENT, rpc.DataString))
				arguments = append(arguments, rpc.Argument{Name: rpc.RPC_COMMENT, DataType: rpc.DataString, Value: a.Arguments.Value(rpc.RPC_COMMENT, rpc.DataString)})
			}
		}

		// arguments have been already validated
		for _, arg := range a.Arguments {
			if !(arg.Name == rpc.RPC_COMMENT || arg.Name == rpc.RPC_EXPIRY || arg.Name == rpc.RPC_DESTINATION_PORT || arg.Name == rpc.RPC_SOURCE_PORT || arg.Name == rpc.RPC_VALUE_TRANSFER || arg.Name == rpc.RPC_NEEDS_REPLYBACK_ADDRESS) {
				switch arg.DataType {
				case rpc.DataString:
					if v, err := ReadString(l, arg.Name, arg.Value.(string)); err == nil {
						arguments = append(arguments, rpc.Argument{Name: arg.Name, DataType: arg.DataType, Value: v})
					} else {
						logger.Error(fmt.Errorf("%s could not be parsed (type %s),", arg.Name, arg.DataType), "")
						break
					}
				case rpc.DataInt64:
					if v, err := ReadInt64(l, arg.Name, arg.Value.(int64)); err == nil {
						arguments = append(arguments, rpc.Argument{Name: arg.Name, DataType: arg.DataType, Value: v})
					} else {
						logger.Error(fmt.Errorf("%s could not be parsed (type %s),", arg.Name, arg.DataType), "")
						break
					}
				case rpc.DataUint64:
					if v, err := ReadUint64(l, arg.Name, arg.Value.(uint64)); err == nil {
						arguments = append(arguments, rpc.Argument{Name: arg.Name, DataType: arg.DataType, Value: v})
					} else {
						logger.Error(fmt.Errorf("%s could not be parsed (type %s),", arg.Name, arg.DataType), "")
						break
					}
				case rpc.DataFloat64:
					if v, err := ReadFloat64(l, arg.Name, arg.Value.(float64)); err == nil {
						arguments = append(arguments, rpc.Argument{Name: arg.Name, DataType: arg.DataType, Value: v})
					} else {
						logger.Error(fmt.Errorf("%s could not be parsed (type %s),", arg.Name, arg.DataType), "")
						break
					}
				case rpc.DataTime:
					logger.Error(fmt.Errorf("time argument is currently not supported."), "")
					break

				}
			}
		}

		if a.Arguments.Has(rpc.RPC_VALUE_TRANSFER, rpc.DataUint64) { // but only it is present
			logger.Info("Transaction", "Value", globals.FormatMoney(a.Arguments.Value(rpc.RPC_VALUE_TRANSFER, rpc.DataUint64).(uint64)))
			amount_to_transfer = a.Arguments.Value(rpc.RPC_VALUE_TRANSFER, rpc.DataUint64).(uint64)
		} else {

			mbal, _ := wallet.Get_Balance()
			amount_str := read_line_with_prompt(l, fmt.Sprintf("Enter amount to transfer in DERO (current balance %s): ", globals.FormatMoney(mbal)))

			if amount_str == "" {
				logger.Error(nil, "Cannot transfer 0")
				break // invalid amount provided, bail out
			}
			amount_to_transfer, err = globals.ParseAmount(amount_str)
			if err != nil {
				logger.Error(err, "Err parsing amount")
				break // invalid amount provided, bail out
			}
		}

		// check whether the service needs the address of sender
		// this is required to enable services which are completely invisisble to external entities
		// external entities means anyone except sender/receiver
		if a.Arguments.Has(rpc.RPC_NEEDS_REPLYBACK_ADDRESS, rpc.DataUint64) {
			logger.Info("This RPC has requested your address.")
			logger.Info("If you are expecting something back, it needs to be sent")
			logger.Info("Your address will remain completely invisible to external entities(only sender/receiver can see your address)")
			arguments = append(arguments, rpc.Argument{Name: rpc.RPC_REPLYBACK_ADDRESS, DataType: rpc.DataAddress, Value: wallet.GetAddress()})
		}

		// if no arguments, use space by embedding a small comment
		if len(arguments) == 0 { // allow user to enter Comment
			if v, err := ReadUint64(l, "Please enter payment id (or destination port number)", uint64(0)); err == nil {
				arguments = append(arguments, rpc.Argument{Name: rpc.RPC_DESTINATION_PORT, DataType: rpc.DataUint64, Value: v})
			} else {
				logger.Error(err, fmt.Sprintf("%s could not be parsed (type %s),", "Number", rpc.DataUint64))
				break
			}

			if v, err := ReadString(l, "Comment", ""); err == nil {
				arguments = append(arguments, rpc.Argument{Name: rpc.RPC_COMMENT, DataType: rpc.DataString, Value: v})
			} else {
				logger.Error(fmt.Errorf("%s could not be parsed (type %s),", "Comment", rpc.DataString), "")
				break
			}
		}

		if _, err := arguments.CheckPack(transaction.PAYLOAD0_LIMIT); err != nil {
			logger.Error(err, "Arguments packing err")
			break
		}

		if ConfirmYesNoDefaultNo(l, color_white+"Confirm Transaction (y/N)") {

			//src_port := uint64(0xffffffffffffffff)

			tx, err := wallet.TransferPayload0([]rpc.Transfer{{Amount: amount_to_transfer, Destination: a.String(), Payload_RPC: arguments}}, 0, false, rpc.Arguments{}, 0, false) // empty SCDATA

			if err != nil {
				logger.Error(err, "Error while building Transaction")
				break
			}

			if err = wallet.SendTransaction(tx); err != nil {
				logger.Error(err, "Error while dispatching Transaction")
				break
			}
			logger.Info("Dispatched tx", "txid", tx.GetHash().String())
			//fmt.Printf("queued tx err %s\n")
		}

	case "12":
		if !valid_registration_or_display_error(l, wallet) {
			break
		}
		if !ValidateCurrentPassword(l, wallet) {
			logger.Error(fmt.Errorf("Invalid password"), "")
			break
		}

		logger.Error(err, "Not supported ")

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
				globals.Logger.Infof("Payment ID is integrated in address ID:%x", a.PaymentID)
			}

			if ConfirmYesNoDefaultNo(l, color_white+"Confirm Transaction to send entire balance (y/N)") {

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
				logger.Info("Wallet password successfully changed")
			} else {
				logger.Error(err, "Wallet password could not be changed")
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
		logger.Info("Prompt mode enabled, type \"menu\" command to start menu mode")

	case "0", "bye", "exit", "quit":
		wallet.Close_Encrypted_Wallet() // save the wallet
		prompt_mutex.Lock()
		wallet = nil
		globals.Exit_In_Progress = true
		prompt_mutex.Unlock()
		fmt.Fprintf(l.Stderr(), color_yellow+"Wallet closed"+color_white)
		fmt.Fprintf(l.Stderr(), color_yellow+"Exiting"+color_white)

	case "13":
		var zeroscid crypto.Hash
		show_transfers(l, wallet, zeroscid, 100)

	case "14":
		logger.Info("Rescanning wallet history")
		rescan_bc(wallet)
	case "15":
		if !ValidateCurrentPassword(l, wallet) {
			logger.Error(fmt.Errorf("Invalid password"), "")
			break
		}

		if _, err := os.Stat("./history"); errors.Is(err, os.ErrNotExist) {
			if err := os.Mkdir("./history", 0700); err != nil {
				logger.Error(err, "Error creating directory")
				break
			}
		}

		var zeroscid crypto.Hash
		account := wallet.GetAccount()
		for k, v := range account.EntriesNative {
			filename := filepath.Join("./history", k.String()+".json")
			if k == zeroscid {
				filename = filepath.Join("./history", "dero.json")
			}
			if data, err := json.Marshal(v); err != nil {
				logger.Error(err, "Error exporting data")
			} else if err = os.WriteFile(filename, data, 0600); err != nil {
				logger.Error(err, "Error exporting data")
			} else {
				logger.Info("successfully exported history", "file", filename)
			}
		}

	default:
		processed = false // just loop

	}
	return
}
