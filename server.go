package main

import (
	"crypto/tls"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

var accessLogFilepath = "./data/accessLog.txt"

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type player struct {
	uid, name, roomKey, category string
	conn                         *websocket.Conn
}

type room struct {
	players []player
}

var rooms = make(map[string]*room) //roomKeyでルーム指定

var mux *http.ServeMux = http.NewServeMux()

func getPlayerIdx(roomKey string, uid string) int {
	if !isRoom(roomKey) {
		return -1
	}

	players := rooms[roomKey].players
	for i := 0; i < len(players); i++ {
		if players[i].uid == uid {
			return i
		}
	}

	return -1 //見つからなかった時
}

func enterRoom(roomKey string, player *player) {
	roomKey += player.category
	if !isRoom(roomKey) { //部屋が無いなら作る
		makeRoom(roomKey)
	}

	idx := getPlayerIdx(roomKey, player.uid)
	if idx == -1 { //まだ自分が部屋に入ってなかったら追加
		rooms[roomKey].players = append(rooms[roomKey].players, *player)
		player.roomKey = roomKey
	}
}

func exitRoom(roomKey string, plData *player) {
	pIdx := getPlayerIdx(roomKey, plData.uid)
	if pIdx != -1 { //自分が部屋にいたら部屋から抜ける
		a := rooms[roomKey].players
		a[pIdx] = a[len(a)-1]
		a = a[:len(a)-1]
		rooms[roomKey].players = a
		plData.roomKey = ""
	}
}

func moveRoom(toRoomKey string, plData *player) {
	exitRoom(plData.roomKey, plData)
	enterRoom(toRoomKey, plData)
}

func sendMsg(conn *websocket.Conn, msg string) {
	conn.WriteMessage(websocket.TextMessage, []byte(msg))
}

func readCmd(str string) ([]string, string, int) {
	cmd := strings.Split(string(str), " ")
	cmdType := cmd[0]
	cmdLen := len(cmd)
	return cmd, cmdType, cmdLen
}

func bloadcastMsg(roomKey string, msg string) {
	for i := 0; i < len(rooms[roomKey].players); i++ {
		sendMsg(rooms[roomKey].players[i].conn, msg)
	}
}

func isRoom(roomKey string) bool {
	_, ok := rooms[roomKey]
	return ok
}

func makeRoom(roomKey string) {
	rooms[roomKey] = &room{players: []player{}}
}

func sendMsgToAnother(roomKey string, exceptPl player, msg string) {
	//自分以外にコマンド送信
	exceptIdx := getPlayerIdx(roomKey, exceptPl.uid)
	toIdx := 1 - exceptIdx
	sendMsg(rooms[roomKey].players[toIdx].conn, msg)
}

func addHandller(category string, url string, fn func(plData player)) {
	handller := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "https://galleon.yachiyo.tech")

		upgrader.CheckOrigin = func(r *http.Request) bool { return true }
		//リモートアドレスからのアクセスを許可する
		conn, _ := upgrader.Upgrade(w, r, nil)

		//ログファイルにアクセスを記録
		log := time.Now().Format("2006/1/2 15:04:05") + " | " + r.RemoteAddr + " | " + category
		WriteFileAppend(accessLogFilepath, log)

		plData := player{uid: MakeUuid(), name: "名無し", roomKey: "default", category: category, conn: conn}

		fn(plData)
	}

	mux.HandleFunc(url, handller)
}

func startServer(port string, fullchainPath string, privkeyPath string) {
	//tls設定
	cfg := &tls.Config{
		ClientAuth: tls.RequestClientCert,
	}

	//サーバー設定
	srv := http.Server{
		Addr:      ":" + port,
		Handler:   mux,
		TLSConfig: cfg,
	}

	println("サーバー起動")
	err := srv.ListenAndServeTLS(fullchainPath, privkeyPath)
	if err != nil {
		println("サーバー起動に失敗")
		println(err.Error())
	}
}
