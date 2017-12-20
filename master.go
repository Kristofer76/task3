package main

import (
	"strconv"
  "net"
  "encoding/json"
  "os"
  "fmt"
  "bufio"
)

type managingMsg struct 
{
  Types string
  Dst int
  Data string
}


func response(conn* net.UDPConn, addr* net.UDPAddr, message []byte) {
    _,err := conn.WriteToUDP(message, addr)
    if err != nil {
        fmt.Printf("Have not sent response %v", err)
		}
}

func main() {
  address,err := net.ResolveUDPAddr("udp","127.0.0.1:50000")
  if err != nil {
    panic("Address error")
	}
  
  server, err := net.ListenUDP("udp", address)
  if err != nil {
    panic("Listen error")
	}
  
  scanner := bufio.NewScanner(os.Stdin)
  maintainancePort := 40000
  for 
	{
		msg := managingMsg{}

		//get type
		fmt.Print("Enter type: ")
		scanner.Scan()
		msg.Types = scanner.Text()
		fmt.Printf("%s\n", msg.Types)

		//get Dst
		fmt.Print("Enter Dst: ")
		scanner.Scan()
		msg.Dst, err = strconv.Atoi(scanner.Text())
		fmt.Printf("%d\n", msg.Dst)

		//get Data
		fmt.Print("Enter Data: ")
		scanner.Scan()
		msg.Data = scanner.Text()
		fmt.Printf("%s\n", msg.Data)

		//get recipient 
		fmt.Print("Enter id of a recipient: ")
		scanner.Scan()
		id, err := strconv.Atoi(scanner.Text())
  
		if err != nil {
		    panic("Convertion error")
			}

		neighbour, err := net.ResolveUDPAddr("udp", "127.0.0.1:" + strconv.Itoa(maintainancePort + id))
		b, err := json.Marshal(msg)
		response(server, neighbour, b)
		fmt.Println("")
  }
}
