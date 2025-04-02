package main

import (
    "fmt"
    "net"
	"strings"
	"time"
	"log"
	"os"
	
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var (
	server string = "irc.autumnnippert.com"
	port string = "6667"
	nick string = "card"
	user string = "card"
	realname string = "card"

	log_file *os.File

	connected_channels = make(map[string]bool)
	channel_outputs map[string]*tview.TextView

	curr_channel string = ""
	curr_output *tview.TextView
	input *tview.InputField

	close bool = false

	app *tview.Application
)

func mkoutput(title string) *tview.TextView {
	var output *tview.TextView
	output = tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetChangedFunc(func() {
			app.Draw()
			output.ScrollToEnd()
		})
	// scroll to the bottom
	// output.SetScrollToEnd(true)
	output.SetTitle(title).SetBorder(true)
	return output
}

func join(conn net.Conn, channel string) {
	request := fmt.Sprintf("JOIN %s\r\n", channel)
	log.Printf("Sending %s\n", (request))
	conn.Write([]byte(request))
	log.Printf("Joining %s\n", channel)
	channel_outputs[channel] = mkoutput(channel)
	connected_channels[channel] = true
	curr_channel = channel

	switch_output(channel_outputs[channel])
	curr_output.Write([]byte(fmt.Sprintf("[green]Joining %s\n", channel)))
}

func part(conn net.Conn, channel string) {
	request := fmt.Sprintf("PART %s :leaving\r\n", channel)

	conn.Write([]byte(request))
	log.Printf("Sending %s\n", request)
	curr_output.Write([]byte(fmt.Sprintf("[red]Leaving %s", curr_channel)))
	// remove channel from connected channels and outputs
	delete(connected_channels, curr_channel)
	delete(channel_outputs, curr_channel)

	curr_channel = ""
	switch_output(channel_outputs["app"])

	// focus on input
	app.SetFocus(input)
}

func msg(conn net.Conn, message string) {
	request := fmt.Sprintf("PRIVMSG %s :%s\r\n", curr_channel, message)
	log.Printf("Sending %s\n", request)
	fmt.Fprintf(conn, request)
}

func get_time() string {
	return time.Now().Format("02-01-2006 15:04:05")
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

func parse_input(conn net.Conn, line, curr_channel string, stopApp func()) {
	tokens := strings.Fields(line)

	if len(tokens) == 0 {
		log.Println("Empty input")
		return
	}

	if strings.HasPrefix(line, "/") {

		switch tokens[0] {
		case "/raw":
			// send raw command to server
			strings := strings.Join(tokens[1:], " ")
			conn.Write([]byte(fmt.Sprintf("%s\r\n", strings)))
			curr_output.Write([]byte(fmt.Sprintf("[white]%19s | [blue]Sending raw command: %s\n", get_time(), strings)))
			log.Printf("Sending raw command: %s\n", strings)
		case "/quit":
			conn.Write([]byte("QUIT :bye\r\n"))
			stopApp()
			curr_output.Write([]byte("[red]Quitting...\n"))
		case "/join":
			if len(tokens) >= 2 {
				channel := tokens[1]

				// check if channel is already in use
				if _, exists := connected_channels[channel]; exists {
					log.Printf("Already in channel %s\n", channel)
					curr_output.Write([]byte(fmt.Sprintf("[red]Already in channel %s\n", channel)))
					return
				}

				join(conn, channel)
			}else{
				curr_output.Write([]byte("[red]Usage: /join #channel\n"))
			}
		case "/part":
			if curr_channel != "" {
				part(conn, curr_channel)
			}
		case "/switch":
			// switch to another channel or a private message if nick exists
			if len(tokens) >= 2 {
				channel := tokens[1]
				curr_channel = channel
				switch_output(channel_outputs[channel])
			} else {
				curr_output.Write([]byte("[red]Usage: /switch #channel\n"))
			}
		case "/channels":
			// shows the channels you're connected to
			if len(connected_channels) == 0 {
				curr_output.Write([]byte("[red]No channels connected\n"))
			} else {
				curr_output.Write([]byte("[green]Connected channels:\n"))
				for _, channel := range connected_channels {
					curr_output.Write([]byte(fmt.Sprintf("[blue] %s", channel)))
				}
			}
		}
	} else {

		if curr_channel == "" {
			log.Printf("No channel selected, cannot send message\n")
			curr_output.Write([]byte("[red]Join a channel first with /join #channel\n"))
		} else {
			request := fmt.Sprintf("PRIVMSG %s :%s\r\n", curr_channel, line)
			log.Printf("Sending %s\n", request)
			conn.Write([]byte(request))
			curr_output.Write([]byte(fmt.Sprintf("[white]%19s | [blue]To %s: %s\n", get_time(), curr_channel, line)))
		}
	}
}

func init_logging(){
    // Open the log file
    log_file, err := os.OpenFile("irc.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
    if err != nil {
        log.Fatal(err)
    }

    // Set the log output to the file
    log.SetOutput(log_file)
	log.Printf("Starting IRC client\n")
	log.Printf("Logging to irc.log\n")
}

func switch_output(newOutput *tview.TextView) {
	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(newOutput, 0, 1, false).
		AddItem(input, 1, 0, true)

	// create a list of channels
	channels_list := tview.NewList()

	index := 0
	for channel := range channel_outputs {
		channel_index := index
		channels_list.AddItem(channel, "", 0, func() {
			// switch to the selected channel
			switch_output(channel_outputs[channel])
			curr_channel = channel

			channels_list.SetCurrentItem(channel_index)

			curr_output.Write([]byte(fmt.Sprintf("[green]Switching to %s\n", channel)))
		})
		if channel == curr_channel {
			channels_list.SetCurrentItem(index)
		}
		index++
	}
	channels_list.SetBorder(true).SetTitle("Channels").SetTitleAlign(tview.AlignLeft)

	root := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(channels_list, 20, 1, true).
		AddItem(flex, 0, 1, false)

	app.SetRoot(root, true)

	curr_output = newOutput
}

func main() {
    init_logging()
	defer log_file.Close()
    
	app = tview.NewApplication()
	app.EnableMouse(true)

	curr_output = mkoutput("app")
	channel_outputs = make(map[string]*tview.TextView)
	channel_outputs["app"] = curr_output

	input = tview.NewInputField().
		SetLabel("> ").
		SetFieldWidth(0)
		
	switch_output(curr_output)

	// Connect to IRC
	server_str := fmt.Sprintf("%s:%s", server, port)
	conn, err := net.Dial("tcp", server_str)
	if err != nil {
		panic(err)
	}
    
	log.Printf("Connecting to %s as %s\n", server_str, nick)
	fmt.Fprintf(conn, "NICK %s\r\n", nick)
	fmt.Fprintf(conn, "USER %s 0 * :%s\r\n", user, user, realname)

	fmt.Fprintf(conn, "i don't know why but there is some stuff in my tcp so the first command doesn't sent unless this is here :shrug:\n")

    // check if server said username taken or not
    buf := make([]byte, 4096)
    n, err := conn.Read(buf)
    if err != nil {
        panic(err)
    }
    lines := strings.Split(string(buf[:n]), "\r\n")
    for _, line := range lines {
        if strings.Contains(line, "433") {
            curr_output.Write([]byte("[red]Username taken, please try again\n"))
            log.Printf("Username taken, please try again\n")
            app.Stop()
            return
        }
        if strings.Contains(line, "001") {
            curr_output.Write([]byte("[green]Connected to server\n"))
            log.Printf("Connected to server %s\n", server_str)
            break
        }
    }
    
	// --- Reader goroutine
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := conn.Read(buf)
			if err != nil {
				curr_output.Write([]byte("[red]Disconnected from server\n"))
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
					continue
				}
				curr_output.Write([]byte(parse_response(line) + "\n"))
			}
		}
	}()

	
	input.SetDoneFunc(func(key tcell.Key) {
			line := input.GetText()
			input.SetText("")
			log.Printf("Input: %s\n", line)
		
			parse_input(conn, line, curr_channel, app.Stop)
		})

	if err := app.Run(); err != nil {
		panic(err)
	}

	// Close the connection
	conn.Close()

}
