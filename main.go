package main

import (
    "fmt"
    "net"
	"strings"
	"time"
	
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var server string = "irc.myserver.com"
var port string = "6667"
var nick string = "test"
var user string = "test"
var realname string = "test"

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

func parse_response(line string) string {
	tokens := strings.Fields(line)
	if len(tokens) < 2 {
		return fmt.Sprintf("%19s | %s\n", get_time(), line)
	}

	user := tokens[0]
	if strings.Contains(user, "!") {
		user = strings.Split(user, "!")[0][1:]
	}
	if strings.HasPrefix(user, ":") {
		user = user[1:]
	}

	if tokens[1] == "PRIVMSG" && len(tokens) >= 4 {
		msg := strings.Join(tokens[3:], " ")
		msg = strings.TrimPrefix(msg, ":")
		return fmt.Sprintf("%19s | [green]%s[-]: %s", get_time(), user, msg)
	}
	return fmt.Sprintf("%19s | %s", get_time(), line)
}

func parse_input(conn net.Conn, line, curr_channel string, stopApp func()) (string, []string) {
	var output []string
	tokens := strings.Fields(line)

	if len(tokens) == 0 {
		return curr_channel, nil
	}

	switch tokens[0] {
	case "/quit":
		conn.Write([]byte("QUIT :bye\r\n"))
		stopApp()
		output = append(output, "[yellow]Quitting...")
	case "/join":
		if len(tokens) >= 2 {
			channel := tokens[1]
			conn.Write([]byte(fmt.Sprintf("JOIN %s\r\n", channel)))
			output = append(output, fmt.Sprintf("[green]Joining %s", channel))
			return channel, output
		}
		output = append(output, "[red]Usage: /join #channel")
	case "/part":
		if curr_channel != "" {
			conn.Write([]byte(fmt.Sprintf("PART %s :leaving\r\n", curr_channel)))
			output = append(output, fmt.Sprintf("[yellow]Parted %s", curr_channel))
			return "", output
		}
		output = append(output, "[red]You're not in a channel")
	default:
		if curr_channel == "" {
			output = append(output, "[red]Join a channel first with /join #channel")
		} else {
			conn.Write([]byte(fmt.Sprintf("PRIVMSG %s :%s\r\n", curr_channel, line)))
			output = append(output, fmt.Sprintf("[white]%19s | [blue]To %s: %s", get_time(), curr_channel, line))
		}
	}

	return curr_channel, output
}

var output *tview.TextView

func main() {
	app := tview.NewApplication()
	app.EnableMouse(true)

	output = tview.NewTextView()

	output.
		SetDynamicColors(true).
		SetScrollable(true).
		SetChangedFunc(func() {
			app.Draw()
			output.ScrollToEnd()
		})

	input := tview.NewInputField().
		SetLabel("> ").
		SetFieldWidth(0)

	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(output, 0, 1, false).
		AddItem(input, 1, 0, true)

	app.SetRoot(flex, true)

	// Connect to IRC
	server_str := fmt.Sprintf("%s:%s", server, port)
	conn, err := net.Dial("tcp", server_str)
	if err != nil {
		panic(err)
	}

	fmt.Fprintf(conn, "NICK %s\r\n", nick)
	fmt.Fprintf(conn, "USER %s 0 * :%s\r\n", user, user, realname)

	// --- Reader goroutine
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := conn.Read(buf)
			if err != nil {
				output.Write([]byte("[red]Disconnected from server\n"))
				app.Stop()
				return
			}
			lines := strings.Split(string(buf[:n]), "\r\n")
			for _, line := range lines {
				if line == "" {
					continue
				}
				if strings.HasPrefix(line, "PING") {
					// Respond to keepalive
					pong := "PONG" + line[4:] + "\r\n"
					conn.Write([]byte(pong))
				}
				output.Write([]byte(parse_response(line) + "\n"))
			}
		}
	}()

	
	input.SetDoneFunc(func(key tcell.Key) {
			line := input.GetText()
			input.SetText("")
		
			newChannel, feedback := parse_input(conn, line, curr_channel, app.Stop)
			curr_channel = newChannel
		
			for _, msg := range feedback {
				output.Write([]byte(msg + "\n"))
			}
		})

	if err := app.Run(); err != nil {
		panic(err)
	}

}
