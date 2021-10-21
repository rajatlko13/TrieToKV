package main

import (
	"fmt"
	"log"
	"os"

	"github.com/boltdb/bolt"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state/snapshot"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/trie"
)

func main() {
	// Open an existing Ethereum database
	db, err := rawdb.NewLevelDBDatabase(os.Args[1], 16, 16, "", false)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	// Retrieve the head block
	hash := rawdb.ReadHeadBlockHash(db)
	number := rawdb.ReadHeaderNumber(db, hash)
	if number == nil {
		log.Fatalf("Failed to retrieve head block number")
	}
	head := rawdb.ReadBlock(db, hash, *number)
	if head == nil {
		log.Fatalf("Failed to retrieve head block")
	}
	// Retrieve the state belonging to the head block
	st, err := trie.New(head.Root(), trie.NewDatabase(db))
	if err != nil {
		log.Fatalf("Failed to retrieve account trie: %v", err)
	}
	log.Printf("Indexing block #%d [%x]", *number, hash)

	// Starting the boltDB connection
	boltDB, err := bolt.Open("myConnection.db", 0600, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer boltDB.Close()

	// Creating the DB bucket
	err = boltDB.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("DB"))
		if err != nil {
			return fmt.Errorf("could not create the DB bucket: %v", err)
		}
		return nil
	})
	if err != nil {
		fmt.Errorf("could not set up buckets, %v", err)
	}

	it := trie.NewIterator(st.NodeIterator(nil))
	for it.Next() {
		var account snapshot.Account

		rlp.DecodeBytes(it.Value, &account)
		log.Printf("Iterating trie #%s", string(it.Key))

		var arr [32]byte
		copy(arr[:], account.Root[:32])
		stateRoot, err := trie.New(arr, trie.NewDatabase(db))
		if err != nil {
			log.Fatalf("Failed to retrieve account trie: %v", err)
		}
		stateIterator := trie.NewIterator(stateRoot.NodeIterator(nil))

		for stateIterator.Next() {
			var nodeAccount snapshot.Account
			rlp.DecodeBytes(stateIterator.Value, &nodeAccount)
			log.Printf("Iterating stateTrie #%s", string(stateIterator.Key))

			// accountBytes, err := json.Marshal(account)
			// if err != nil {
			// 	return fmt.Errorf("could not marshal account json: %v", err)
			// }
			// stateBytes, err := json.Marshal(stateRoot)
			// if err != nil {
			// 	return fmt.Errorf("could not marshal state root json: %v", err)
			// }

			err = boltDB.Update(func(tx *bolt.Tx) error {
				err = tx.Bucket([]byte("DB")).Put([]byte(string(it.Key)+"_"+string(stateIterator.Key)), stateIterator.Value)
				if err != nil {
					return fmt.Errorf("Could not add to KV database: %v", err)
				}
				return nil
			})

		}

	}
}
