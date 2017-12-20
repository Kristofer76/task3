package main

import (
	"strconv"
  "net"
  "time"
  "encoding/json"
  "os"
  "fmt"
  "sync"
)

type managingMsg struct 
{
  Types string
  Dst int
  Data string
}

type message struct
{
  Types string
  Sender int
  Origin int
  Dst int
  Data string
}

type Node struct 
{
	id   int
	port int
  managing_port int
}

func response(conn* net.UDPConn, addr* net.UDPAddr, message []byte) {
    _,err := conn.WriteToUDP(message, addr)
    if err != nil {
        fmt.Printf("Have not sent response %v", err)
	}
}

type Graph map[Node]Node
type conMap map[int]*net.UDPAddr

var base_port int
var maintainance_port int
var msgId int64 = 1

var token_delay time.Duration = time.Millisecond * 100
var token_timeout time.Duration = time.Millisecond * 100

var wg sync.WaitGroup

func CheckError(err error) {
   if err  != nil {
	   fmt.Println("Error: " , err)
	   os.Exit(0)
   }
}

func (n Node) Port() int {
	return n.port
}

func create_node(a int) Node {
	node := Node {
		id:   a,
		port: a + base_port,
    		managing_port: a + maintainance_port,
	}
	return node
}

func (g Graph) get_current_node(id int) (Node, bool) {
	node := create_node(id)
	_, ret := g[node]
	return node, ret
}

func (g Graph) neighbours(id int) (Node, bool) {
	node := create_node(id)
	neighbour, ret := g[node]
	return neighbour, ret
}

func (g Graph) add_edge(a, b Node) {
	g[a] = b
}

func create_graph(n, port int, managing_port int) Graph {
	if n<=0 {
		panic("nodes <= 0")
	}

	base_port = port
  maintainance_port = managing_port
	g := make(Graph)

	for i:=1; i<n; i++ {
		g.add_edge(create_node(i-1), create_node(i))
	}
  g.add_edge(create_node(n-1), create_node(0))

	return g
}

// permit attempts to obtain a permit within the allotted time
func permit(chan1 chan int, timeout time.Duration) bool {
    cancel := make(chan struct{}, 1)
    t := time.AfterFunc(timeout, func() {
        close(cancel)
    })
    defer t.Stop()
    select 
		{
		  case <- cancel:
		     return false
		  case <- chan1:
		      return true
    }
}

