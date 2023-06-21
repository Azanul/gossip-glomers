package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"

	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
)

type TxnMessage struct {
	Transactions [][3]interface{} `json:"txn"`
}

func main() {
	n := maelstrom.NewNode()

	kv := sync.Map{}

	n.Handle("txn", func(msg maelstrom.Message) error {
		var body TxnMessage
		if err := json.Unmarshal(msg.Body, &body); err != nil {
			return err
		}

		txns := [][3]interface{}{}
		for _, txn := range body.Transactions {
			txnType := txn[0].(string)

			targetKey := txn[1].(float64)
			crr := [3]interface{}{txnType, targetKey, nil}
			var v interface{}
			switch txnType {
			case "r":
				v, _ = kv.Load(string(rune(targetKey)))

			case "w":
				v = txn[2]
				kv.Store(string(rune(targetKey)), v.(float64))
			}
			crr[2] = v
			txns = append(txns, crr)
		}

		go replicate(txns, n)

		return n.Reply(msg, map[string]any{"type": "txn_ok", "txn": txns})
	})

	n.Handle("sync", func(msg maelstrom.Message) error {
		var body TxnMessage
		if err := json.Unmarshal(msg.Body, &body); err != nil {
			return err
		}

		for _, txn := range body.Transactions {
			txnType := txn[0].(string)
			targetKey := txn[1].(float64)
			if txnType == "w" {
				kv.Store(string(rune(targetKey)), txn[2].(float64))
			}

		}
		return n.Reply(msg, map[string]any{"type": "sync_ok"})
	})

	if err := n.Run(); err != nil {
		log.Fatal(err)
	}
}

func replicate(txns [][3]interface{}, n *maelstrom.Node) {
	body := map[string]any{
		"type": "sync",
		"txn":  txns,
	}
	for _, dest := range n.NodeIDs() {
		_, err := n.SyncRPC(context.TODO(), dest, body)
		if err != nil {
			fmt.Fprint(os.Stderr, "Error message: ", err, "\n")
		}
	}
}
