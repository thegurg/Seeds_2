package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/mdns"
)

//go:embed index.html
var indexHTML embed.FS

// Peer represents a discovered node on the network
type Peer struct {
	Name     string    `json:"name"`
	IP       string    `json:"ip"`
	Port     int       `json:"port"`
	LastSeen time.Time `json:"last_seen"`
}

// PeerRegistry maintains the list of discovered peers
type PeerRegistry struct {
	peers map[string]Peer
	mutex sync.RWMutex
}

// ConnectedClients tracks active HTTP connections
type ConnectedClients struct {
	clients map[string]time.Time
	mutex   sync.RWMutex
}

// Server holds the application state
type Server struct {
	port             int
	sharedDir        string
	downloadsDir     string
	peerRegistry     *PeerRegistry
	connectedClients *ConnectedClients
	mdnsServer       *mdns.Server
	hostname         string
}

var (
	portFlag = flag.Int("port", 0, "Port to listen on (0 for auto-assign)")
)

// Buffer size for file streaming (32KB)
const bufferSize = 32 * 1024

// HTML template for the web UI
const htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>File Transfer</title>
    <style>
        * { box-sizing: border-box; margin: 0; padding: 0; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, sans-serif;
            background: #1a1a2e;
            color: #eee;
            padding: 20px;
            min-height: 100vh;
        }
        .container {
            max-width: 800px;
            margin: 0 auto;
        }
        h1 {
            text-align: center;
            margin-bottom: 30px;
            color: #00d4aa;
        }
        .section {
            background: #16213e;
            border-radius: 12px;
            padding: 20px;
            margin-bottom: 20px;
            border: 1px solid #0f3460;
        }
        .section-title {
            font-size: 18px;
            margin-bottom: 15px;
            color: #00d4aa;
            display: flex;
            align-items: center;
            gap: 8px;
        }
        .drop-zone {
            border: 3px dashed #0f3460;
            border-radius: 8px;
            padding: 40px;
            text-align: center;
            transition: all 0.3s;
            cursor: pointer;
            background: #1a1a2e;
        }
        .drop-zone:hover, .drop-zone.dragover {
            border-color: #00d4aa;
            background: #0f3460;
        }
        .drop-zone i {
            font-size: 48px;
            margin-bottom: 10px;
            display: block;
        }
        input[type="file"] { display: none; }
        .btn {
            background: #00d4aa;
            color: #1a1a2e;
            border: none;
            padding: 10px 20px;
            border-radius: 6px;
            cursor: pointer;
            font-weight: bold;
            transition: all 0.3s;
        }
        .btn:hover { background: #00b894; }
        .btn:disabled { background: #666; cursor: not-allowed; }
        .progress-container {
            margin-top: 15px;
            display: none;
        }
        .progress-bar {
            width: 100%;
            height: 20px;
            background: #0f3460;
            border-radius: 10px;
            overflow: hidden;
        }
        .progress-fill {
            height: 100%;
            background: linear-gradient(90deg, #00d4aa, #00b894);
            width: 0%;
            transition: width 0.3s;
        }
        .progress-text {
            text-align: center;
            margin-top: 5px;
            font-size: 14px;
            color: #888;
        }
        .peers-list {
            display: flex;
            flex-direction: column;
            gap: 10px;
        }
        .peer-item {
            background: #1a1a2e;
            padding: 15px;
            border-radius: 8px;
            display: flex;
            justify-content: space-between;
            align-items: center;
            border: 1px solid #0f3460;
            transition: all 0.3s;
        }
        .peer-item:hover {
            border-color: #00d4aa;
        }
        .peer-info {
            display: flex;
            flex-direction: column;
            gap: 5px;
        }
        .peer-name {
            font-weight: bold;
            color: #00d4aa;
        }
        .peer-address {
            font-size: 12px;
            color: #888;
        }
        .peer-select {
            width: 20px;
            height: 20px;
            cursor: pointer;
        }
        .files-list {
            display: flex;
            flex-direction: column;
            gap: 8px;
        }
        .file-item {
            background: #1a1a2e;
            padding: 12px;
            border-radius: 6px;
            display: flex;
            justify-content: space-between;
            align-items: center;
            border: 1px solid #0f3460;
        }
        .file-name {
            flex: 1;
            overflow: hidden;
            text-overflow: ellipsis;
            white-space: nowrap;
        }
        .file-size {
            color: #888;
            font-size: 12px;
            margin-right: 15px;
        }
        .toast {
            position: fixed;
            bottom: 20px;
            right: 20px;
            background: #00d4aa;
            color: #1a1a2e;
            padding: 15px 25px;
            border-radius: 8px;
            box-shadow: 0 4px 12px rgba(0,0,0,0.4);
            transform: translateY(100px);
            opacity: 0;
            transition: all 0.3s;
            z-index: 1000;
            font-weight: bold;
        }
        .toast.show {
            transform: translateY(0);
            opacity: 1;
        }
        .toast.error { background: #e74c3c; color: white; }
        .empty-state {
            text-align: center;
            color: #666;
            padding: 20px;
        }
        .selected-peers {
            margin-top: 10px;
            padding: 10px;
            background: #1a1a2e;
            border-radius: 6px;
            font-size: 14px;
        }
        .selected-count {
            color: #00d4aa;
            font-weight: bold;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>🚀 File Transfer</h1>
        
        <div class="section">
            <div class="section-title">📤 Send File</div>
            <div class="drop-zone" id="dropZone">
                <span>📁</span>
                <p>Drag & drop files here or click to select</p>
                <input type="file" id="fileInput" multiple>
            </div>
            <div class="progress-container" id="progressContainer">
                <div class="progress-bar">
                    <div class="progress-fill" id="progressFill"></div>
                </div>
                <div class="progress-text" id="progressText">0%</div>
            </div>
            <div class="selected-peers" id="selectedPeers">
                Selected peers: <span class="selected-count" id="selectedCount">0</span>
            </div>
        </div>

        <div class="section">
            <div class="section-title">👥 Discovered Peers</div>
            <div class="peers-list" id="peersList">
                <div class="empty-state">Scanning network...</div>
            </div>
        </div>

        <div class="section">
            <div class="section-title">📂 Shared Files</div>
            <div class="files-list" id="filesList">
                <div class="empty-state">No files in shared folder</div>
            </div>
        </div>
    </div>

    <div class="toast" id="toast"></div>

    <script>
        let selectedPeers = new Set();
        let currentFile = null;

        const dropZone = document.getElementById('dropZone');
        const fileInput = document.getElementById('fileInput');
        const progressContainer = document.getElementById('progressContainer');
        const progressFill = document.getElementById('progressFill');
        const progressText = document.getElementById('progressText');
        const selectedCount = document.getElementById('selectedCount');
        const toast = document.getElementById('toast');

        // Drag & Drop
        dropZone.addEventListener('click', () => fileInput.click());
        dropZone.addEventListener('dragover', (e) => {
            e.preventDefault();
            dropZone.classList.add('dragover');
        });
        dropZone.addEventListener('dragleave', () => dropZone.classList.remove('dragover'));
        dropZone.addEventListener('drop', (e) => {
            e.preventDefault();
            dropZone.classList.remove('dragover');
            if (e.dataTransfer.files.length > 0) {
                handleFiles(e.dataTransfer.files);
            }
        });
        fileInput.addEventListener('change', () => {
            if (fileInput.files.length > 0) {
                handleFiles(fileInput.files);
            }
        });

        function showToast(message, isError = false) {
            toast.textContent = message;
            toast.className = 'toast show' + (isError ? ' error' : '');
            setTimeout(() => toast.classList.remove('show'), 3000);
        }

        function formatSize(bytes) {
            if (bytes === 0) return '0 B';
            const k = 1024;
            const sizes = ['B', 'KB', 'MB', 'GB'];
            const i = Math.floor(Math.log(bytes) / Math.log(k));
            return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
        }

        function handleFiles(files) {
            if (selectedPeers.size === 0) {
                showToast('Please select at least one peer first', true);
                return;
            }
            
            Array.from(files).forEach(file => {
                selectedPeers.forEach(peer => {
                    uploadFile(file, peer);
                });
            });
        }

        function uploadFile(file, peer) {
            currentFile = file;
            progressContainer.style.display = 'block';
            
            const xhr = new XMLHttpRequest();
            const formData = new FormData();
            formData.append('file', file);

            xhr.upload.addEventListener('progress', (e) => {
                if (e.lengthComputable) {
                    const percent = Math.round((e.loaded / e.total) * 100);
                    progressFill.style.width = percent + '%';
                    progressText.textContent = percent + '% - ' + file.name + ' to ' + peer.name;
                }
            });

            xhr.addEventListener('load', () => {
                if (xhr.status === 200) {
                    showToast('✅ Uploaded ' + file.name + ' to ' + peer.name);
                } else {
                    showToast('❌ Failed to upload ' + file.name, true);
                }
                setTimeout(() => {
                    progressContainer.style.display = 'none';
                    progressFill.style.width = '0%';
                }, 1000);
            });

            xhr.addEventListener('error', () => {
                showToast('❌ Network error uploading ' + file.name, true);
                progressContainer.style.display = 'none';
            });

            showToast('📤 Starting upload of ' + file.name + ' to ' + peer.name);
            xhr.open('POST', 'http://' + peer.ip + ':' + peer.port + '/upload');
            xhr.send(formData);
        }

        async function loadPeers() {
            try {
                const response = await fetch('/peers');
                const peers = await response.json();
                const peersList = document.getElementById('peersList');
                
                if (peers.length === 0) {
                    peersList.innerHTML = '<div class="empty-state">No peers discovered</div>';
                    return;
                }

                peersList.innerHTML = peers.map(peer => {
                    const isSelected = selectedPeers.has(peer);
                    return '<div class="peer-item">' +
                        '<div class="peer-info">' +
                            '<div class="peer-name">' + escapeHtml(peer.name) + '</div>' +
                            '<div class="peer-address">' + peer.ip + ':' + peer.port + '</div>' +
                        '</div>' +
                        '<input type="checkbox" class="peer-select" ' + (isSelected ? 'checked' : '') + 
                            ' onchange="togglePeer(' + JSON.stringify(peer).replace(/"/g, '&quot;') + ', this.checked)">' +
                    '</div>';
                }).join('');
            } catch (err) {
                console.error('Failed to load peers:', err);
            }
        }

        function togglePeer(peer, checked) {
            if (checked) {
                selectedPeers.add(peer);
            } else {
                selectedPeers.delete(peer);
            }
            selectedCount.textContent = selectedPeers.size;
        }

        function escapeHtml(text) {
            const div = document.createElement('div');
            div.textContent = text;
            return div.innerHTML;
        }

        async function loadFiles() {
            try {
                const response = await fetch('/files');
                const files = await response.json();
                const filesList = document.getElementById('filesList');
                
                if (files.length === 0) {
                    filesList.innerHTML = '<div class="empty-state">No files in shared folder</div>';
                    return;
                }

                filesList.innerHTML = files.map(file => 
                    '<div class="file-item">' +
                        '<span class="file-name">' + escapeHtml(file.name) + '</span>' +
                        '<span class="file-size">' + formatSize(file.size) + '</span>' +
                        '<a href="/download/' + encodeURIComponent(file.name) + '" class="btn">Download</a>' +
                    '</div>'
                ).join('');
            } catch (err) {
                console.error('Failed to load files:', err);
            }
        }

        // Refresh data periodically
        loadPeers();
        loadFiles();
        setInterval(loadPeers, 5000);
        setInterval(loadFiles, 10000);
    </script>
</body>
</html>`

// Start initializes the server, binds the port, starts mDNS, and serves HTTP
func (s *Server) Start() error {
	// 1. Create listener first (handles auto port assignment if port is 0)
	addr := fmt.Sprintf(":%d", s.port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	// 2. Get the actual port (in case it was 0)
	s.port = listener.Addr().(*net.TCPAddr).Port

	// 3. NOW start mDNS with confirmed port
	if err := s.startMDNS(); err != nil {
		listener.Close()
		return fmt.Errorf("failed to start mDNS: %w", err)
	}

	// 4. Start mDNS discovery in background
	go s.discoverPeers()

	// 5. Setup HTTP handlers
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/info", s.handleInfo)
	mux.HandleFunc("/peers", s.handlePeers)
	mux.HandleFunc("/files", s.handleFiles)
	mux.HandleFunc("/downloads", s.handleDownloads)
	mux.HandleFunc("/connected", s.handleConnected)
	mux.HandleFunc("/upload", s.handleUpload)
	mux.HandleFunc("/download/", s.handleDownload)

	// Wrap with connection tracker middleware
	handler := s.trackConnections(mux)

	// 6. Log and serve using existing listener
	log.Printf("Starting server on http://localhost:%d", s.port)
	log.Printf("Shared directory: %s", s.sharedDir)
	log.Printf("Downloads directory: %s", s.downloadsDir)

	return http.Serve(listener, handler)
}

func main() {
	flag.Parse()

	// Initialize directories
	sharedDir := "./shared"
	downloadsDir := getDownloadsDir()

	if err := os.MkdirAll(sharedDir, 0755); err != nil {
		log.Fatalf("Failed to create shared directory: %v", err)
	}

	// Auto-cleanup: Delete all contents of downloads folder on start
	if err := os.RemoveAll(downloadsDir); err != nil {
		log.Printf("[CLEANUP] Warning: Failed to clean downloads: %v", err)
	}
	if err := os.MkdirAll(downloadsDir, 0755); err != nil {
		log.Fatalf("Failed to create downloads directory: %v", err)
	}
	log.Printf("[CLEANUP] Downloads folder cleaned for new session")

	// Windows Firewall notification
	if runtime.GOOS == "windows" {
		log.Printf("")
		log.Printf("[SYSTEM] If other devices cannot connect, please ALLOW this app in your Windows Firewall settings.")
		log.Printf("[SYSTEM] On Windows, go to: Control Panel > Windows Defender Firewall > Allow an app...")
		log.Printf("")
	}

	// Get hostname
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	// Create peer registry
	registry := &PeerRegistry{
		peers: make(map[string]Peer),
	}

	// Create connected clients tracker
	clients := &ConnectedClients{
		clients: make(map[string]time.Time),
	}

	// Create server
	server := &Server{
		port:             *portFlag,
		sharedDir:        sharedDir,
		downloadsDir:     downloadsDir,
		peerRegistry:     registry,
		connectedClients: clients,
		hostname:         hostname,
	}

	// Start server (handles port detection, mDNS registration, and HTTP serving)
	if err := server.Start(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

// getDownloadsDir returns the appropriate downloads directory for the platform
func getDownloadsDir() string {
	if runtime.GOOS == "android" {
		// On Android, use the standard Downloads directory
		home := os.Getenv("HOME")
		if home == "" {
			home = "/sdcard"
		}
		return filepath.Join(home, "Download", "FileTransfer")
	}
	return "./downloads"
}

// startMDNS registers this service via mDNS
func (s *Server) startMDNS() error {
	host, _ := os.Hostname()
	info := []string{
		fmt.Sprintf("Hostname=%s", host),
		fmt.Sprintf("Port=%d", s.port),
	}

	service, err := mdns.NewMDNSService(
		host,
		"_projectzero._tcp",
		"",
		"",
		s.port,
		nil,
		info,
	)
	if err != nil {
		return err
	}

	server, err := mdns.NewServer(&mdns.Config{Zone: service})
	if err != nil {
		return err
	}

	s.mdnsServer = server
	return nil
}

// stopMDNS shuts down the mDNS server
func (s *Server) stopMDNS() {
	if s.mdnsServer != nil {
		s.mdnsServer.Shutdown()
	}
}

// discoverPeers scans for other _projectzero._tcp services
func (s *Server) discoverPeers() {
	entries := make(chan *mdns.ServiceEntry, 100)

	params := &mdns.QueryParam{
		Service: "_projectzero._tcp",
		Domain:  "local",
		Timeout: time.Second * 5,
		Entries: entries,
	}

	// Continuous discovery
	for {
		go func() {
			if err := mdns.Query(params); err != nil {
				log.Printf("mDNS query error: %v", err)
			}
		}()

		for entry := range entries {
			// Skip ourselves
			if entry.Name == s.hostname+"._projectzero._tcp.local." {
				continue
			}

			ip := entry.AddrV4.String()
			if ip == "<nil>" && entry.AddrV6 != nil {
				ip = entry.AddrV6.String()
			}

			peer := Peer{
				Name:     strings.TrimSuffix(entry.Name, "._projectzero._tcp.local."),
				IP:       ip,
				Port:     entry.Port,
				LastSeen: time.Now(),
			}

			s.peerRegistry.mutex.Lock()
			s.peerRegistry.peers[peer.Name] = peer
			s.peerRegistry.mutex.Unlock()
		}

		time.Sleep(5 * time.Second)
	}
}

// handleIndex serves the web UI
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	content, err := indexHTML.ReadFile("index.html")
	if err != nil {
		http.Error(w, "Failed to load UI", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	w.Write(content)
}

// ServerInfo holds information about this server instance
type ServerInfo struct {
	URL      string `json:"url"`
	Hostname string `json:"hostname"`
	Port     int    `json:"port"`
}

// getRealIP returns the actual outbound IP address by dialing an external address
// This ensures we get the real network interface IP (not APIPA 169.254.x.x)
func getRealIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "127.0.0.1"
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String()
}

// trackConnections middleware tracks connected clients by their IP address
func (s *Server) trackConnections(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := extractClientIP(r)
		if ip != "" {
			s.connectedClients.mutex.Lock()
			s.connectedClients.clients[ip] = time.Now()
			s.connectedClients.mutex.Unlock()
		}
		h.ServeHTTP(w, r)
	})
}

// extractClientIP extracts the client IP from the request
func extractClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first (for proxies)
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		return strings.Split(xff, ", ")[0]
	}
	// Fall back to RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		// If no port, RemoteAddr might be just IP
		return strings.Split(r.RemoteAddr, ":")[0]
	}
	return ip
}

// handleConnected returns the list of connected client IPs
func (s *Server) handleConnected(w http.ResponseWriter, r *http.Request) {
	s.connectedClients.mutex.RLock()
	// Clean up stale connections (> 5 min) and collect active ones
	cutoff := time.Now().Add(-5 * time.Minute)
	var active []string
	for ip, lastSeen := range s.connectedClients.clients {
		if lastSeen.After(cutoff) {
			active = append(active, ip)
		}
	}
	s.connectedClients.mutex.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(active)
}

// handleInfo returns server information for QR code generation
func (s *Server) handleInfo(w http.ResponseWriter, r *http.Request) {
	ip := getRealIP()
	info := ServerInfo{
		URL:      fmt.Sprintf("http://%s:%d", ip, s.port),
		Hostname: s.hostname,
		Port:     s.port,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}

// handlePeers returns the list of discovered peers
func (s *Server) handlePeers(w http.ResponseWriter, r *http.Request) {
	s.peerRegistry.mutex.RLock()
	peers := make([]Peer, 0, len(s.peerRegistry.peers))
	now := time.Now()

	for _, peer := range s.peerRegistry.peers {
		// Remove peers not seen in last 60 seconds
		if now.Sub(peer.LastSeen) < 60*time.Second {
			peers = append(peers, peer)
		}
	}
	s.peerRegistry.mutex.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(peers)
}

// FileInfo represents a file in the shared directory
type FileInfo struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}

// handleFiles returns the list of files in the shared directory
func (s *Server) handleFiles(w http.ResponseWriter, r *http.Request) {
	files := make([]FileInfo, 0)

	entries, err := os.ReadDir(s.sharedDir)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(files)
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		files = append(files, FileInfo{
			Name: entry.Name(),
			Size: info.Size(),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(files)
}

// handleUpload streams incoming files to disk
func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse multipart form with memory limit
	r.ParseMultipartForm(32 << 20) // 32MB memory limit for form parsing

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Failed to get file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Generate unique filename
	filename := getUniqueFilename(s.downloadsDir, header.Filename)
	filepath := filepath.Join(s.downloadsDir, filename)

	// Create output file
	out, err := os.Create(filepath)
	if err != nil {
		http.Error(w, "Failed to create file", http.StatusInternalServerError)
		return
	}
	defer out.Close()

	// Stream with buffer (CRITICAL: Don't load entire file into memory)
	buf := make([]byte, bufferSize)
	_, err = io.CopyBuffer(out, file, buf)
	if err != nil {
		http.Error(w, "Failed to write file", http.StatusInternalServerError)
		os.Remove(filepath)
		return
	}

	log.Printf("Received file: %s (%s)", filename, formatSize(header.Size))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"filename": filename,
		"size":     header.Size,
	})
}

// handleDownloads returns the list of files in the downloads directory
func (s *Server) handleDownloads(w http.ResponseWriter, r *http.Request) {
	files := make([]FileInfo, 0)

	entries, err := os.ReadDir(s.downloadsDir)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(files)
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		files = append(files, FileInfo{
			Name: entry.Name(),
			Size: info.Size(),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(files)
}

// handleDownload serves files with support for range requests and resumes
func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract filename from URL
	filename := strings.TrimPrefix(r.URL.Path, "/download/")
	filename, err := url.PathUnescape(filename)
	if err != nil {
		http.Error(w, "Invalid filename", http.StatusBadRequest)
		return
	}

	// Security: Prevent directory traversal
	if strings.Contains(filename, "..") || strings.Contains(filename, "/") || strings.Contains(filename, "\\") {
		http.Error(w, "Invalid filename", http.StatusBadRequest)
		return
	}

	// Search for file in downloads first, then shared (Option B)
	var fullPath string
	downloadsPath := filepath.Join(s.downloadsDir, filename)
	sharedPath := filepath.Join(s.sharedDir, filename)

	if info, err := os.Stat(downloadsPath); err == nil && !info.IsDir() {
		fullPath = downloadsPath
	} else if info, err := os.Stat(sharedPath); err == nil && !info.IsDir() {
		fullPath = sharedPath
	} else {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	// Verify it's not a directory
	info, err := os.Stat(fullPath)
	if err != nil || info.IsDir() {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	// Force download behavior with proper headers
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.Header().Set("Content-Type", "application/octet-stream")

	// Use http.ServeFile for proper file serving
	http.ServeFile(w, r, fullPath)
}

// getUniqueFilename generates a unique filename using "filename (1).ext" pattern
func getUniqueFilename(dir, filename string) string {
	fullPath := filepath.Join(dir, filename)
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return filename
	}

	ext := filepath.Ext(filename)
	name := strings.TrimSuffix(filename, ext)

	counter := 1
	for {
		newName := fmt.Sprintf("%s (%d)%s", name, counter, ext)
		newPath := filepath.Join(dir, newName)
		if _, err := os.Stat(newPath); os.IsNotExist(err) {
			return newName
		}
		counter++
	}
}

// formatSize converts bytes to human readable format
func formatSize(bytes int64) string {
	if bytes == 0 {
		return "0 B"
	}
	const k = 1024
	sizes := []string{"B", "KB", "MB", "GB", "TB"}
	i := 0
	size := float64(bytes)
	for size >= k && i < len(sizes)-1 {
		size /= k
		i++
	}
	return fmt.Sprintf("%.2f %s", size, sizes[i])
}
