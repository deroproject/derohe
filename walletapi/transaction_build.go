package walletapi

import (
	"fmt"
	"math"
	"math/big"
	"strconv"

	//import "encoding/binary"
	mathrand "math/rand"

	"github.com/deroproject/derohe/config"
	"github.com/deroproject/derohe/cryptography/bn256"
	"github.com/deroproject/derohe/cryptography/crypto"
	"github.com/deroproject/derohe/globals"
	"github.com/deroproject/derohe/rpc"
	"github.com/deroproject/derohe/transaction"
)

// this is run some tests and benchmarks
type GenerateProofFunc func(scid crypto.Hash, scid_index int, s *crypto.Statement, witness *crypto.Witness, u *bn256.G1, txid crypto.Hash, burn_value uint64) *crypto.Proof

var GenerateProoffuncptr GenerateProofFunc = crypto.GenerateProof

// generate proof  etc
func (w *Wallet_Memory) BuildTransaction(transfers []rpc.Transfer, emap [][][]byte, rings [][]*bn256.G1, block_hash crypto.Hash, height uint64, scdata rpc.Arguments, roothash []byte, max_bits int, fees uint64) *transaction.Transaction {

	sender := w.account.Keys.Public.G1()
	sender_secret := w.account.Keys.Secret.BigInt()

	var retry_count int

rebuild_tx:

	var tx transaction.Transaction

	tx.Version = 1
	tx.Height = height
	tx.BLID = block_hash
	tx.TransactionType = transaction.NORMAL

	if len(scdata) >= 1 {
		tx.TransactionType = transaction.SC_TX
		tx.SCDATA = scdata
	}

	crand := mathrand.New(globals.NewCryptoRandSource())

	var witness_list []crypto.Witness

	umap := map[string][]byte{}

	fees_done := false

	if retry_count%len(rings[0]) == 0 {
		max_bits += 3
	}

	if max_bits >= 240 {
		panic("currently we cannot use more than 240 bits")
	}

	for ; max_bits%8 != 0; max_bits++ { // round to next higher byte size
	}

	for t := range transfers {

		var publickeylist, C, CLn, CRn []*bn256.G1
		var D bn256.G1

		receiver_addr, _ := rpc.NewAddress(transfers[t].Destination)
		receiver := receiver_addr.PublicKey.G1()

		var witness_index []int
		for i := 0; i < len(rings[t]); i++ { // todocheck whether this is power of 2 or not
			witness_index = append(witness_index, i)
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
		anonset_balances := emap[t][2:]
		ebalances_list := make([]*crypto.ElGamal, 0, len(rings[t]))
		for i := range witness_index {
			switch i {
			case witness_index[0]:
				publickeylist = append(publickeylist, sender)
				ebalances_list = append(ebalances_list, new(crypto.ElGamal).Deserialize(emap[t][0]))
			case witness_index[1]:
				publickeylist = append(publickeylist, receiver)
				ebalances_list = append(ebalances_list, new(crypto.ElGamal).Deserialize(emap[t][1]))
			default:
				publickeylist = append(publickeylist, anonset_publickeys[0])
				ebalances_list = append(ebalances_list, new(crypto.ElGamal).Deserialize(anonset_balances[0]))
				anonset_publickeys = anonset_publickeys[1:]
				anonset_balances = anonset_balances[1:]
			}

			// fmt.Printf("adding %d %s  (ring count %d) \n", i,publickeylist[i].String(), len(anonset_publickeys))
		}

		// use updated balances if possible
		for i := range publickeylist {
			if bal, ok := umap[transfers[t].SCID.String()+publickeylist[i].String()]; ok {
				ebalances_list[i] = new(crypto.ElGamal).Deserialize(bal)
			}
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

		//fmt.Printf("t %d building transfers %+v\n", t, transfers[t])
		//fmt.Printf("building t %d r  calculated %s\n", t, r.Text(16))

		var asset transaction.AssetPayload

		asset.SCID = transfers[t].SCID
		asset.BurnValue = transfers[t].Burn

		value := transfers[t].Amount
		burn_value := transfers[t].Burn

		// If user provide fees, we will use it, otherwise we will calculate it
		// Fees are only paid on DERO transfers, not on tokens transfers
		should_do_fees := asset.SCID.IsZero() && !fees_done && fees != 0

		if fees == 0 && asset.SCID.IsZero() && !fees_done {
			total_bytes := 0
			// Transaction Prefix
			{
				// Those following are Uvarint, which means they are variable size
				// But we put full max size to support any future update
				total_bytes += 8 // Version
				total_bytes += 8 // Source Network
				total_bytes += 8 // Destination Network
				total_bytes += 8 // Transaction Type
				if tx.TransactionType == transaction.PREMINE || tx.TransactionType == transaction.SC_TX {
					total_bytes += 8 // Burn Value
				}

				if tx.TransactionType == transaction.PREMINE || tx.TransactionType == transaction.COINBASE || tx.TransactionType == transaction.REGISTRATION {
					total_bytes += 33 // Miner Address
				}

				if tx.TransactionType == transaction.REGISTRATION {
					total_bytes += 2 * 33 // C, S (fixed size)
				}

				if tx.TransactionType == transaction.BURN_TX || tx.TransactionType == transaction.NORMAL || tx.TransactionType == transaction.SC_TX {
					total_bytes += 8  // Height
					total_bytes += 32 // BLID
					// This one is a varint too (but we put max size)
					total_bytes += len(transfers) * 8 // Number of Assets (uint64)
				}
			}

			for t := range transfers {
				rings_count := len(rings[t])

				// Common Payload
				{
					total_bytes += 1  // RPC Type
					total_bytes += 32 // SCID

					// the payload max size
					total_bytes += transaction.PAYLOAD_LIMIT
				}

				// Statement
				statement_size := 0
				{
					// Publickeylist pointers total size
					rings_size := rings_count * (max_bits / 8)
					statement_size += rings_size

					// 1 byte for power, 1 byte for bytes per public key
					statement_size += 2

					// 8 bytes for fees (varint but max value in case)
					statement_size += 8

					// 33 bytes for D
					statement_size += 33

					// 33 bytes per C field (bn256.G1 compressed format)
					statement_size += 33 * rings_count

					// 32 bytes for roothash
					statement_size += 32
				}
				total_bytes += statement_size

				// Proof
				proof_size := 0
				{
					// BA, BS, A, B, u, T_1, T_2
					proof_size += 7 * 33

					// dynamic sizes
					m := int(math.Log2(float64(rings_count)))
					// CLnG, CRnG, C_0G, DG, y_0G, gG, C_XG, y_XG
					proof_size += 8 * m * 33

					// FieldVector in Proof must be double of CLnG len
					// which is m
					// each element in vector is 32 bytes
					proof_size += m * 2 * 32

					// z_A, that, mu, c, s_sk, s_r, s_b, s_tau
					proof_size += 8 * 32

					// InnerProduct
					{
						// a, b (32 bytes each)
						proof_size += 2 * 32
						// bulletproofs are 128 bits, so its 7 elements in ls, rs
						proof_size += 7 * 2 * 33
					}
				}
				total_bytes += proof_size
			}

			// Add SC Data
			if tx.TransactionType == transaction.SC_TX {
				if data, err := scdata.MarshalBinary(); err != nil {
					panic(err)
				} else {
					total_bytes += len(data) + 8 // 8 is varint max value size
				}
			}

			// multiply total_bytes by DERO fees per KB with fee multiplier
			{
				size_in_kb := total_bytes / 1024

				if (total_bytes % 1024) != 0 { // for any part there of, use a full KB fee
					size_in_kb += 1
				}

				fees = uint64(float64(size_in_kb) * (float64(config.FEE_PER_KB) * float64(w.GetFeeMultiplier())))
			}

			should_do_fees = true
			fees_done = true
		}

		fees_in_statement := false
		for i := range publickeylist { // setup commitments
			var x bn256.G1
			switch {
			case i == witness_index[0]:
				if asset.SCID.IsZero() && should_do_fees {
					x.ScalarMult(crypto.G, new(big.Int).SetInt64(0-int64(value)-int64(fees)-int64(burn_value))) // decrease senders balance (with fees)
					should_do_fees = false
					fees_in_statement = true
				} else {
					x.ScalarMult(crypto.G, new(big.Int).SetInt64(0-int64(value)-int64(burn_value))) // decrease senders balance
				}
				//fmt.Printf("sender %d %s \n", i, sender.String())

			case i == witness_index[1]:
				x.ScalarMult(crypto.G, new(big.Int).SetInt64(int64(value))) // increase receiver's balance
				//fmt.Printf("receiver %d %s \n",i, receiver.String())

				// lets encrypt the payment id, it's simple, we XOR the paymentID
				//blinder := new(bn256.G1).ScalarMult(publickeylist[i], r)

				// we must obfuscate it for non-client call, actual limit is 128, but for future testing/support
				if len(publickeylist) >= 512 {
					panic("currently we donot support ring size >= 512")
				}

				asset.RPCType = transaction.ENCRYPTED_DEFAULT_PAYLOAD_CBOR

				data, _ := transfers[t].Payload_RPC.CheckPack(transaction.PAYLOAD0_LIMIT)

				shared_key := crypto.GenerateSharedSecret(r, publickeylist[i])

				asset.RPCPayload = append([]byte{byte(uint(witness_index[1]))}, data...)
				//fmt.Printf("buulding shared_key %x  index of receiver %d\n",shared_key,i)
				//fmt.Printf("building plaintext payload %x\n",asset.RPCPayload)

				//fmt.Printf("%d packed rpc payload %d %x\n ", t, len(data), data)
				// make sure used data encryption is optional, just in case we would like to play together with ring members
				// we intoduce an element to create dependency of input key, so receiver cannot prove otherwise
				crypto.EncryptDecryptUserData(crypto.Keccak256(shared_key[:], publickeylist[i].EncodeCompressed()), asset.RPCPayload)

				//fmt.Printf("building encrypted payload %x\n",asset.RPCPayload)

			default:
				x.ScalarMult(crypto.G, new(big.Int).SetInt64(0))
			}

			x.Add(new(bn256.G1).Set(&x), new(bn256.G1).ScalarMult(publickeylist[i], r)) // hide all commitments behind r
			C = append(C, &x)
		}
		D.ScalarMult(crypto.G, r)

		for i := range publickeylist {
			ebalance := ebalances_list[i]

			var ll, rr bn256.G1
			//ebalance := b.balances[publickeylist[i].String()] // note these are taken from the chain live

			ll.Add(ebalance.Left, C[i])
			CLn = append(CLn, &ll)
			//  fmt.Printf("%d CLnG %x\n", i,CLn[i].EncodeCompressed())

			rr.Add(ebalance.Right, &D)
			CRn = append(CRn, &rr)
			//  fmt.Printf("%d CRnG %x\n",i, CRn[i].EncodeCompressed())

		}

		// decode sender (our) balance now, it might have been updated
		balance := w.DecodeEncryptedBalanceNow(ebalances_list[witness_index[0]])

		//fmt.Printf("t %d scid %s  balance %d\n", t, transfers[t].SCID, balance)

		// time for bullets-sigma
		fees_currentasset := uint64(0)
		if asset.SCID.IsZero() && fees_in_statement {
			fees_currentasset = fees
		}
		statement := GenerateStatement(CLn, CRn, publickeylist, C, &D, fees_currentasset) // generate statement

		copy(statement.Roothash[:], roothash[:])
		statement.Bytes_per_publickey = byte(max_bits / 8)

		witness := GenerateWitness(sender_secret, r, value, balance-value-fees_currentasset-burn_value, witness_index)

		witness_list = append(witness_list, witness)

		// this goes to proof.u

		//Print(statement, witness)
		asset.Statement = statement

		tx.Payloads = append(tx.Payloads, asset)

		// get ready for another round by internal processing of state

		for i := range publickeylist {
			balance := ebalances_list[i]
			echanges := crypto.ConstructElGamal(statement.C[i], statement.D)
			balance = balance.Add(echanges)                                                  // homomorphic addition of changes
			umap[transfers[t].SCID.String()+publickeylist[i].String()] = balance.Serialize() // reserialize and store
		}

	}

	scid_map := map[crypto.Hash]int{}
	for t := range transfers {
		// the u is dependent on roothash,SCID and counter ( counter is dynamic and depends on order of assets)

		uinput := append([]byte(crypto.PROTOCOL_CONSTANT), tx.Payloads[t].Statement.Roothash[:]...)
		scid_index := scid_map[tx.Payloads[t].SCID]
		scid_index_str := strconv.Itoa(scid_index)
		uinput = append(uinput, tx.Payloads[t].SCID[:]...)
		uinput = append(uinput, scid_index_str...)

		u := new(bn256.G1).ScalarMult(crypto.HashToPoint(crypto.HashtoNumber(uinput)), sender_secret) // this should be moved to generate proof

		scid_map[tx.Payloads[t].SCID] = scid_map[tx.Payloads[t].SCID] + 1

		//tx.Payloads[t].Proof = crypto.GenerateProof(&tx.Payloads[t].Statement, &witness_list[t], u, tx.GetHash(), tx.Payloads[t].BurnValue)

		tx.Payloads[t].Proof = GenerateProoffuncptr(tx.Payloads[t].SCID, scid_index, &tx.Payloads[t].Statement, &witness_list[t], u, tx.GetHash(), tx.Payloads[t].BurnValue)
	}

	// after the tx is serialized, it loses information which is then fed by blockchain

	scid_map_t := map[crypto.Hash]int{}
	for t := range tx.Payloads {
		if tx.Payloads[t].Proof.Verify(tx.Payloads[t].SCID, scid_map_t[tx.Payloads[t].SCID], &tx.Payloads[t].Statement, tx.GetHash(), tx.Payloads[t].BurnValue) {
			//fmt.Printf("TX verified with proof successfuly %s  burn_value %d\n", tx.GetHash(), tx.Payloads[t].BurnValue)
		} else {
			fmt.Printf("TX verification failed, did u try sending more than you have !!!!!!!!!!\n")
			return nil
		}
		scid_map_t[tx.Payloads[t].SCID] = scid_map_t[tx.Payloads[t].SCID] + 1
	}

	if tx.TransactionType == transaction.SC_TX {
		if tx.SCDATA.Has(rpc.SCACTION, rpc.DataUint64) {
			if rpc.SC_INSTALL == rpc.SC_ACTION(tx.SCDATA.Value(rpc.SCACTION, rpc.DataUint64).(uint64)) {
				txid := tx.GetHash()
				if txid[0] < 0x80 || txid[31] < 0x80 { // last byte should be more than 0x80
					if retry_count <= 20 {
						//fmt.Printf("rebuilding tx %s retry_count %d\n", txid, retry_count)
						goto rebuild_tx
					}
				}
			}
		}
	}

	// these 2 steps are only necessary, since blockchain doesn't accept unserialized txs
	//var dtx transaction.Transaction
	//_ = dtx.DeserializeHeader(tx.Serialize())

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
