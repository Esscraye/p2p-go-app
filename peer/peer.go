package peer

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const (
	partSize   = 1 * 1024 * 1024 // 1Mo
	storageDir = "file_parts"    // directory to store file part
)

type Peer struct {
	Addr      string            `json:"addr"`
	PublicIP  string            `json:"public_ip"`
	FileParts map[string]string `json:"file_parts"`
}

func NewPeer(addr string) *Peer {
	publicIP, err := getPublicIP()
	if err != nil {
		log.Printf("Error getting public IP: %s", err)
	}
	return &Peer{
		Addr:     addr,
		PublicIP: publicIP,
	}
}

// SplitFileIntoParts divise un fichier en parties et les stocke dans le dossier de stockage.
func (p *Peer) SplitFileIntoParts(filePath string) (map[string]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return nil, err
	}

	fileSize := fileInfo.Size()
	fileHash := sha256.New()
	if _, err := io.Copy(fileHash, file); err != nil {
		return nil, err
	}
	fileHashStr := hex.EncodeToString(fileHash.Sum(nil))

	parts := make(map[string]string)
	for start := int64(0); start < fileSize; start += partSize {
		end := start + partSize
		if end > fileSize {
			end = fileSize
		}
		partFileName := fmt.Sprintf("%s_%d", fileHashStr, len(parts))
		partFilePath := filepath.Join(storageDir, partFileName)

		partFile, err := os.Create(partFilePath)
		if err != nil {
			return nil, err
		}
		defer partFile.Close()

		if _, err := file.Seek(start, io.SeekStart); err != nil {
			return nil, err
		}
		if _, err := io.CopyN(partFile, file, end-start); err != nil {
			return nil, err
		}

		partHash := sha256.New()
		if _, err := io.Copy(partHash, partFile); err != nil {
			return nil, err
		}
		partHashStr := hex.EncodeToString(partHash.Sum(nil))
		parts[partFileName] = partHashStr
	}

	return parts, nil
}

// CombineFileParts combine les parties de fichiers en un fichier complet.
func (p *Peer) CombineFileParts(fileHashStr string, outputFilePath string) error {
	outputFile, err := os.Create(outputFilePath)
	if err != nil {
		return err
	}
	defer outputFile.Close()

	for i := 0; ; i++ {
		partFileName := fmt.Sprintf("%s_%d", fileHashStr, i)
		partFilePath := filepath.Join(storageDir, partFileName)
		partFile, err := os.Open(partFilePath)
		if os.IsNotExist(err) {
			break
		} else if err != nil {
			return err
		}
		defer partFile.Close()

		if _, err := io.Copy(outputFile, partFile); err != nil {
			return err
		}
	}

	return nil
}

func (p *Peer) RegisterWithServer(serverAddr string) error {
	data := map[string]string{"addr": p.Addr, "public_ip": p.PublicIP}
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	resp, err := http.Post(fmt.Sprintf("http://%s/register-peer", serverAddr), "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to register peer: %s", resp.Status)
	}
	return nil
}

func getPublicIP() (string, error) {
	resp, err := http.Get("https://api.ipify.org?format=json")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	return result["ip"], nil
}

func (p *Peer) RemoveFromServer(serverAddr string) error {
	data := map[string]string{"addr": p.Addr}
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("http://%s/remove-peer", serverAddr), bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to remove peer: %s", resp.Status)
	}
	return nil
}

func (p *Peer) GetPeersFromServer(serverAddr string) (map[string]Peer, error) {
	resp, err := http.Get(fmt.Sprintf("http://%s/get-peers", serverAddr))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get peers: %s", resp.Status)
	}
	var peers map[string]Peer
	if err := json.NewDecoder(resp.Body).Decode(&peers); err != nil {
		return nil, err
	}
	return peers, nil
}

func (p *Peer) StartServer() {
	http.HandleFunc("/download", p.handleDownload)
	http.ListenAndServe(":"+p.Addr, nil)
}

func (p *Peer) handleDownload(w http.ResponseWriter, r *http.Request) {
	filename := r.URL.Query().Get("filename")
	if filename == "" {
		http.Error(w, "Filename is required", http.StatusBadRequest)
		return
	}

	dirPath := filepath.Join("file_parts")
	filePath := filepath.Join(dirPath, filename)

	log.Printf("Trying to open file: %s", filePath)

	file, err := os.Open(filePath)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}
	defer file.Close()

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeFile(w, r, filePath)

	log.Printf("File sent: %s", filePath)
}

