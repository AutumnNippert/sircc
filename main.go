package main

import (
	"bufio"
    "fmt"
    "net"
    "io"
	"os"
	"strings"
	"time"
)

var curr_channel string = ""
var close bool = false

func join(conn net.Conn, channel string) {
	fmt.Fprintf(conn, "JOIN %s\r\n", channel)
}
func part(conn net.Conn, channel string, message string) {
	if message == "" {
		message = "Bye"
	}
	fmt.Fprintf(conn, "PART %s :%s\r\n", channel, message)
}
func msg(conn net.Conn, channel string, message string) {
	fmt.Fprintf(conn, "PRIVMSG %s :%s\r\n", channel, message)
}

func get_time() string {
	// get current time
	t := time.Now()
	// format it to 15:04:05
	return t.Format("02-01-2006 15:04:05")
}

func parse_response(line string) {
	tokens := strings.Fields(line)
	// So basically the first token is the user or the server
	user := tokens[0]
	// try split by !
	if strings.Contains(user, "!") {
		user = strings.Split(user, "!")[0][1:]
	}

	if tokens[1] == "PRIVMSG" {
		tokens[3] = strings.Trim(tokens[3], ":")
		
		fmt.Printf("%19s | %s: %s\n", get_time(), user, strings.Join(tokens[3:], " "))
	
	} else {
		fmt.Printf("%19s | %s\n", get_time(), line)
	}
}

func handle_user_input(conn net.Conn, line string) {
	tokens := strings.Fields(line)
	if len(tokens) == 0 {
		return
	}

	// parse commands
	switch tokens[0] {
	case "/quit":
		fmt.Println("Disconnecting...")
		part(conn, curr_channel, "Bye")
		close = true
		return
	case "/join":
		if len(tokens) < 2 {
			fmt.Println("Usage: /join <channel>")
			return
		}
		channel := tokens[1]
		join(conn, channel)
		curr_channel = channel
		return
	case "/part":
		if curr_channel == "" {
			fmt.Println("You are not in a channel.")
			return
		}
		part(conn, curr_channel, "")
		curr_channel = ""
		return
	}
	// if no command, send as message

	if curr_channel == "" {
		fmt.Println("You are not in a channel. Use /join <channel> to join a channel.")
		return
	}
	fmt.Fprintf(conn, "PRIVMSG %s :%s\r\n", curr_channel, line)
}

func main() {
    conn, err := net.Dial("tcp", "irc.autumnnippert.com:6667")
    if err != nil {
        panic(err)
    }
    defer conn.Close()

    fmt.Fprintf(conn, "NICK card\r\n")
	fmt.Fprintf(conn, "USER card 0 * :card\r\n")

	// --- Reading from IRC server
	go func() {
		scanner := bufio.NewScanner(conn)
		for scanner.Scan() {
			line := scanner.Text()
			// parse the line
			parse_response(line)
		}

		if err := scanner.Err(); err != nil && err != io.EOF {
			fmt.Fprintln(os.Stderr, "Error reading from server:", err)
		}
	}()

	// --- Writing user input to server
	stdin := bufio.NewScanner(os.Stdin)
	for stdin.Scan() {
		text := stdin.Text()
		if text == "" {
			continue
		}
		// handle user input
		handle_user_input(conn, text)
		if close {
			break
		}
	}
}
