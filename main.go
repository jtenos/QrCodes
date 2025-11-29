package main

import (
	"encoding/base64"
	"fmt"
	"html/template"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
	goqrcode "github.com/skip2/go-qrcode"
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/webp"
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
	http.HandleFunc("/read-qr", readQRHandler)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	fmt.Println("Server starting on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if err := templates.ExecuteTemplate(w, "home.html", nil); err != nil {
		log.Printf("Error executing home template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func generateHandler(w http.ResponseWriter, r *http.Request) {
	if err := templates.ExecuteTemplate(w, "generate.html", nil); err != nil {
		log.Printf("Error executing generate template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func readingHandler(w http.ResponseWriter, r *http.Request) {
	if err := templates.ExecuteTemplate(w, "reading.html", nil); err != nil {
		log.Printf("Error executing reading template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
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
	png, err := goqrcode.Encode(totpURI, goqrcode.Medium, 256)
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

	if err := templates.ExecuteTemplate(w, "qr_result.html", data); err != nil {
		log.Printf("Error executing qr_result template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// ParsedValue represents a key-value pair for parsed QR content
type ParsedValue struct {
	Key   string
	Value string
}

// QRReadResult holds the result of reading a QR code
type QRReadResult struct {
	RawText      string
	ParsedValues []ParsedValue
	QRType       string
	GenerateLink string
	Error        string
	Base64Image  string
}

func readQRHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse multipart form (max 10MB)
	err := r.ParseMultipartForm(10 << 20)
	if err != nil {
		result := QRReadResult{Error: "Error parsing form: " + err.Error()}
		templates.ExecuteTemplate(w, "qr_read_result.html", result)
		return
	}

	file, header, err := r.FormFile("qr_image")
	if err != nil {
		result := QRReadResult{Error: "Error reading uploaded file: " + err.Error()}
		templates.ExecuteTemplate(w, "qr_read_result.html", result)
		return
	}
	defer file.Close()

	// Validate file type
	contentType := header.Header.Get("Content-Type")
	validTypes := map[string]bool{
		"image/png":  true,
		"image/jpeg": true,
		"image/jpg":  true,
		"image/webp": true,
		"image/bmp":  true,
		"image/gif":  true,
	}
	if !validTypes[contentType] {
		result := QRReadResult{Error: "Invalid file type. Supported formats: PNG, JPG, WEBP, BMP, GIF"}
		templates.ExecuteTemplate(w, "qr_read_result.html", result)
		return
	}

	// Read image data for preview
	imageData, err := io.ReadAll(file)
	if err != nil {
		result := QRReadResult{Error: "Error reading image data: " + err.Error()}
		templates.ExecuteTemplate(w, "qr_read_result.html", result)
		return
	}
	base64Image := base64.StdEncoding.EncodeToString(imageData)

	// Seek back to beginning for decoding
	file.Seek(0, 0)

	// Decode image
	img, _, err := image.Decode(file)
	if err != nil {
		result := QRReadResult{Error: "Error decoding image: " + err.Error(), Base64Image: base64Image}
		templates.ExecuteTemplate(w, "qr_read_result.html", result)
		return
	}

	// Read QR code
	bmp, err := gozxing.NewBinaryBitmapFromImage(img)
	if err != nil {
		result := QRReadResult{Error: "Error processing image: " + err.Error(), Base64Image: base64Image}
		templates.ExecuteTemplate(w, "qr_read_result.html", result)
		return
	}

	qrReader := qrcode.NewQRCodeReader()
	qrResult, err := qrReader.Decode(bmp, nil)
	if err != nil {
		result := QRReadResult{Error: "No QR code found in the image. Please ensure the image contains a clear QR code.", Base64Image: base64Image}
		templates.ExecuteTemplate(w, "qr_read_result.html", result)
		return
	}

	rawText := qrResult.GetText()
	parsedValues, qrType := parseQRContent(rawText)
	generateLink := "/generate?text=" + url.QueryEscape(rawText)

	result := QRReadResult{
		RawText:      rawText,
		ParsedValues: parsedValues,
		QRType:       qrType,
		GenerateLink: generateLink,
		Base64Image:  base64Image,
	}

	if err := templates.ExecuteTemplate(w, "qr_read_result.html", result); err != nil {
		log.Printf("Error executing qr_read_result template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func parseQRContent(data string) ([]ParsedValue, string) {
	var values []ParsedValue

	// WiFi
	if strings.HasPrefix(data, "WIFI:") {
		values = append(values, ParsedValue{Key: "Type", Value: "WiFi Network"})
		wifiData := data[5:]
		if match := regexp.MustCompile(`S:([^;]*)`).FindStringSubmatch(wifiData); len(match) > 1 {
			values = append(values, ParsedValue{Key: "Network Name (SSID)", Value: match[1]})
		}
		if match := regexp.MustCompile(`T:([^;]*)`).FindStringSubmatch(wifiData); len(match) > 1 {
			values = append(values, ParsedValue{Key: "Security Type", Value: match[1]})
		}
		if match := regexp.MustCompile(`P:([^;]*)`).FindStringSubmatch(wifiData); len(match) > 1 {
			values = append(values, ParsedValue{Key: "Password", Value: match[1]})
		}
		if match := regexp.MustCompile(`H:([^;]*)`).FindStringSubmatch(wifiData); len(match) > 1 && strings.ToLower(match[1]) == "true" {
			values = append(values, ParsedValue{Key: "Hidden Network", Value: "Yes"})
		}
		return values, "wifi"
	}

	// vCard
	if strings.HasPrefix(data, "BEGIN:VCARD") {
		values = append(values, ParsedValue{Key: "Type", Value: "vCard/Contact"})
		lines := strings.Split(data, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "FN:") {
				values = append(values, ParsedValue{Key: "Full Name", Value: line[3:]})
			} else if strings.HasPrefix(line, "TEL") && strings.Contains(line, ":") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) > 1 {
					values = append(values, ParsedValue{Key: "Phone", Value: parts[1]})
				}
			} else if strings.HasPrefix(line, "EMAIL") && strings.Contains(line, ":") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) > 1 {
					values = append(values, ParsedValue{Key: "Email", Value: parts[1]})
				}
			} else if strings.HasPrefix(line, "ORG:") {
				values = append(values, ParsedValue{Key: "Organization", Value: line[4:]})
			} else if strings.HasPrefix(line, "TITLE:") {
				values = append(values, ParsedValue{Key: "Title", Value: line[6:]})
			}
		}
		return values, "vcard"
	}

	// Phone
	if strings.HasPrefix(strings.ToLower(data), "tel:") {
		values = append(values, ParsedValue{Key: "Type", Value: "Phone Number"})
		values = append(values, ParsedValue{Key: "Number", Value: data[4:]})
		return values, "phone"
	}

	// SMS
	if strings.HasPrefix(strings.ToLower(data), "sms:") || strings.HasPrefix(strings.ToLower(data), "smsto:") {
		values = append(values, ParsedValue{Key: "Type", Value: "SMS"})
		smsData := regexp.MustCompile(`(?i)^(sms|smsto):`).ReplaceAllString(data, "")
		parts := strings.SplitN(smsData, "?", 2)
		values = append(values, ParsedValue{Key: "Number", Value: parts[0]})
		if len(parts) > 1 {
			if match := regexp.MustCompile(`(?i)body=([^&]*)`).FindStringSubmatch(parts[1]); len(match) > 1 {
				decoded, _ := url.QueryUnescape(match[1])
				values = append(values, ParsedValue{Key: "Message", Value: decoded})
			}
		}
		return values, "sms"
	}

	// Email
	if strings.HasPrefix(strings.ToLower(data), "mailto:") {
		values = append(values, ParsedValue{Key: "Type", Value: "Email"})
		emailData := data[7:]
		parts := strings.SplitN(emailData, "?", 2)
		values = append(values, ParsedValue{Key: "Address", Value: parts[0]})
		if len(parts) > 1 {
			params, _ := url.ParseQuery(parts[1])
			if subject := params.Get("subject"); subject != "" {
				values = append(values, ParsedValue{Key: "Subject", Value: subject})
			}
			if body := params.Get("body"); body != "" {
				values = append(values, ParsedValue{Key: "Body", Value: body})
			}
		}
		return values, "email"
	}

	// Geo location
	if strings.HasPrefix(strings.ToLower(data), "geo:") {
		values = append(values, ParsedValue{Key: "Type", Value: "Location"})
		geoData := data[4:]
		coords := strings.SplitN(geoData, ",", 2)
		if len(coords) >= 2 {
			values = append(values, ParsedValue{Key: "Latitude", Value: coords[0]})
			lon := strings.SplitN(coords[1], "?", 2)[0]
			values = append(values, ParsedValue{Key: "Longitude", Value: lon})
		}
		return values, "geo"
	}

	// URL (http/https only)
	if strings.HasPrefix(strings.ToLower(data), "http://") || strings.HasPrefix(strings.ToLower(data), "https://") {
		parsedURL, err := url.Parse(data)
		if err == nil {
			values = append(values, ParsedValue{Key: "Type", Value: "URL"})
			values = append(values, ParsedValue{Key: "Protocol", Value: parsedURL.Scheme})
			values = append(values, ParsedValue{Key: "Host", Value: parsedURL.Host})
			if parsedURL.Path != "" && parsedURL.Path != "/" {
				values = append(values, ParsedValue{Key: "Path", Value: parsedURL.Path})
			}
			if parsedURL.RawQuery != "" {
				values = append(values, ParsedValue{Key: "Query", Value: parsedURL.RawQuery})
			}
			return values, "url"
		}
	}

	// Plain text (no specific format detected)
	return nil, "text"
}
