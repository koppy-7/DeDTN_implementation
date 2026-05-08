package pbft

import (
        "bytes"
        "encoding/json"
        "fmt"
        "net/http"
        "time"
)

func sendMessage(addr string, msg Message) {
        data, _ := json.Marshal(msg)
        url := "http://" + addr + "/message"
        client := &http.Client{Timeout: 2 * time.Second}

        for i := 0; i < 3; i++ { // Retry up to 3 times
                resp, err := client.Post(url, "application/json", bytes.NewReader(data))
                if err == nil {
                        resp.Body.Close()
                        return
                }
                fmt.Printf("[WARN] Retry %d: failed to send %s to %s (%v)\n", i+1, msg.Type, addr, err)
                time.Sleep(200 * time.Millisecond)
        }
}