package p2p

// this file implements incoming chunk processor
import "fmt"

import "time"
import "sync"
import "bytes"

import "github.com/fxamacker/cbor/v2"

import "github.com/klauspost/reedsolomon"
import "github.com/deroproject/derohe/block"
import "github.com/deroproject/derohe/transaction"
import "github.com/deroproject/derohe/globals"
import "github.com/deroproject/derohe/errormsg"
import "github.com/deroproject/derohe/config"
import "github.com/deroproject/derohe/metrics"
import "github.com/deroproject/derohe/cryptography/crypto"

var chunk_map sync.Map // key is blid, value is pointer to  Chunks_Per_Block_Data

var chunk_lock sync.Mutex
var single_construction sync.Mutex // used to single threaded processing while reconstructing blocks

const MAX_CHUNKS uint8 = 255

type Chunks_Per_Block_Data struct {
	ChunkCollection [MAX_CHUNKS]*Block_Chunk // nil means we donot have the chunk
	Created         time.Time                // when was this structure allocated
	Processed       bool                     // whether processing has completed successfully
	Complete        bool                     // whether everything is complete
	Sent            int64                    // at what time, the original send sent it
	bl              *block.Block             // this is used internally
	sync.Mutex
}

// cleans up chunks every minute
func chunks_clean_up() {
	chunk_map.Range(func(key, value interface{}) bool {
		chunks_per_block := value.(*Chunks_Per_Block_Data)
		if time.Now().Sub(chunks_per_block.Created) > time.Second*180 {
			chunk_map.Delete(key)
		}
		return true
	})
}

// return whether chunk exist
func is_chunk_exist(hhash [32]byte, cid uint8) *Block_Chunk {
	chunksi, ok := chunk_map.Load(fmt.Sprintf("%x", hhash))
	if !ok {
		//debug.PrintStack()
		return nil
	}
	chunks_per_block := chunksi.(*Chunks_Per_Block_Data)
	return chunks_per_block.ChunkCollection[cid]
}

