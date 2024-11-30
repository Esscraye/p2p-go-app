package peer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
)

type Peer struct {
	Addr string
}

func NewPeer(addr string) *Peer {
	return &Peer{Addr: addr}
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

func (p *Peer) GetPeersFromServer(serverAddr string) (map[string]string, error) {
	resp, err := http.Get(fmt.Sprintf("http://%s/get-peers", serverAddr))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get peers: %s", resp.Status)
	}
	var peers map[string]string
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

	dirPath := p.Addr + "_files"
	if err := os.MkdirAll(dirPath, os.ModePerm); err != nil {
		http.Error(w, "Error creating directory", http.StatusInternalServerError)
		return
	}
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
	return err
}
