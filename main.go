package main

import (
	"fmt"
	"os"
	"p2p-file-transfer/peer"
	"p2p-file-transfer/server"
	"strconv"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go <port>")
		return
	}

	port := os.Args[1]
	portNum, err := strconv.Atoi(port)
	var p *peer.Peer

	if err != nil || portNum == 8080 {
		srv := server.NewServer()
		srv.Start()
		return
	}
	if portNum > 1024 && portNum < 65535 {
		p = peer.NewPeer(":" + port)
	} else {
		fmt.Println("Invalid port number. Port number should be between 1024 and 65535")
		return
	}

	go p.StartServer()

	if err := p.RegisterWithServer("localhost:8080"); err != nil {
		fmt.Println("Error registering peer:", err)
		return
	}

	time.Sleep(800 * time.Millisecond)

	for {
		fmt.Println()
		fmt.Println("Enter a number for a command : ")
		fmt.Println("(1) -- Get Peers")
		fmt.Println("(2) -- Upload File")
		fmt.Println("(3) -- Download File")
		fmt.Println("(4) -- Exit")
		fmt.Println()

		var choice int
		fmt.Scan(&choice)
		fmt.Println()
		fmt.Println("---------------------------------")

		switch choice {
		case 1:
			peers, err := p.GetPeersFromServer("localhost:8080")
			if err != nil {
				fmt.Println("Error getting peers:", err)
			} else {
				fmt.Println("Peers:")
				fmt.Println("~~~~~~")
				i := 1
				for _, addr := range peers {
					fmt.Printf("(%d) <%s>\n", i, addr)
					i++
				}
				fmt.Println("~~~~~~")
			}
		case 2:
			var filePath, targetAddr string
			fmt.Println("Enter file path:")
			fmt.Scan(&filePath)
			fmt.Println("Enter target peer address:")
			fmt.Scan(&targetAddr)
			if err := p.UploadFile(filePath, ":"+targetAddr); err != nil {
				fmt.Println("Error uploading file:", err)
			} else {
				fmt.Printf("Successfully uploaded file to peer %s\n", targetAddr)
			}
		case 3:
			var filename, sourceAddr string
			fmt.Println("Enter filename:")
			fmt.Scan(&filename)
			fmt.Println("Enter source peer address:")
			fmt.Scan(&sourceAddr)
			if err := p.DownloadFile(filename, ":"+sourceAddr); err != nil {
				fmt.Println("Error downloading file:", err)
			} else {
				fmt.Printf("Successfully downloaded file from peer %s\n", sourceAddr)
			}
		case 4:
			fmt.Println("Exiting...")
			if err := p.RemoveFromServer("localhost:8080"); err != nil {
				fmt.Println("Error removing peer:", err)
			}
			return
		default:
			fmt.Println("Invalid choice")
		}
		fmt.Println("---------------------------------")
	}
}
