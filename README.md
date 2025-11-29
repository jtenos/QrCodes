# QrCodes

QR Code Reader and Writer - A Go web application for generating TOTP QR codes.

## Features

- **QR Generation**: Generate TOTP QR codes for two-factor authentication setup
  - Input: Name (Issuer), User (Account), Secret, and optional Time Period
  - Download generated QR codes as PNG files
  - Copy QR codes to clipboard
- **QR Reading**: Coming soon

## Requirements

- Go 1.24 or later

## Running the Application

```bash
# Install dependencies
go mod download

# Build and run
go build -o qrcodes .
./qrcodes
```

Then open http://localhost:8080 in your browser.

## Tech Stack

- Go (backend)
- Bootstrap 5 (styling)
- HTMX (dynamic interactions)
