package main

import (
	"encoding/base64"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"strconv"

	"github.com/skip2/go-qrcode"
)

var templates *template.Template

func main() {
	var err error
	templates, err = template.ParseGlob("templates/*.html")
	if err != nil {
		log.Fatal("Error parsing templates:", err)
	}

	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/generate", generateHandler)
	http.HandleFunc("/generate-qr", generateQRHandler)
	http.HandleFunc("/reading", readingHandler)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	fmt.Println("Server starting on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	templates.ExecuteTemplate(w, "home.html", nil)
}

func generateHandler(w http.ResponseWriter, r *http.Request) {
	templates.ExecuteTemplate(w, "generate.html", nil)
}

func readingHandler(w http.ResponseWriter, r *http.Request) {
	templates.ExecuteTemplate(w, "reading.html", nil)
}

func generateQRHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Error parsing form", http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	user := r.FormValue("user")
	secret := r.FormValue("secret")
	periodStr := r.FormValue("period")

	if name == "" || user == "" || secret == "" {
		http.Error(w, "Name, User, and Secret are required", http.StatusBadRequest)
		return
	}

	period := 30
	if periodStr != "" {
		p, err := strconv.Atoi(periodStr)
		if err == nil && p > 0 {
			period = p
		}
	}

	// Build TOTP URI: otpauth://totp/ISSUER:ACCOUNT?secret=SECRET&issuer=ISSUER&period=PERIOD
	totpURI := fmt.Sprintf("otpauth://totp/%s:%s?secret=%s&issuer=%s&period=%d",
		url.PathEscape(name),
		url.PathEscape(user),
		url.QueryEscape(secret),
		url.QueryEscape(name),
		period,
	)

	// Generate QR code as PNG
	png, err := qrcode.Encode(totpURI, qrcode.Medium, 256)
	if err != nil {
		http.Error(w, "Error generating QR code", http.StatusInternalServerError)
		return
	}

	// Convert to base64 for embedding in HTML
	base64Image := base64.StdEncoding.EncodeToString(png)

	// Return HTML fragment for HTMX
	data := struct {
		Name        string
		User        string
		Base64Image string
	}{
		Name:        name,
		User:        user,
		Base64Image: base64Image,
	}

	templates.ExecuteTemplate(w, "qr_result.html", data)
}
