package peer

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const (
	partSize   = 1 * 1024 * 1024 // 1Mo
	storageDir = "file_parts"    // directory to store file part
)

type Peer struct {
	Addr      string            `json:"addr"`
	FileParts map[string]string `json:"file_parts"`
}

func NewPeer(addr string) *Peer {
	return &Peer{Addr: addr}
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
	data := map[string]string{"addr": p.Addr}
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
	http.HandleFunc("/upload", p.handleUpload)
	http.HandleFunc("/download", p.handleDownload)
	http.ListenAndServe(p.Addr, nil)
}

func (p *Peer) handleUpload(w http.ResponseWriter, r *http.Request) {
	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Error retrieving the file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	fileBytes, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "Error reading the file", http.StatusInternalServerError)
		return
	}

	dirPath := filepath.Join(p.Addr+"_files", "file_parts")
	filePath := filepath.Join(dirPath, r.FormValue("filename"))
	newFile, err := os.Create(filePath)
	if err != nil {
		http.Error(w, "Error creating the file", http.StatusInternalServerError)
		return
	}
	defer newFile.Close()

	if _, err := newFile.Write(fileBytes); err != nil {
		http.Error(w, "Error writing the file", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (p *Peer) handleDownload(w http.ResponseWriter, r *http.Request) {
	filename := r.URL.Query().Get("filename")
	dirPath := p.Addr + "_files"
	filePath := filepath.Join(dirPath, filename)
	file, err := os.Open(filePath)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}
	defer file.Close()

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeFile(w, r, filePath)
}

func (p *Peer) UploadFile(filePath, targetAddr string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return err
	}

	_, err = io.Copy(part, file)
	if err != nil {
		return err
	}
	writer.WriteField("filename", filepath.Base(filePath))
	writer.Close()

	req, err := http.NewRequest("POST", fmt.Sprintf("http://%s/upload", targetAddr), body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %v", err)
	}
	fmt.Println("bodyBytes: ", bodyBytes)
	bodyString := string(bodyBytes)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to upload file: %s, response: %s", resp.Status, bodyString)
	}

	return nil
}

func (p *Peer) DownloadFile(filename, sourceAddr string) error {
	resp, err := http.Get(fmt.Sprintf("http://%s/download?filename=%s", sourceAddr, filename))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download file: %s", resp.Status)
	}

	dirPath := p.Addr + "_files"
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

	//TODO: Update the server with the new file part
	return nil
}

// Ajoutez une méthode pour interroger le serveur pour obtenir les peers qui possèdent certaines parties de fichiers :
func (p *Peer) QueryFilePartsFromServer(serverAddr, fileName string) (map[string]string, error) {
	data := map[string]string{
		"file_name": fileName,
		"addr":      p.Addr,
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

func (p *Peer) UpdateFilePartsOnServer(serverAddr string, fileParts map[string]string) error {
	data := Peer{
		Addr:      p.Addr,
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
