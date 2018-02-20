package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"

	homedir "github.com/mitchellh/go-homedir"
	"golang.org/x/crypto/ssh"
)

var help bool
var quiet bool
var privateKey string
var port string

const (
	defaultPort = "22"
)

func init() {
	flag.BoolVar(&help, "h", false, "Show help")
	flag.BoolVar(&quiet, "q", false, "Don't show the INFO log")
	flag.StringVar(&privateKey, "i", "", "Private key")
	flag.StringVar(&port, "p", "", "Port")
	flag.Parse()
}

func main() {
	if help {
		showUsage()
		os.Exit(0)
	}

	args := flag.Args()
	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "ERROR: Arguments length is invalid\n")
		fmt.Fprintf(os.Stderr, "Usage: %s [configfile]\n", os.Args[0])
		os.Exit(1)
	}

	file, err := os.Open(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err.Error())
		os.Exit(1)
	}

	var dests []Dest
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&dests); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err.Error())
		os.Exit(1)
	}

	if privateKey == "" {
		home, err := homedir.Dir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err.Error())
			os.Exit(1)
		}
		privateKey = filepath.Join(home, ".ssh", "id_rsa")
	}

	if port == "" {
		port = defaultPort
	}

	var wg sync.WaitGroup
	wg.Add(len(dests))
	for _, dest := range dests {
		go call(&wg, dest.Host, dest.User)
	}
	wg.Wait()
}

func call(wg *sync.WaitGroup, host, user string) {
	defer wg.Done()
	session, err := NewSession(host, port, user, privateKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: [Host] %s@%s\n", host, user)
	}
	defer session.Close()

	bytes, err := session.GetCrontab()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: [Host] %s@%s\n", host, user)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: [Host] %s@%s\n", host, user)
		fmt.Fprintf(os.Stderr, "%s\n", err.Error())
	}

	content := fmt.Sprintf(string(bytes))
	fmt.Printf("[Host] %s@%s\n", user, host)
	fmt.Printf("[Content] \n%s\n", content)
}

func showUsage() {
	fmt.Fprintf(os.Stderr, "Usage: %s [configfile]\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "Flags:\n")
	flag.PrintDefaults()
}

type Dest struct {
	Host string `json:"host"`
	User string `json:"user"`
}

// Session is struct representing ssh Session
type Session struct {
	config    *ssh.ClientConfig
	conn      *ssh.Client
	session   *ssh.Session
	StdinPipe io.WriteCloser
}

// NewSession returns new Session instance
func NewSession(ip, port, user, privateKey string) (*Session, error) {
	buf, err := ioutil.ReadFile(privateKey)
	if err != nil {
		return nil, err
	}

	key, err := ssh.ParsePrivateKey(buf)
	if err != nil {
		return nil, err
	}

	config := &ssh.ClientConfig{
		User:            user,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(key),
		},
	}

	conn, err := ssh.Dial("tcp", ip+":"+port, config)
	if err != nil {
		return nil, err
	}

	session, err := conn.NewSession()
	if err != nil {
		return nil, err
	}

	return &Session{
		config:  config,
		conn:    conn,
		session: session,
	}, nil
}

// Close close the session & connection
func (s *Session) Close() {
	if s.session != nil {
		s.session.Close()
	}

	if s.conn != nil {
		s.conn.Close()
	}
}

// Get is func that get file contents
func (s *Session) GetCrontab() ([]byte, error) {
	cmd := fmt.Sprintf("crontab -l\n")
	return s.session.Output(cmd)
}