// feed a chunk until we are able to fully decode a chunk
func (connection *Connection) feed_chunk(chunk *Block_Chunk, sent int64) error {

	chunk_lock.Lock()
	defer chunk_lock.Unlock()

	if chunk.HHash != chunk.HeaderHash() {
		connection.logger.V(2).Info("This peer should be banned, since he supplied wrong chunk")
		connection.exit()
		return fmt.Errorf("Corrupted Chunk")
	}

	if chunk.CHUNK_COUNT > uint(MAX_CHUNKS) || chunk.CHUNK_NEED > chunk.CHUNK_COUNT {
		return fmt.Errorf("Invalid Chunk Count")
	}
	if chunk.CHUNK_COUNT != uint(len(chunk.CHUNK_HASH)) {
		return fmt.Errorf("Corrupted Chunk")
	}

	if chunk.CHUNK_ID >= chunk.CHUNK_COUNT {
		return fmt.Errorf("Invalid Chunk id")
	}

	if chunk.CHUNK_NEED < 10 {
		return fmt.Errorf("Insufficient chunk")
	}
	parity := chunk.CHUNK_COUNT - chunk.CHUNK_NEED
	if parity < chunk.CHUNK_NEED {
		return fmt.Errorf("Insufficient parity")
	}

	if uint64(chunk.DSIZE) > config.STARGATE_HE_MAX_BLOCK_SIZE {
		return fmt.Errorf("Invalid Chunk size")
	}

	if chunk.CHUNK_HASH[chunk.CHUNK_ID] != crypto.Keccak256_64(chunk.CHUNK_DATA) { // chunk data corrupt
		return fmt.Errorf("Corrupted Chunk")
	}

	if nil != is_chunk_exist(chunk.HHash, uint8(chunk.CHUNK_ID)) { // chunk already exists return
		return nil
	}

	chunks_per_block := new(Chunks_Per_Block_Data)
	if chunksi, ok := chunk_map.LoadOrStore(fmt.Sprintf("%x", chunk.HHash), chunks_per_block); ok {
		chunks_per_block = chunksi.(*Chunks_Per_Block_Data)

		// make sure we are matching what is stored already
		var existing *Block_Chunk
		for j := range chunks_per_block.ChunkCollection {
			if existing = chunks_per_block.ChunkCollection[j]; existing != nil {
				break
			}
		}
		if existing != nil { // compare current headers wuth what we have
			if chunk.HHash != existing.HHash || len(chunk.CHUNK_HASH) != len(existing.CHUNK_HASH) {
				return nil // this collision can never occur
			}
			for j := range chunk.CHUNK_HASH {
				if chunk.CHUNK_HASH[j] != existing.CHUNK_HASH[j] {
					return nil // again this is impossible
				}
			}
		}

	}

	if chunks_per_block.bl == nil {

		var bl block.Block
		if err := bl.Deserialize(chunk.BLOCK); err != nil {
			logger.V(1).Error(err, "error deserializing block")
			return nil
		}
		if bl.GetHash() != chunk.BLID {
			return fmt.Errorf("Corrupted Chunk. bad block data")
		}

		// we must check the Pow now
		if int64(bl.Height) >= chain.Get_Height()-3 && int64(bl.Height) <= chain.Get_Height()+3 {

		} else {
			return nil // we need not broadcast
		}

		if len(bl.Tips) == 0 || len(bl.MiniBlocks) < 5 {
			return nil
		}

		for _, mbl := range bl.MiniBlocks {
			if !chain.VerifyMiniblockPoW(&bl, mbl) {
				return errormsg.ErrInvalidPoW
			}
		}

		chunks_per_block.Created = time.Now()
		chunks_per_block.Sent = sent
		chunks_per_block.bl = &bl
		chunks_per_block.ChunkCollection[chunk.CHUNK_ID] = chunk
	}

	if chunks_per_block.Processed {
		return nil
	}

	if chunks_per_block.ChunkCollection[chunk.CHUNK_ID] == nil {
		chunks_per_block.ChunkCollection[chunk.CHUNK_ID] = chunk
		broadcast_Chunk(chunk, 0, sent) // broadcast chunk INV
	}

	chunk_count := 0
	for _, c := range chunks_per_block.ChunkCollection {
		if c != nil {
			chunk_count++
		}
	}

	logger.V(3).Info("Have  chunks", "have", chunk_count, "total", chunk.CHUNK_COUNT, "tx_count", len(chunks_per_block.bl.Tx_hashes))

	var cbl Complete_Block
	cbl.Block = chunk.BLOCK
	if len(chunks_per_block.bl.Tx_hashes) >= 1 { // if txs are present, then we need to join chunks, else we are already done

		if uint(chunk_count) < chunk.CHUNK_NEED { // we do not have enough chunks
			return nil
		}

		var shards [][]byte
		for i := 0; i < int(chunk.CHUNK_COUNT); i++ {
			if chunks_per_block.ChunkCollection[i] == nil {
				shards = append(shards, nil)
			} else {
				shards = append(shards, chunks_per_block.ChunkCollection[i].CHUNK_DATA)
			}
		}

		enc, _ := reedsolomon.New(int(chunk.CHUNK_NEED), int(chunk.CHUNK_COUNT-chunk.CHUNK_NEED))

		if err := enc.Reconstruct(shards); err != nil {
			logger.V(3).Error(err, "error reconstructing data ")
			return nil
		}

		var writer bytes.Buffer

		if err := enc.Join(&writer, shards, int(chunk.DSIZE)); err != nil {
			logger.V(1).Error(err, "error joining data")
			return nil
		}

		if err := cbor.Unmarshal(writer.Bytes(), &cbl); err != nil {
			logger.V(1).Error(err, "error deserializing txset")
			return nil
		}
	}

	chunks_per_block.Processed = true // we have successfully reconstructed data,so we give it a try

	object := Objects{CBlocks: []Complete_Block{cbl}}

	// first complete all our chunks, so as we can give to others
	logger.V(2).Info("successfully reconstructed using chunks", "blid", chunks_per_block.bl.GetHash(), "have", chunk_count, "total", chunk.CHUNK_COUNT, "tx_count", len(cbl.Txs))

	if chunks_per_block.Sent != 0 && chunks_per_block.Sent < globals.Time().UTC().UnixMicro() {
		time_to_receive := float64(globals.Time().UTC().UnixMicro()-chunks_per_block.Sent) / 1000000
		metrics.Set.GetOrCreateHistogram("block_propagation_duration_histogram_seconds").Update(time_to_receive)
	}

	if err := connection.processChunkedBlock(object, int(chunk.CHUNK_NEED), int(chunk.CHUNK_COUNT-chunk.CHUNK_NEED)); err != nil {
		//fmt.Printf("error inserting block received using chunks, err %s", err)
	}

	return nil
}

// cehck whether we have already chunked this
func is_already_chunked_by_us(blid crypto.Hash, data_shard_count, parity_shard_count int) (hash [32]byte, chunk_count int) {
	chunk_map.Range(func(key, value interface{}) bool {

		chunks_per_block := value.(*Chunks_Per_Block_Data)
		for _, c := range chunks_per_block.ChunkCollection {
			if c != nil && c.BLID == blid && int(c.CHUNK_NEED) == data_shard_count && int(c.CHUNK_COUNT-c.CHUNK_NEED) == parity_shard_count && chunks_per_block.Complete {
				hash = c.HeaderHash()
				chunk_count = data_shard_count + parity_shard_count
				return false
			}
		}
		return true
	})
	return
}

// convert complete block to p2p block format
func Convert_CBL_TO_P2PCBL(cbl *block.Complete_Block, processblock bool) []byte {
	var cbor_cbl Complete_Block

	if processblock {
		cbor_cbl.Block = cbl.Bl.Serialize()
	}
	for _, tx := range cbl.Txs {
		cbor_cbl.Txs = append(cbor_cbl.Txs, tx.Serialize())
	}
	if len(cbor_cbl.Txs) != len(cbl.Bl.Tx_hashes) {
		panic("invalid complete block")
	}

	cbl_serialized, err := cbor.Marshal(cbor_cbl)
	if err != nil {
		panic(err)
	}
	return cbl_serialized
}

