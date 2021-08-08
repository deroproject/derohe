package walletapi

import "fmt"
import "math/big"
import mathrand "math/rand"
import "github.com/deroproject/derohe/rpc"
import "github.com/deroproject/derohe/globals"
import "github.com/deroproject/derohe/config"
import "github.com/deroproject/derohe/transaction"
import "github.com/deroproject/derohe/cryptography/crypto"
import "github.com/deroproject/derohe/cryptography/bn256"

// generate proof  etc
// we use a previous point in history and cryptographically prove that we have not used the funds till now
func (w *Wallet_Memory) BuildTransaction(transfers []rpc.Transfer, emap map[string]map[string][]byte, rings [][]*bn256.G1, height uint64, scdata rpc.Arguments, roothash []byte, max_bits_array []int) *transaction.Transaction {

	var tx transaction.Transaction

	sender := w.account.Keys.Public.G1()
	sender_secret := w.account.Keys.Secret.BigInt()

	tx.Version = 1
	tx.Height = height
	tx.TransactionType = transaction.NORMAL


	if height % config.BLOCK_BATCH_SIZE != 0 {
		panic(fmt.Sprintf("Height must be a multiple of %d (config.BLOCK_BATCH_SIZE)", config.BLOCK_BATCH_SIZE))
	}
	/*
		if burn_value >= 1 {
			tx.TransactionType = transaction.BURN_TX
			tx.Value = burn_value
		}
	*/
	if len(scdata) >= 1 {
		tx.TransactionType = transaction.SC_TX
		tx.SCDATA = scdata

		reg_tx := w.GetRegistrationTX()

		tx.MinerAddress = reg_tx.MinerAddress
		tx.S = reg_tx.S
		tx.C = reg_tx.C

	}

	crand := mathrand.New(globals.NewCryptoRandSource())

	var witness_list []crypto.Witness

	for t, _ := range transfers {

		var publickeylist, C, CLn, CRn []*bn256.G1
		var D bn256.G1

		receiver_addr, _ := rpc.NewAddress(transfers[t].Destination)
		receiver := receiver_addr.PublicKey.G1()

		var witness_index []int
		for i := 0; i < len(rings[t]); i++ { // todocheck whether this is power of 2 or not
			witness_index = append(witness_index, i)
		}
		max_bits := max_bits_array[t]
		for ; max_bits%8 != 0; max_bits++ { // round to next higher byte size
		}

		//witness_index[3], witness_index[1] = witness_index[1], witness_index[3]
		for {
			crand.Shuffle(len(witness_index), func(i, j int) {
				witness_index[i], witness_index[j] = witness_index[j], witness_index[i]
			})

			// make sure sender and receiver are not both odd or both even
			// sender will always be at  witness_index[0] and receiver will always be at witness_index[1]
			if witness_index[0]%2 != witness_index[1]%2 {
				break
			}
		}

		// Lots of ToDo for this, enables satisfying lots of  other things
		anonset_publickeys := rings[t][2:]
		ebalances_list := make([]*crypto.ElGamal, 0, len(rings[t]))
		for i := range witness_index {
			switch i {
			case witness_index[0]:
				publickeylist = append(publickeylist, sender)
				ebalances_list = append(ebalances_list, new(crypto.ElGamal).Deserialize(emap[string(transfers[t].SCID.String())][sender.String()]))
			case witness_index[1]:
				publickeylist = append(publickeylist, receiver)
				ebalances_list = append(ebalances_list, new(crypto.ElGamal).Deserialize(emap[string(transfers[t].SCID.String())][receiver.String()]))
			default:
				publickeylist = append(publickeylist, anonset_publickeys[0])
				ebalances_list = append(ebalances_list, new(crypto.ElGamal).Deserialize(emap[string(transfers[t].SCID.String())][anonset_publickeys[0].String()]))
				anonset_publickeys = anonset_publickeys[1:]
			}

			// fmt.Printf("adding %d %s  (ring count %d) \n", i,publickeylist[i].String(), len(anonset_publickeys))
		}

		//  fmt.Printf("len of publickeylist  %d \n", len(publickeylist))

		//  revealing r will disclose the amount and the sender and receiver and separate anonymous ring members
		// calculate r deterministically, so its different every transaction, in emergency it can be given to other, and still will not allows key attacks
		rinputs := append([]byte{}, roothash[:]...)
		for i := range publickeylist {
			rinputs = append(rinputs, publickeylist[i].EncodeCompressed()...)
		}
		rencrypted := new(bn256.G1).ScalarMult(crypto.HashToPoint(crypto.HashtoNumber(append([]byte(crypto.PROTOCOL_CONSTANT), rinputs...))), sender_secret)
		r := crypto.ReducedHash(rencrypted.EncodeCompressed())

		//r := crypto.RandomScalarFixed()

		//fmt.Printf("r %s\n", r.Text(16))

		var asset transaction.AssetPayload

		asset.SCID = transfers[t].SCID
		asset.BurnValue = transfers[t].Burn

		fees := uint64(0)
		value := transfers[t].Amount
		burn_value := transfers[t].Burn
		if asset.SCID.IsZero() && (value+burn_value) == 0 {
			fees = 1
		}
		for i := range publickeylist { // setup commitments
			var x bn256.G1
			switch {
			case i == witness_index[0]:
				x.ScalarMult(crypto.G, new(big.Int).SetInt64(0-int64(value)-int64(fees)-int64(burn_value))) // decrease senders balance
				//fmt.Printf("sender %s \n", x.String())
			case i == witness_index[1]:
				x.ScalarMult(crypto.G, new(big.Int).SetInt64(int64(value))) // increase receiver's balance
				//fmt.Printf("receiver %s \n", x.String())

				// lets encrypt the payment id, it's simple, we XOR the paymentID
				blinder := new(bn256.G1).ScalarMult(publickeylist[i], r)

				// we must obfuscate it for non-client call
				if len(publickeylist) >= 512 {
					panic("currently we donot support ring size >= 512")
				}

				asset.RPCType = transaction.ENCRYPTED_DEFAULT_PAYLOAD_CBOR

				data, _ := transfers[t].Payload_RPC.CheckPack(transaction.PAYLOAD0_LIMIT)

				asset.RPCPayload = append([]byte{byte(uint(witness_index[0]))}, data...)

				//fmt.Printf("%d packed rpc payload %d %x\n ", t, len(data), data)
				// make sure used data encryption is optional, just in case we would like to play together with ring members
				crypto.EncryptDecryptUserData(blinder, asset.RPCPayload)

			default:
				x.ScalarMult(crypto.G, new(big.Int).SetInt64(0))
			}

			x.Add(new(bn256.G1).Set(&x), new(bn256.G1).ScalarMult(publickeylist[i], r)) // hide all commitments behind r
			C = append(C, &x)
		}
		D.ScalarMult(crypto.G, r)

		//fmt.Printf("t %d publickeylist %d\n", t, len(publickeylist))
		for i := range publickeylist {
			var ebalance *crypto.ElGamal

			switch {
			case i == witness_index[0]:
				ebalance = new(crypto.ElGamal).Deserialize(emap[string(transfers[t].SCID.String())][sender.String()])
			case i == witness_index[1]:
				ebalance = new(crypto.ElGamal).Deserialize(emap[string(transfers[t].SCID.String())][receiver.String()])
				//fmt.Printf("receiver %s \n", x.String())
			default:
				//x.ScalarMult(crypto.G, new(big.Int).SetInt64(0))
				// panic("anon ring currently not supported")
				ebalance = ebalances_list[i]
			}

			var ll, rr bn256.G1
			//ebalance := b.balances[publickeylist[i].String()] // note these are taken from the chain live

			ll.Add(ebalance.Left, C[i])
			CLn = append(CLn, &ll)
			//  fmt.Printf("%d CLnG %x\n", i,CLn[i].EncodeCompressed())

			rr.Add(ebalance.Right, &D)
			CRn = append(CRn, &rr)
			//  fmt.Printf("%d CRnG %x\n",i, CRn[i].EncodeCompressed())

		}

		// decode balance now
		balance := w.DecodeEncryptedBalanceNow(new(crypto.ElGamal).Deserialize(emap[string(transfers[t].SCID.String())][sender.String()]))

		//fmt.Printf("t %d scid %s  balance %d\n", t, transfers[t].SCID, balance)

		// time for bullets-sigma
		statement := GenerateStatement(CLn, CRn, publickeylist, C, &D, fees) // generate statement
		copy(statement.Roothash[:], roothash[:])
		statement.Bytes_per_publickey = byte(max_bits / 8)

		witness := GenerateWitness(sender_secret, r, value, balance-value-fees-burn_value, witness_index)

		witness_list = append(witness_list, witness)

		// this goes to proof.u

		//Print(statement, witness)
		asset.Statement = statement

		tx.Payloads = append(tx.Payloads, asset)

		// get ready for another round by internal processing of state
		for i := range publickeylist {
			balance := new(crypto.ElGamal).Deserialize(emap[string(transfers[t].SCID.String())][publickeylist[i].String()])
			echanges := crypto.ConstructElGamal(statement.C[i], statement.D)

			balance = balance.Add(echanges)                                                           // homomorphic addition of changes
			emap[string(transfers[t].SCID.String())][publickeylist[i].String()] = balance.Serialize() // reserialize and store
		}

	}



	u := new(bn256.G1).ScalarMult(crypto.HeightToPoint(height), sender_secret) // this should be moved to generate proof
	u1 := new(bn256.G1).ScalarMult(crypto.HeightToPoint(height + config.BLOCK_BATCH_SIZE), sender_secret) // this should be moved to generate proof
	for t := range transfers {
		tx.Payloads[t].Proof = crypto.GenerateProof(&tx.Payloads[t].Statement, &witness_list[t], u,u1, height, tx.GetHash(), tx.Payloads[t].BurnValue)
	}

	// after the tx is serialized, it loses information which is then fed by blockchain

	//fmt.Printf("txhash before %s\n", tx.GetHash())

	for t := range tx.Payloads {
		if tx.Payloads[t].Proof.Verify(&tx.Payloads[t].Statement, tx.GetHash(), height, tx.Payloads[t].BurnValue) {
			fmt.Printf("TX verified with proof successfuly %s  burn_value %d\n", tx.GetHash(), tx.Payloads[t].BurnValue)

			//fmt.Printf("Statement %+v\n", tx.Payloads[t].Statement)
			//fmt.Printf("Proof %+v\n", tx.Payloads[t].Proof)

		} else {

			//fmt.Printf("TX verification failed !!!!!!!!!!\n")
			fmt.Printf("TX verification failed, did u try sending more than you have !!!!!!!!!!\n")
		}
	}

	/*
		serialized := tx.Serialize()
		fmt.Printf("serialized  kength %d  \n", len(serialized)*2)

		var dtx transaction.Transaction
		fmt.Printf("err deserialing %s\n", dtx.DeserializeHeader(serialized))

		serialized = dtx.Serialize()
		fmt.Printf("dtx2 serialized  kength %d\n", len(serialized)*2)

		fmt.Printf("txhash after %s scdata  %+v\n", dtx.GetHash(), tx.SCDATA)
	*/
	return &tx
}

// generate statement
func GenerateStatement(CLn, CRn, publickeylist, C []*bn256.G1, D *bn256.G1, fees uint64) crypto.Statement {
	return crypto.Statement{CLn: CLn, CRn: CRn, Publickeylist: publickeylist, C: C, D: D, Fees: fees}
}

// generate witness
func GenerateWitness(secretkey, r *big.Int, TransferAmount, Balance uint64, index []int) crypto.Witness {
	return crypto.Witness{SecretKey: secretkey, R: r, TransferAmount: TransferAmount, Balance: Balance, Index: index}
}
