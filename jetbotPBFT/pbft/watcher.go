package pbft

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"time"
)

// WatchSemanticFile monitors a JSON file and automatically proposes it
func (n *Node) WatchSemanticFile(path string) {
	go func() {
		lastMod := time.Time{}
		for {
			info, err := os.Stat(path)
			if err == nil && info.ModTime().After(lastMod) {

				lastMod = info.ModTime()

				data, err := ioutil.ReadFile(path)
				if err != nil {
					fmt.Printf("[Watcher]  ERROR reading file: %v\n", err)
					continue
				}

				var tx Transaction
				if err := json.Unmarshal(data, &tx); err != nil {
					fmt.Printf("[Watcher]  ERROR parsing semantic JSON: %v\n", err)
					fmt.Println("[Watcher] RAW JSON:", string(data))
					continue
				}

				fmt.Printf("[Watcher] New semantic data detected: %s\n", tx.SceneHash)

				payload, err := json.Marshal(tx)
				if err != nil {
					fmt.Printf("[Watcher]  ERROR marshaling tx: %v\n", err)
					continue
				}

				nowMs := time.Now().UnixNano() / 1e6

				n.setRoundStart(tx.SceneHash, nowMs)
				n.appendReadLatest(tx.SceneHash, nowMs)

				n.Propose(tx.SceneHash, payload)
			}

			time.Sleep(4 * time.Second)
		}
	}()
}