// convert p2p complete block to complete block format
func Convert_P2PCBL_TO_CBL(input []byte) *block.Complete_Block {
	var cbor_cbl Complete_Block
	cbl := &block.Complete_Block{Bl: &block.Block{}}

	if err := cbor.Unmarshal(input, &cbor_cbl); err != nil {
		panic(err)
	}

	if err := cbl.Bl.Deserialize(cbor_cbl.Block); err != nil {
		panic(err)
	}

	for _, tx_bytes := range cbor_cbl.Txs {
		var tx transaction.Transaction
		if err := tx.Deserialize(tx_bytes); err != nil {
			panic(err)
		}
		cbl.Txs = append(cbl.Txs, &tx)
	}

	return cbl
}

// note we do not send complete block,
// since other nodes already have most of the mempool, let the mempool be used as much as possible
func convert_block_to_chunks(cbl *block.Complete_Block, data_shard_count, parity_shard_count int) ([32]byte, int) {
	var cbor_cbl TXSET
	bl_serialized := cbl.Bl.Serialize()
	blid := [32]byte(cbl.Bl.GetHash())

	for _, tx := range cbl.Txs {
		cbor_cbl.Txs = append(cbor_cbl.Txs, tx.Serialize())
	}
	if len(cbor_cbl.Txs) != len(cbl.Bl.Tx_hashes) {
		panic("invalid complete block")
	}

	cbl_serialized, err := cbor.Marshal(cbor_cbl)
	if err != nil {
		panic(err)
	}

	//if hhash, count := is_already_chunked_by_us(blid, data_shard_count, parity_shard_count); count > 0 {
	//	return hhash, count
	//}
	// loop through all the data chunks and overide from there
	chunk_map.Range(func(key, value interface{}) bool {
		chunk := value.(*Chunks_Per_Block_Data)
		for _, c := range chunk.ChunkCollection {
			if c != nil && c.BLID == blid {
				if data_shard_count != int(c.CHUNK_NEED) || parity_shard_count != int(c.CHUNK_COUNT-c.CHUNK_NEED) {
					data_shard_count = int(c.CHUNK_NEED)
					parity_shard_count = int(c.CHUNK_COUNT - c.CHUNK_NEED)
					logger.V(2).Info("overiding shards count to %d, parity to %d\n", "data_count", data_shard_count, "parity_count", parity_shard_count)
				}
				return false
			}
		}
		return true
	})

	// we will use a 16 datablocks,32 parity blocks RS code,
	// if the peer receives any of 16 blocks in any order, they can reconstruct entire block

	// Create an encoder with 16 data and 32 parity slices.
	// you must use atleast 10 data chunks and parity must be equal or more  than data chunk count

	if data_shard_count < 10 {
		panic(fmt.Errorf("data shard  must be > 10, actual %d", data_shard_count))
	}
	if parity_shard_count < data_shard_count {
		panic(fmt.Errorf("parity shard must be equal or more than data shards. data_shards %d parity shards %d", data_shard_count, parity_shard_count))
	}
	enc, _ := reedsolomon.New(data_shard_count, parity_shard_count)

	shards, err := enc.Split(cbl_serialized)
	if err != nil {
		panic(err)
	}

	if err = enc.Encode(shards); err != nil {
		panic(err)
	}

	chunk := make([]Block_Chunk, data_shard_count+parity_shard_count, data_shard_count+parity_shard_count)

	var chunk_hash []uint64

	for i := 0; i < data_shard_count+parity_shard_count; i++ {
		chunk_hash = append(chunk_hash, crypto.Keccak256_64(shards[i]))
	}

	for i := 0; i < data_shard_count+parity_shard_count; i++ {
		chunk[i].BLID = blid
		chunk[i].DSIZE = uint(len(cbl_serialized))
		chunk[i].BLOCK = bl_serialized
		chunk[i].CHUNK_ID = uint(i)
		chunk[i].CHUNK_COUNT = uint(data_shard_count + parity_shard_count)
		chunk[i].CHUNK_NEED = uint(data_shard_count)
		chunk[i].CHUNK_HASH = chunk_hash
		chunk[i].CHUNK_DATA = shards[i]
		chunk[i].HHash = chunk[i].HeaderHash()

		if chunk[0].HHash != chunk[i].HHash {
			panic("Corrupt data")
		}

	}
	chunks := new(Chunks_Per_Block_Data)
	for i := 0; i < data_shard_count+parity_shard_count; i++ {
		chunks.ChunkCollection[i] = &chunk[i]
	}
	chunks.Created = time.Now()
	chunks.Processed = true
	chunks.Complete = true

	chunk_map.Store(fmt.Sprintf("%x", chunk[0].HHash), chunks)
	return chunk[0].HeaderHash(), data_shard_count + parity_shard_count
}
