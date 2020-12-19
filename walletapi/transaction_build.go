package walletapi

//import "fmt"
import "math/big"
import mathrand "math/rand"
import "github.com/deroproject/derohe/globals"
import "github.com/deroproject/derohe/transaction"
import "github.com/deroproject/derohe/crypto"
import "github.com/deroproject/derohe/crypto/bn256"

// generate proof  etc
func BuildTransaction(sender *bn256.G1, sender_secret *big.Int, receiver *bn256.G1, sender_ebalance, receiver_ebalance *crypto.ElGamal, balance, value uint64, anonset_publickeys []*bn256.G1, anonset_ebalance []*crypto.ElGamal, fees uint64, height uint64, payment_id []byte, roothash []byte) *transaction.Transaction {

	var tx transaction.Transaction

	var publickeylist, C, CLn, CRn []*bn256.G1
	var D bn256.G1

	tx.Version = 1
	tx.Height = height
	tx.TransactionType = transaction.NORMAL

	crand := mathrand.New(globals.NewCryptoRandSource())

	var witness_index []int
	for i := 0; i < 2+len(anonset_publickeys); i++ { // todocheck whether this is power of 2 or not
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

	ebalances_list := make([]*crypto.ElGamal, 0, 2+len(anonset_publickeys))
	for i := range witness_index {
		switch i {
		case witness_index[0]:
			publickeylist = append(publickeylist, sender)
			//publickeylist = append(publickeylist, new(bn256.G1).ScalarMult(crypto.G, sender_secret))
			ebalances_list = append(ebalances_list, sender_ebalance)
		case witness_index[1]:
			publickeylist = append(publickeylist, receiver)
			ebalances_list = append(ebalances_list, receiver_ebalance)

		default:
			publickeylist = append(publickeylist, anonset_publickeys[0])
			anonset_publickeys = anonset_publickeys[1:]
			ebalances_list = append(ebalances_list, anonset_ebalance[0])
			anonset_ebalance = anonset_ebalance[1:]
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

	for i := range publickeylist { // setup commitments
		var x bn256.G1
		switch {
		case i == witness_index[0]:
			x.ScalarMult(crypto.G, new(big.Int).SetInt64(0-int64(value)-int64(fees))) // decrease senders balance
			//fmt.Printf("sender %s \n", x.String())
		case i == witness_index[1]:
			x.ScalarMult(crypto.G, new(big.Int).SetInt64(int64(value))) // increase receiver's balance
			//fmt.Printf("receiver %s \n", x.String())

			// lets encrypt the payment id, it's simple, we XOR the paymentID
			blinder := new(bn256.G1).ScalarMult(publickeylist[i], r)

			output := crypto.EncryptDecryptPaymentID(blinder, payment_id[:])
			copy(tx.PaymentID[:], output[:])

		default:
			x.ScalarMult(crypto.G, new(big.Int).SetInt64(0))
		}

		x.Add(new(bn256.G1).Set(&x), new(bn256.G1).ScalarMult(publickeylist[i], r)) // hide all commitments behind r
		C = append(C, &x)
	}
	D.ScalarMult(crypto.G, r)

	for i := range publickeylist {

		var ebalance *crypto.ElGamal

		switch {
		case i == witness_index[0]:
			ebalance = sender_ebalance
		case i == witness_index[1]:
			ebalance = receiver_ebalance
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

	// time for bullets-sigma
	statement := GenerateStatement(CLn, CRn, publickeylist, C, &D, fees) // generate statement
	copy(statement.Roothash[:], roothash[:])

	witness := GenerateWitness(sender_secret, r, value, balance-value-fees, witness_index)

	// this goes to proof.u
	u := new(bn256.G1).ScalarMult(crypto.HashToPoint(crypto.HashtoNumber(append([]byte(crypto.PROTOCOL_CONSTANT), statement.Roothash[:]...))), sender_secret) // this should be moved to generate proof
	//Print(statement, witness)
	tx.Statement = statement
	tx.Proof = crypto.GenerateProof(&statement, &witness, u, tx.GetHash())

	// after the tx is serialized, it loses information which is then fed by blockchain
	if tx.Proof.Verify(&tx.Statement, tx.GetHash()) {
		//fmt.Printf("TX verified with proof successfuly value %d\n", value)
	} else {

		//fmt.Printf("TX verification failed !!!!!!!!!!\n")
		panic("TX verification failed !!!!!!!!!!")
	}

	/*
	   serialized := tx.Serialize()
	   //fmt.Printf("serialized  kength %d \n", len(serialized)*2)


	   var dtx transaction.Transaction
	   //fmt.Printf("err deserialing %s\n", dtx.DeserializeHeader(serialized))


	   serialized = dtx.Serialize()
	   //fmt.Printf("dtx2 serialized  kength %d\n",  len(serialized)*2)

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
