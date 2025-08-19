package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/rs/cors"
)

// Flutterから受け取る投票リクエストの形式
type VoteRequest struct {
	UserID string `json:"userId"` // ユーザーID (Firebaseから取得したものを想定)
	Vote   string `json:"vote"`   // "あつい", "ちょうどよい", "さむい" のいずれか
}

// サーバー内でデータを保持する変数
var (
	// 全体の投票数を保存するマップ
	voteCounts = map[string]int{
		"あつい":    0,
		"ちょうどよい": 0,
		"さむい":    0,
	}

	// どのユーザーが何に投票したかを保存するマップ
	userVotes = make(map[string]string)

	// 複数のリクエストが同時にデータを書き換えるのを防ぐためのロック
	mutex = &sync.Mutex{}
)

// POST /vote エンドポイントの処理
func voteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	var req VoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if _, ok := voteCounts[req.Vote]; !ok {
		http.Error(w, "Invalid vote option", http.StatusBadRequest)
		return
	}

	// 投票ロジック (データを保護するためにロック)
	mutex.Lock()
	defer mutex.Unlock() // 関数終了時に自動でロックを解除

	// ユーザーが以前に投票していたかチェック
	if previousVote, ok := userVotes[req.UserID]; ok {
		// 以前の投票があった場合、その票を1つ減らす
		if previousVote != req.Vote {
			voteCounts[previousVote]--
		}
	}

	// 新しい投票を記録
	voteCounts[req.Vote]++
	userVotes[req.UserID] = req.Vote

	log.Printf("Vote received: UserID=%s, Vote=%s", req.UserID, req.Vote)
	log.Printf("Current counts: %+v", voteCounts)

	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "Vote recorded successfully")
}

// GET /results エンドポイントの処理
func resultsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	mutex.Lock()
	defer mutex.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(voteCounts)
}

// main関数（修正済み）
func main() {
	// 新しいルーター(mux)を作成
	mux := http.NewServeMux()

	// http.HandleFuncではなく、mux.HandleFuncに処理を登録
	mux.HandleFunc("/vote", voteHandler)
	mux.HandleFunc("/results", resultsHandler)

	// CORSの設定を作成
	c := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST"},
	})

	// muxをCORSミドルウェアでラップして、最終的なhandlerを作成
	handler := c.Handler(mux)

	fmt.Println("Server starting on :8080")

	// サーバーの起動時に、nilではなく作成したhandlerを渡す
	if err := http.ListenAndServe(":8080", handler); err != nil {
		log.Fatalf("Could not start server: %s\n", err)
	}
}
