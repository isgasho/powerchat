package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"time"
)

var (
	token      []byte
	id         int64
	httpId     int64
	proxyPort  int
	cSrv       *pChatClient
	serverAddr string
	servePort  int = 7890
	pgPath     string
	retry      bool = false
	nameRetry  string
	pwdRetry   string
	lServe     *localServer
)

func init() {
	cSrv = new(pChatClient)
	main_init()
	//log.Printf("CmdChat:%d,CmdSysReturn:%d\n", CmdChat, CmdSysReturn)
}

func client() {
	defer notifyMsg(&MsgType{CmdChat, 0, 0, []byte("Connection Down! re-connect 10s later")})
	var cfg tls.Config
	//	roots := x509.NewCertPool()
	//	pem, _ := ioutil.ReadFile("pems/a-cert.pem")
	//	roots.AppendCertsFromPEM(pem)
	//	cfg.RootCAs = roots
	cfg.InsecureSkipVerify = true
	conn1, err := tls.Dial("tcp", serverAddr, &cfg)
	if err != nil {
		log.Println(err)
		return
	} else {
		log.Println("connected")
	}
	defer conn1.Close()
	go httpProxy2(conn1)
	var ok bool = true
	if !retry {
		lServe = newLocalServer(conn1)
		res1 := make(chan bool, 1)
		go lServe.Serve(res1)
		ok = <-res1
		close(res1)
	} else {
		lServe.SetConn(conn1)
	}

	if ok {
		readConn(conn1)
	}
}

type UserInfo struct {
	Id       int64
	Name     string
	Sex      int
	Birthday string
	Desc     string
	Pwdmd5   string
}

func OnReady(msg *MsgType) {
	token = msg.Msg
	cSrv.token = msg.Msg
	if retry {
		log.Println("login again.")
		go retry_login()
	}
}

//for httpProxy
var httpChan chan MsgType
var serveChan chan MsgType = make(chan MsgType, 1)

//goroutine replace httpServe and startcSrv4Glib
type localServer struct {
	conn net.Conn
}

func newLocalServer(c net.Conn) *localServer {
	cSrv.setConn(c)
	return &localServer{conn: c}
}

func (s *localServer) SetConn(c net.Conn) {
	s.conn = c
	cSrv.setConn(c)
}

func (s *localServer) Serve(res1 chan bool) {
	var l net.Listener
	var err error
	for i := 0; i < 8; i++ {
		l, err = net.Listen("tcp", fmt.Sprintf("localhost:%d", servePort))
		if err == nil {
			break
		} else {
			servePort++
			proxyPort = servePort + 2000
		}
	}
	if err != nil {
		panic(err)
	}
	defer l.Close()

	u1, _ := user.Current()
	go startMyHttpServe(filepath.Join(u1.HomeDir, "ChatShare"), fmt.Sprintf("localhost:%d", proxyPort))
	res1 <- true
	go httpRespRouter()
	for {
		conn, err := l.Accept()
		if err != nil {
			log.Println("2.accept", err)
			return
		}
		go httpResponse2(s.conn, conn, httpId)

	}
}

func pushHttpChan(msg *MsgType) {
	httpChan <- *msg
}
func pushServeChan(msg *MsgType) {
	serveChan <- *msg
	//fmt.Printf("%v\n", p)
}

//goroutine
func readConn(conn1 net.Conn) {
	defer conn1.Close()

	for {
		conn1.SetReadDeadline(time.Now().Add(time.Minute * 3))
		msgb, err := ReadMsg(conn1)
		if err != nil {
			log.Printf("ReadMsg:%v\n", err)
			//notifyMsg(&MsgType{Cmd: CmdSysReturn, From: 0, To: 0, Msg: []byte("ConnDown\n")})
			return
		}
		msg := MsgDecode(msgb)
		switch msg.Cmd {
		case CmdReady:
			OnReady(msg)
		case CmdChat:
			notifyMsg(msg)
		case CmdHttpRequest:
			pushHttpChan(msg)
		case CmdHttpReqContinued:
			pushHttpChan(msg)
		case CmdHttpReqClose:
			pushHttpChan(msg)

		case CmdHttpRespContinued:
			pushServeChan(msg)
		case CmdHttpRespClose:
			pushServeChan(msg)

		case CmdFileHeader:
			pushFileMsg2(conn1, msg)
		case CmdFileContinued:
			//file
			pushFileMsg2(conn1, msg)
		case CmdFileClose:
			//file
			pushFileMsg2(conn1, msg)
		case CmdFileCancel:
			//file
			pushFileMsg2(conn1, msg)
		case CmdFileAccept:
			//begin send file
			fileResp <- *msg
		case CmdFileBlock:
			//cancel send file
			fileResp <- *msg
		case CmdFileStop:
			//stop send file by session
		case CmdLogResult:
			cmdChan <- *msg
		case CmdRegResult:
			cmdChan <- *msg
		case CmdRetFriends:
			cmdChan <- *msg
		case CmdReturnPersons:
			cmdChan <- *msg
		case CmdSysReturn:
			notifyMsg(msg)
		case CmdReturnStrangers:
			cmdChan <- *msg
		case CmdReturnQueryID:
			cmdChan <- *msg
		case CmdUserStatus:
			cmdChan <- *msg
		default:
			log.Printf("Cmd:%d From:%d To:%d Msg:%s\n", msg.Cmd, msg.From, msg.To, string(msg.Msg))
		}
	}
}
func main_init() {
	servePort = 7890
	proxyPort = servePort + 2000
	filepath1, err := os.Executable()
	if err != nil {
		log.Fatal(err)
	}
	path1 := filepath.Dir(filepath1)
	pgPath = path1
	var cfg1 = make(map[string]string)
	cfgFile, err := ioutil.ReadFile(filepath.Join(path1, "..", "etc", "powerchat", "config.json"))
	if err != nil {
		log.Fatal(err)
	}
	err = json.Unmarshal(cfgFile, &cfg1)
	if err != nil {
		log.Fatal(err)
	}
	var ok bool
	serverAddr, ok = cfg1["Host"]
	if ok == false {
		log.Fatal("config file parse error\n")
	}

	go func() {
		for {
			httpChan = make(chan MsgType, 10)
			client()
			close(httpChan)
			log.Println("disconnect ,re-connect 10s later.")
			time.Sleep(time.Second * 10)
		}

	}()
}

func getRelatePath(name1 string) string {
	u1, _ := user.Current()
	res := filepath.Join(u1.HomeDir, ".powerchat", name1)
	os.MkdirAll(res, os.ModePerm)
	return res
}

func readManual(uid int64) map[string]string {
	dir1 := getRelatePath("manual")
	pathname := filepath.Join(dir1, fmt.Sprintf("%d", uid))
	p, err := ioutil.ReadFile(pathname)
	if err != nil {
		log.Println("readManual error:", err)
		return nil
	}
	res := make(map[string]string)
	err = json.Unmarshal(p, &res)
	if err != nil {
		log.Println("readManual error:", err)
		return nil
	}
	return res
}

func saveManual(uid int64, data map[string]string) error {
	dir1 := getRelatePath("manual")
	pathname := filepath.Join(dir1, fmt.Sprintf("%d", uid))
	p, err := json.Marshal(data)
	if err != nil {
		return err
	}
	fp, err := os.Create(pathname)
	if err != nil {
		return err
	}
	defer fp.Close()
	_, err = fp.Write(p)
	return err
}
