package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

const (
	pingInterval = 5 * time.Second
	pongTimeout  = 10 * time.Second
	writeTimeout = 5 * time.Second
)

type feedReport struct {
	Report struct {
		FeedID     string `json:"feedID"`
		FullReport string `json:"fullReport"`
	} `json:"report"`
}

type reportFixture struct {
	Reports []reportEntry `json:"reports"`
}

type reportEntry struct {
	FeedID     string `json:"feed_id"`
	FullReport string `json:"full_report"`
}

func main() {
	var (
		host     string
		path     string
		feedIDs  string
		outPath  string
		timeout  time.Duration
		printRaw bool
	)

	flag.StringVar(&host, "host", "ws.dataengine.chain.link", "WebSocket host")
	flag.StringVar(&path, "path", "/api/v1/ws", "WebSocket path")
	flag.StringVar(&feedIDs, "feed-ids", "0x00039d9e45394f473ab1f050a1b963e6b05351e52d71e507509ada0c95ed75b8,0x00084edc844a6f88449c59c8cfcdb2225799a2330503472cb0bc4f9369a717fa", "Comma-separated feed IDs")
	flag.StringVar(&outPath, "out", "", "Optional output file for JSON fixture")
	flag.DurationVar(&timeout, "timeout", 60*time.Second, "Max time to wait for reports")
	flag.BoolVar(&printRaw, "print-raw", false, "Print raw JSON messages to stderr")
	flag.Parse()

	apiKey := os.Getenv("STREAMS_API_KEY")
	apiSecret := os.Getenv("STREAMS_API_SECRET")
	if apiKey == "" || apiSecret == "" {
		log.Fatal("missing API credentials (set STREAMS_API_KEY and STREAMS_API_SECRET)")
	}

	ids := splitFeedIDs(feedIDs)
	if len(ids) == 0 {
		log.Fatal("no feed IDs provided")
	}

	queryParams := fmt.Sprintf("feedIDs=%s", strings.Join(ids, ","))
	fullPath := fmt.Sprintf("%s?%s", path, queryParams)
	signature, timestamp := generateHMAC("GET", fullPath, apiKey, apiSecret)

	header := http.Header{}
	header.Add("Authorization", apiKey)
	header.Add("X-Authorization-Timestamp", fmt.Sprintf("%d", timestamp))
	header.Add("X-Authorization-Signature-SHA256", signature)

	wsURL := fmt.Sprintf("wss://%s%s?%s", host, path, queryParams)
	fmt.Fprintf(os.Stderr, "Connecting to %s\n", wsURL)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	conn, resp, err := websocket.DefaultDialer.DialContext(ctx, wsURL, header)
	if err != nil {
		if resp != nil {
			log.Fatalf("WebSocket connection error (HTTP %d): %v", resp.StatusCode, err)
		}
		log.Fatalf("WebSocket connection error: %v", err)
	}
	defer conn.Close()

	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(pongTimeout))
	})
	if err := conn.SetReadDeadline(time.Now().Add(pongTimeout)); err != nil {
		log.Fatalf("failed to set read deadline: %v", err)
	}

	done := make(chan struct{})
	reports := make(map[string]string)
	expected := make(map[string]struct{})
	for _, id := range ids {
		expected[strings.ToLower(id)] = struct{}{}
	}

	go pingLoop(ctx, conn)

	go func() {
		defer close(done)
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				log.Printf("read error: %v", err)
				return
			}
			if printRaw {
				fmt.Fprintf(os.Stderr, "raw: %s\n", string(message))
			}
			var report feedReport
			if err := json.Unmarshal(message, &report); err != nil {
				log.Printf("failed to parse message: %v", err)
				continue
			}
			feedID := strings.ToLower(report.Report.FeedID)
			if feedID == "" || report.Report.FullReport == "" {
				continue
			}
			if _, ok := expected[feedID]; !ok {
				continue
			}
			if _, exists := reports[feedID]; !exists {
				reports[feedID] = report.Report.FullReport
				fmt.Fprintf(os.Stderr, "captured report for %s\n", report.Report.FeedID)
			}
			if len(reports) == len(expected) {
				return
			}
		}
	}()

	select {
	case <-done:
	case <-interrupt:
		fmt.Fprintln(os.Stderr, "interrupt received, closing connection")
	case <-ctx.Done():
		fmt.Fprintln(os.Stderr, "timeout waiting for reports")
	}

	if err := conn.WriteControl(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
		time.Now().Add(writeTimeout),
	); err != nil {
		fmt.Fprintf(os.Stderr, "close error: %v\n", err)
	}

	if len(reports) != len(expected) {
		missing := missingFeedIDs(expected, reports)
		log.Fatalf("missing reports for feed IDs: %s", strings.Join(missing, ", "))
	}

	entries := make([]reportEntry, 0, len(ids))
	for _, id := range ids {
		entries = append(entries, reportEntry{
			FeedID:     id,
			FullReport: reports[strings.ToLower(id)],
		})
	}

	out, err := json.MarshalIndent(reportFixture{Reports: entries}, "", "  ")
	if err != nil {
		log.Fatalf("failed to marshal output: %v", err)
	}

	if outPath == "" {
		fmt.Printf("%s\n", out)
		return
	}
	if err := os.WriteFile(outPath, out, 0o600); err != nil {
		log.Fatalf("failed to write output: %v", err)
	}
}

func generateHMAC(method, path, apiKey, apiSecret string) (string, int64) {
	timestamp := time.Now().UTC().UnixMilli()
	bodyHash := sha256.Sum256(nil)
	stringToSign := fmt.Sprintf("%s %s %s %s %d", method, path, hex.EncodeToString(bodyHash[:]), apiKey, timestamp)

	signedMessage := hmac.New(sha256.New, []byte(apiSecret))
	signedMessage.Write([]byte(stringToSign))
	signature := hex.EncodeToString(signedMessage.Sum(nil))

	return signature, timestamp
}

func pingLoop(ctx context.Context, conn *websocket.Conn) {
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(writeTimeout)); err != nil {
				return
			}
		}
	}
}

func splitFeedIDs(feedIDs string) []string {
	parts := strings.Split(feedIDs, ",")
	ids := make([]string, 0, len(parts))
	for _, id := range parts {
		trimmed := strings.TrimSpace(id)
		if trimmed == "" {
			continue
		}
		ids = append(ids, trimmed)
	}
	return ids
}

func missingFeedIDs(expected map[string]struct{}, reports map[string]string) []string {
	missing := make([]string, 0)
	for feedID := range expected {
		if _, ok := reports[feedID]; !ok {
			missing = append(missing, feedID)
		}
	}
	sortStrings(missing)
	return missing
}

func sortStrings(values []string) {
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j] < values[j-1]; j-- {
			values[j], values[j-1] = values[j-1], values[j]
		}
	}
}