func client_step(myNode Node, neighbour Node){
  myId := myNode.id
  
  address, err := net.ResolveUDPAddr("udp","127.0.0.1:" + strconv.Itoa(myNode.port))
  if err != nil {
    panic("Address error")
  }
  
  server, err := net.ListenUDP("udp", address)
  if err != nil {
    panic("Listen error")
  }
  
  maintainance_address, err := net.ResolveUDPAddr("udp","127.0.0.1:" + strconv.Itoa(myNode.managing_port))
  if err != nil {
    panic("Mintainance address error")
  }
  
  maintainance_server, err := net.ListenUDP("udp", maintainance_address)
  if err != nil {
    panic("Listen maintainance error")
  }

  neighbour_address, err := net.ResolveUDPAddr("udp", "127.0.0.1:" + strconv.Itoa(neighbour.port))
  
  buf := make([]byte, 1500*1024)
  send_buf := make([]message, 0)

  is_token := false
  if myId == 0 {
    is_token = true
  }

  sync_chan0 := make(chan int)
  sync_chan := make(chan int)
  loose_token := false
  
  
	//messages
  go func() {
    for {
      n,_,err := server.ReadFromUDP(buf)
      if err != nil {
        panic("Read error")
      }
      
      tmp_msg := message{}
      err2 := json.Unmarshal(buf[0:n], &tmp_msg)
      if !loose_token {
        if err2 == nil {
          if tmp_msg.Types == "send" {
            if myId == tmp_msg.Dst {
              fmt.Printf("node %d: received token from node %d with data from %d (data: '%s'), sending token to %d\n", myId, tmp_msg.Sender, tmp_msg.Origin, tmp_msg.Data, neighbour.id)
              tmp_msg.Types = "notification"
              tmp_msg.Dst = tmp_msg.Origin
              tmp_msg.Origin = myId
              tmp_msg.Sender = myId
              tmp_msg.Data = ""
              b, err3 := json.Marshal(tmp_msg)
	      CheckError(err3)
              time.Sleep(token_delay)
              response(server, neighbour_address, b)
            } else {
              tmp_msg.Sender = myId
              b, err3 := json.Marshal(tmp_msg)
	      CheckError(err3)
              time.Sleep(token_delay)
              response(server, neighbour_address, b)
            }
          } else if tmp_msg.Types == "notification" {
            if myId != tmp_msg.Dst {
              tmp_msg.Sender = myId
              b, err3 := json.Marshal(tmp_msg)
	      CheckError(err3)
              time.Sleep(token_delay)
              response(server, neighbour_address, b)
            } else {
              fmt.Printf("node %d: received token from node %d with delivery confirmation from %d\n", myId, tmp_msg.Sender, tmp_msg.Origin)
              sync_chan <- 1
            }
          } else if tmp_msg.Types == "empty" {
            fmt.Printf("node %d: received empty token from node %d\n", myId, tmp_msg.Sender)
            is_token = true
          } else if tmp_msg.Types == "new-token" {
            if myId == 0 {
              is_token = true
              fmt.Printf("node %d: received new-token message again, generating new token\n", myId)
            } else {
              if is_token {
                sync_chan <- 1
              }
              is_token = false
              fmt.Printf("node %d: received new-token message from node %d, resetting all processes\n", myId, tmp_msg.Sender)
              b, err3 := json.Marshal(tmp_msg)
	      CheckError(err3)
              time.Sleep(token_delay)
              response(server, neighbour_address, b)
            }
            
          }
          if myId == 0 {
            sync_chan0 <- 1
          }
        }
      } else {
        loose_token = false
      }
    }
  }() 


	//managing messages
  go func(){
    for {
      n,_,err := maintainance_server.ReadFromUDP(buf)
      if err != nil {
        panic("Maintainance read error")
      }
      
      tmp_msg := managingMsg{}
      err2 := json.Unmarshal(buf[0:n], &tmp_msg)
      if err2 == nil {
        fmt.Printf("node %d: received service message: %s\n", myId, buf[0:n])
        if tmp_msg.Types == "send" {
          tmp := message{}
          tmp.Data = tmp_msg.Data
          tmp.Dst = tmp_msg.Dst
          tmp.Origin = myId
          tmp.Sender = myId
          tmp.Types = "send"
          send_buf = append(send_buf, tmp)
        } else if tmp_msg.Types == "drop" {
          loose_token = true
        }
      }
    }
  }()
  
  //loose or empty token
  go func() {
    for {
      if !is_token {
        if myId == 0 {
          flag := permit(sync_chan0, token_timeout)
          if !flag {
            confirmed := false
            for !confirmed {
              //token loose
              fmt.Printf("node %d: token timeout, initializing process of creating new one\n", myId)
              tmp_msg := message{}
              tmp_msg.Types = "new-token"
              tmp_msg.Data = ""
              tmp_msg.Origin = myId
              tmp_msg.Sender = myId
              tmp_msg.Dst = myId
              b, err3 := json.Marshal(tmp_msg)
	      CheckError(err3)
              response(server, neighbour_address, b)
              confirmed = permit(sync_chan0, token_timeout)
              if is_token && confirmed {
                confirmed = true
              } else {
                confirmed = false
              }
            }
          }
        }
      } else {
        confirmed := false
        for index, element := range send_buf {
					 confirmed = false
					 for !confirmed {
              b, err3 := json.Marshal(element)
	      CheckError(err3)
              response(server, neighbour_address, b)
              confirmed = permit(sync_chan, token_timeout)
              if myId == 0 {
                <- sync_chan0
              }
              if !is_token {
                break
              }
           }
           if !is_token {
              send_buf = send_buf[index: ]
              break
          	}
        }
        if is_token {
          tmp_msg := message{}
          tmp_msg.Data = ""
          tmp_msg.Types = "empty"
          tmp_msg.Origin = myId
          tmp_msg.Sender = myId
          tmp_msg.Dst = neighbour.id
          b, err3 := json.Marshal(tmp_msg)
	  CheckError(err3)
          time.Sleep(token_delay)
          response(server, neighbour_address, b)
          is_token = false
          send_buf = make([]message, 0)
        }
      }
    }
  }()
}

func main() {
  num_nodes := 0

  if len(os.Args) > 1 {
    n, err := strconv.Atoi(os.Args[1])
    if err != nil {
      panic(err)
    } else {
      num_nodes = n
    }
  }

  if len(os.Args) > 2 {
    delay, err := strconv.Atoi(os.Args[2])
    if err != nil {
      panic(err)
    } else {
      token_delay = time.Millisecond * time.Duration(delay)
    }
  }

  token_timeout = token_delay * time.Duration(float64(num_nodes) * 2.5)
	config_graph := create_graph(num_nodes, 20000, 50000)
  
  for res, _ := range config_graph {
    neighbours, ret := config_graph.neighbours(res.id)
    if ret {
      go client_step(res, neighbours)
    } else {
      panic("No neighbours")
    }
  }

  wg.Add(1)
  wg.Wait()
}
