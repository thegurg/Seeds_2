# 🌱 Seeds

**Seeds** is a lightweight, self-hosted file-sharing utility written in **Go**. It replaces boring, corporate file-transfer tools with a cozy, gamified "Digital Garden" experience.


## Showcase
### Mobile UI
<img width="391" height="849" alt="image" src="https://github.com/user-attachments/assets/616d0f21-86e8-4053-8a71-83b7b84f90f8" />

### Tablet/Desktop UI
<img width="952" height="944" alt="image" src="https://github.com/user-attachments/assets/303f5883-7d84-4fd1-ad3d-9835811c682c" />

### Another Version
If you dont like Pixel Art UI you can use previus version of this (Glassmorphism UI)
https://github.com/thegurg/Seeds.git

## ✨ Features

- **Pixel Art UI:** A unique, retro-inspired interface using the "Press Start 2P" aesthetic.
    
- **Zero Dependencies:** Compiled into a single binary. No need for Python, Node.js, or external databases.
    
- **True Portability:** Runs on Windows, Linux (Arch, Debian, etc.), and ARM devices (Raspberry Pi).
    
- **Automatic Setup:** The application automatically generates `downloads` and `shared` directories on the first launch.
    
- **Privacy-Centric:** Your data never leaves your local network. No third-party servers involved.
    
---

## ⚠️ Important Note for Windows Users

When running **SEEDS**, you might encounter **Windows SmartScreen** or **Defender** warnings:

- **Why?** The app is written in Go and is not digitally signed with a costly corporate certificate. Windows often flags unsigned `.exe` files as "unknown" or "suspicious."
    
- **Is it safe?** Yes. This is a local-only tool.
    
- **How to bypass:** 1. Click **"More info"**. 2. Click **"Run anyway"**. 3. If the **Firewall** prompt appears, select **"Allow access"** for Private Networks, otherwise other devices won't be able to find your PC.
    

---


## 🚀 Quick Start

1. Download the binary for your OS from the `dist/` folder.
    
2. Launch the executable:
    
    Bash
    
    ```
    ./seeds
    ```
    
3. Open your browser at: `http://localhost:8080` (or use your machine's local IP).
    
4. **SOW:** Upload a file to the "garden."
    
5. **HARVEST:** Download shared files on any other device in your network.
    

## 🛠 Tech Stack

- **Backend:** [Go (Golang)](https://go.dev/) — chosen for its high performance, low memory footprint, and easy cross-compilation.
    
- **Frontend:** Pure HTML5 / CSS3 — optimized for pixel-perfect rendering using `image-rendering: pixelated`.
    
- **Typography:** [Press Start 2P](https://fonts.google.com/specimen/Press+Start+2P) for that authentic 8-bit feel.
    

## 📁 Project Structure

Plaintext

```
seeds/
├── dist/               # Pre-compiled binaries for all platforms
├── shared/             # Files currently being shared
├── downloads/          # Destination for incoming files
├── main.go             # The core Go server logic
└── index.html          # The pixel-art frontend
```

## 🏗 Build from Source

If you want to customize the garden or build it yourself:

Bash

```
git clone https://github.com/thegurg/Seeds_2.git
cd Seeds_2
go build -o seeds main.go
```

---