func (p *Peer) DownloadFilePart(filename, sourceIP, sourceAddr string) error {
	resp, err := http.Get(fmt.Sprintf("http://%s:%s/download?filename=%s", sourceIP, sourceAddr, filename))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	log.Printf("Downloading file part: %s from %s:%s", filename, sourceIP, sourceAddr)
	log.Printf("Response status: %s", resp.Status)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download file: %s", resp.Status)
	}

	dirPath := filepath.Join("downloads")
	if err := os.MkdirAll(dirPath, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}
	filePath := filepath.Join(dirPath, filename)
	out, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	return nil
}

func (p *Peer) DownloadFile(filename string, nbParts int, serverAddr string) error {
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errs []error

	for i := 0; i < nbParts; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			partFileName := fmt.Sprintf("%s_%d", filename, i)
			fileParts, err := p.QueryFilePartsFromServer(serverAddr, partFileName)
			if err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("failed to query file parts for %s: %v", partFileName, err))
				mu.Unlock()
				return
			}

			if len(fileParts) == 0 {
				mu.Lock()
				errs = append(errs, fmt.Errorf("no peers found for file part: %s", partFileName))
				mu.Unlock()
				return
			}

			// Sélectionner un pair au hasard
			keys := make([]string, 0, len(fileParts))
			for key := range fileParts {
				keys = append(keys, key)
			}
			randomKey := keys[rand.Intn(len(keys))]
			parts := strings.Split(randomKey, ":")
			if len(parts) != 2 {
				mu.Lock()
				errs = append(errs, fmt.Errorf("invalid peer address format: %s", randomKey))
				mu.Unlock()
				return
			}
			ip, port := parts[0], parts[1]

			ip = "localhost" // TODO: make open port and remove this line in production

			// Télécharger la partie de fichier
			if err := p.DownloadFilePart(partFileName, ip, port); err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("failed to download file part %s from %s:%s: %v", partFileName, ip, port, err))
				mu.Unlock()
				return
			}

			log.Printf("Successfully downloaded file part: %s from %s:%s", partFileName, ip, port)
		}(i)
	}

	wg.Wait()

	if len(errs) > 0 {
		for _, err := range errs {
			fmt.Println(err)
		}
		return fmt.Errorf("failed to download some file parts")
	}

	//TODO: update file parts on server
	return nil
}

// Ajoutez une méthode pour interroger le serveur pour obtenir les peers qui possèdent certaines parties de fichiers :
func (p *Peer) QueryFilePartsFromServer(serverAddr, fileName string) (map[string]string, error) {
	data := map[string]string{
		"file_name": fileName,
		"addr_ip":   p.PublicIP + ":" + p.Addr,
	}
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	resp, err := http.Post(fmt.Sprintf("http://%s/query-file-parts", serverAddr), "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to query file parts: %s", resp.Status)
	}
	var response map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}
	return response, nil
}

// ScanStorageDir scanne le dossier de stockage et récupère les informations sur les fichiers et leurs parties.
func (p *Peer) ScanStorageDir() (map[string]string, error) {
	files := make(map[string]string)

	err := filepath.Walk("file_parts", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			fileName := info.Name()
			parts := strings.Split(fileName, "_")
			if len(parts) == 2 {
				fileHash := parts[0]
				partIndex := parts[1]

				partFile, err := os.Open(path)
				if err != nil {
					return err
				}
				defer partFile.Close()

				partHash := sha256.New()
				if _, err := io.Copy(partHash, partFile); err != nil {
					return err
				}
				partHashStr := hex.EncodeToString(partHash.Sum(nil))

				// Ajouter toutes les parties du fichier
				files[fileHash+"_"+partIndex] = partHashStr
			}
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return files, nil
}

func (p *Peer) UpdateFilePartsOnServer(serverAddr string) error {
	fileParts, err := p.ScanStorageDir()

	data := Peer{
		Addr:      p.Addr,
		PublicIP:  p.PublicIP,
		FileParts: fileParts,
	}
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	resp, err := http.Post(fmt.Sprintf("http://%s/update-peer-file-parts", serverAddr), "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to update file parts: %s", resp.Status)
	}
	return nil
}
