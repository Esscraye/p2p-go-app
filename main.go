package main

import (
	"fmt"
	"os"
	"p2p-file-transfer/peer"
	"p2p-file-transfer/server"
	"strconv"
	"time"
)

const serverIp = "localhost" //"145.223.79.38"
const serverPort = "8080"

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
		p = peer.NewPeer(port)
	} else {
		fmt.Println("Invalid port number. Port number should be between 1024 and 65535")
		return
	}

	go p.StartServer()

	if err := p.RegisterWithServer(serverIp + ":" + serverPort); err != nil {
		fmt.Println("Error registering peer:", err)
		return
	}

	time.Sleep(800 * time.Millisecond)

	for {
		fmt.Println()
		fmt.Println("Enter a number for a command : ")
		fmt.Println("(1) -- Get Peers")
		fmt.Println("(2) -- Download File Part")
		fmt.Println("(3) -- Download File")
		fmt.Println("(4) -- Update File Parts")
		fmt.Println("(5) -- Query File Parts")
		fmt.Println("(6) -- Exit")
		fmt.Println("(7) -- Split File Into Parts")
		fmt.Println("(8) -- Combine File Parts")
		fmt.Println()

		var choice int
		fmt.Scan(&choice)
		fmt.Println()
		fmt.Println("---------------------------------")

		switch choice {
		case 1:
			peers, err := p.GetPeersFromServer(serverIp + ":" + serverPort)
			if err != nil {
				fmt.Println("Error getting peers:", err)
			} else {
				fmt.Println("Peers:")
				fmt.Println("~~~~~~")
				i := 1
				for _, peer := range peers {
					fmt.Printf("(%d) <%s:%s>\n", i, peer.PublicIP, peer.Addr)
					i++
				}
				fmt.Println("~~~~~~")
			}
		case 2:
			fileName := "d7061c7b0c63301f41d30cfb1aad4ba191f9d828981e4bd9ec8744f44a8eb57a_0"
			err := p.DownloadFilePart(fileName, "localhost", "4000")
			if err != nil {
				fmt.Println("Error downloading file part:", err)
				return
			}
		case 3:
			var filename string
			var nbParts int
			fmt.Println("Enter filename:")
			fmt.Scan(&filename)
			fmt.Println("Enter number of parts:")
			fmt.Scan(&nbParts)
			if err := p.DownloadFile(filename, nbParts, serverIp+":"+serverPort); err != nil {
				fmt.Println("Error downloading file:", err)
			} else {
				fmt.Printf("Successfully downloaded file")
			}
		case 4:
			// Informer le serveur des parties de fichiers que le peer possède
			if err := p.UpdateFilePartsOnServer(serverIp + ":" + serverPort); err != nil {
				fmt.Println("Error updating file parts on server:", err)
				return
			}
			fmt.Println("Successfully updated file parts on server")
		case 5:
			fileName := "d7061c7b0c63301f41d30cfb1aad4ba191f9d828981e4bd9ec8744f44a8eb57a_0"
			fileParts, err := p.QueryFilePartsFromServer(serverIp+":"+serverPort, fileName)
			if err != nil {
				fmt.Println("Error querying file parts:", err)
				return
			}
			fmt.Println("File parts:", fileParts)
		case 6:
			fmt.Println("Exiting...")
			if err := p.RemoveFromServer(serverIp + ":" + serverPort); err != nil {
				fmt.Println("Error removing peer:", err)
			}
			return
		case 7:
			parts, err := p.SplitFileIntoParts("./peer_directory/complete_files/test.txt")
			if err != nil {
				fmt.Println("Error splitting file into parts:", err)
				return
			}
			fmt.Println("File parts:", parts)
		case 8:
			if err := p.CombineFileParts("d7061c7b0c63301f41d30cfb1aad4ba191f9d828981e4bd9ec8744f44a8eb57a", "peer_directory/complete_files/file.txt"); err != nil {
				fmt.Println("Error combining file parts:", err)
				return
			}
			fmt.Println("File combined successfully")
		default:
			fmt.Println("Invalid choice")
		}
		fmt.Println("---------------------------------")
	}
}
