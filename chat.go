// This is the chat app. It allows transmission of text
// messages between an arbitrary number of users. All
// messages are passed through a single server. The app
// can be started in one of two modes:
//
//	server mode
//	     The app listens for connections from clients
//	     and relays messages between them.
//	client mode
//	     The app reads text from standard input
//	     and sends them to the server for broadcast
//	     to other clients.
package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
)

// This function starts a new server session by listening
// for incoming client connections on the given port.
//
// The endpoint will be an address:port pair, such as
// 0.0.0.0:8011 to connect to port 8011 on localhost.
//
// The server needs to do the following actions:
//
//	Wait for clients to connect.
//	Respond to new clients by sending them the
//	  message log.
//	Handle new messages sent from clients by
//	  adding them to the message log and
//	  broadcasting them to all other clients.

// TODO RETROACTIVELY SEND MSG HISTORY TO NEW USERS

type messagePacket struct {
	text   string
	source string // this should be the connection address
	sender string // connection's username
}

type user struct {
	connection net.Conn
	username   string
}

func server(port int) {
	ln, err := net.Listen("tcp4", ":"+strconv.Itoa(port))
	if err != nil {
		log.Print(err)
	}

	log.Println("Listening on", ln.Addr())

	messageChannel := make(chan messagePacket)
	var threadGroup sync.WaitGroup

	// [address, <net.Conn obj>]
	connectionPool := make(map[string]user)

	var messageHistory []messagePacket

	threadGroup.Add(1)
	go serverBroadCast(&connectionPool, &messageChannel, &threadGroup, &messageHistory)

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Print(err)
			continue
		}

		go handleConnection(conn, &connectionPool, &messageChannel, &messageHistory)

	}

}

func handleConnection(conn net.Conn, connectionPool *map[string]user, messageChannel *chan messagePacket, messageHistory *[]messagePacket) {
	defer conn.Close()
	connectionAddress := conn.RemoteAddr().String()

	// read username
	userBuf := make([]byte, 1024)
	size, err := conn.Read(userBuf)

	if err != nil {
		log.Print(err)
		return
	}

	name := strings.TrimSpace(string(userBuf[:size]))

	var newUser = user{
		connection: conn,
		username:   name,
	}

	(*connectionPool)[connectionAddress] = newUser

	log.Print("New connection from user ", name)

	// retroactively send them messages
	for _, packet := range *messageHistory {
		res := "BROADCAST " + packet.sender + ": " + packet.text + "\n"

		conn.Write([]byte(res))
	}

	for {
		// block until message received
		buffer := make([]byte, 1024)

		size, err := (conn).Read(buffer)

		if err == io.EOF {
			log.Print(name, " has disconnected")
			return
		} else if err != nil {
			log.Print(err)
		}

		packet := messagePacket{
			text:   strings.TrimSpace(string(buffer[:size])),
			source: connectionAddress,
			sender: name,
		}
		*messageChannel <- packet

		buffer = nil

	}
}

func serverBroadCast(connectionPool *map[string]user, messageChannel *chan messagePacket,
	threadGroup *sync.WaitGroup, messageHistory *[]messagePacket) {
	defer threadGroup.Done()

	for {
		packet := <-*messageChannel

		// add packet to history
		*messageHistory = append(*messageHistory, packet)

		for _, userConn := range *connectionPool {
			// don't want to send broadcast to the source address
			if packet.source != userConn.connection.RemoteAddr().String() {
				res := "BROADCAST " + packet.sender + ": " + packet.text

				userConn.connection.Write([]byte(res))
			}

		}
	}
}

// Helper function reads a line of input from
// the terminal. Roughly equivalent to Python
// 3's input(). Removes leading and trailing
// whitespace.
func readln() string {
	reader := bufio.NewReader(os.Stdin)
	text, _ := reader.ReadString('\n')
	return strings.TrimSpace(text)
}

// This function starts a new client session by connecting
// to the server at the given endpoint.
//
// The endpoint will be an address:port pair, such as
// 0.0.0.0:8011 to connect to port 8011 on localhost.
//
// The client needs to do the following actions:
//
//	Prompt the user to enter their username.
//	Announce its presence to the server, so it
//	  can receive the message log.
//	Start listening to receive messages from
//	  the server.
//	Wait for the user to type messages, and
//	  send them to the server.
func client(serverEndpoint string, port int) {
	var threadGroup sync.WaitGroup
	fmt.Print("Enter your username: ")
	username := readln()
	_ = username // ignore unused variable

	fmt.Println("Connecting to", serverEndpoint)
	conn, err := net.Dial("tcp4", serverEndpoint)

	if err != nil {
		log.Fatal(err)
	}

	defer conn.Close()

	// send server username
	conn.Write([]byte(username))

	threadGroup.Add(1)

	go clientSendMessage(&conn, &threadGroup)
	go clientReceiveMessage(&conn, &threadGroup)

	threadGroup.Wait()

	return
}

func clientReceiveMessage(conn *net.Conn, group *sync.WaitGroup) {
	defer (*conn).Close()
	// reader := bufio.NewReader(*conn)

	for {

		// text, err := reader.ReadString('\n')

		buffer := make([]byte, 1024)

		size, err := (*conn).Read(buffer)

		if err == io.EOF {
			log.Fatal("Server has closed")
			return
		} else if err != nil {
			log.Print(err)
		}

		fmt.Println(strings.TrimSpace(string(buffer[:size])))

	}
}

func clientSendMessage(conn *net.Conn, group *sync.WaitGroup) {
	for {
		text := readln()
		if _, err := (*conn).Write([]byte(text)); err != nil {
			log.Fatal(err)
		}
	}

}

// Main entry point of the program
func main() {
	var port int = 8011
	if len(os.Args) < 2 {
		log.Fatal("Insufficient parameters")
	}
	switch os.Args[1] {

	case "server":
		// If we are running in server mode, listen on
		// the usual port
		server(port)

	case "client":
		// If we are running in client mode, start
		// by connecting to the specified server
		if len(os.Args) != 3 {
			log.Fatal("Insufficient parameters")
		}
		client(os.Args[2], port)

	default:
		log.Fatal("Please use subcommand 'server' or 'client'")
	}
}
