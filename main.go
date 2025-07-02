package main

import (
	"encoding/json"
	"encoding/xml"
	"log"
	"net"
	"net/http"
	"net/smtp"
	"os"
	"regexp"
	"strings"
	"time"
)

type EmailResult struct {
	XMLName     xml.Name `json:"-" xml:"result"`
	Address     string   `json:"address" xml:"address"`
	Username    string   `json:"username" xml:"username"`
	Domain      string   `json:"domain" xml:"domain"`
	HostExists  bool     `json:"hostExists" xml:"hostExists"`
	Deliverable bool     `json:"deliverable" xml:"deliverable"`
	FullInbox   bool     `json:"fullInbox" xml:"fullInbox"`
	CatchAll    bool     `json:"catchAll" xml:"catchAll"`
	Disposable  bool     `json:"disposable" xml:"disposable"`
	Gravatar    bool     `json:"gravatar" xml:"gravatar"`
}

func main() {
	// Routes matching Trumail API
	http.HandleFunc("/v1/json/", validateEmailJSONHandler)
	http.HandleFunc("/v1/xml/", validateEmailXMLHandler)
	http.HandleFunc("/v1/health", healthCheckHandler)
	http.HandleFunc("/", healthCheckHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Starting Trumail-compatible email validation service on port %s", port)
	log.Printf("Available endpoints:")
	log.Printf("  GET /v1/json/{email}")
	log.Printf("  GET /v1/xml/{email}")
	log.Printf("  GET /v1/health")

	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func validateEmailJSONHandler(w http.ResponseWriter, r *http.Request) {
	email := strings.TrimPrefix(r.URL.Path, "/v1/json/")
	if email == "" {
		http.Error(w, "Email parameter required", http.StatusBadRequest)
		return
	}

	result := validateEmail(email)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func validateEmailXMLHandler(w http.ResponseWriter, r *http.Request) {
	email := strings.TrimPrefix(r.URL.Path, "/v1/xml/")
	if email == "" {
		http.Error(w, "Email parameter required", http.StatusBadRequest)
		return
	}

	result := validateEmail(email)
	w.Header().Set("Content-Type", "application/xml")
	xml.NewEncoder(w).Encode(result)
}

func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	response := map[string]string{
		"status":  "healthy",
		"service": "trumail-compatible",
		"version": "1.0",
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func validateEmail(email string) EmailResult {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return EmailResult{
			Address:     email,
			Username:    "",
			Domain:      "",
			HostExists:  false,
			Deliverable: false,
			FullInbox:   false,
			CatchAll:    false,
			Disposable:  false,
			Gravatar:    false,
		}
	}

	username := parts[0]
	domain := parts[1]

	// Basic email format validation
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	if !emailRegex.MatchString(email) {
		return EmailResult{
			Address:     email,
			Username:    username,
			Domain:      domain,
			HostExists:  false,
			Deliverable: false,
			FullInbox:   false,
			CatchAll:    false,
			Disposable:  false,
			Gravatar:    false,
		}
	}

	// Check if domain has MX record
	hostExists := checkMXRecord(domain)

	// Attempt SMTP verification
	deliverable := false
	if hostExists {
		deliverable = checkSMTPDeliverable(email, domain)
	}

	// Check for disposable email domains
	disposable := isDisposableEmail(domain)

	return EmailResult{
		Address:     email,
		Username:    username,
		Domain:      domain,
		HostExists:  hostExists,
		Deliverable: deliverable,
		FullInbox:   false,
		CatchAll:    false,
		Disposable:  disposable,
		Gravatar:    false,
	}
}

func checkMXRecord(domain string) bool {
	_, err := net.LookupMX(domain)
	return err == nil
}

func checkSMTPDeliverable(email, domain string) bool {
	// Get MX records
	mxRecords, err := net.LookupMX(domain)
	if err != nil || len(mxRecords) == 0 {
		return false
	}

	// Try to connect to the first MX server
	mxHost := strings.TrimSuffix(mxRecords[0].Host, ".")

	// Set up connection with timeout
	conn, err := net.DialTimeout("tcp", mxHost+":25", 10*time.Second)
	if err != nil {
		return false
	}
	defer conn.Close()

	// Create SMTP client
	client, err := smtp.NewClient(conn, mxHost)
	if err != nil {
		return false
	}
	defer client.Quit()

	// HELO
	sourceAddr := os.Getenv("SOURCE_ADDR")
	if sourceAddr == "" {
		sourceAddr = "verify@example.com"
	}

	err = client.Hello("trumail-validator.com")
	if err != nil {
		return false
	}

	// MAIL FROM
	err = client.Mail(sourceAddr)
	if err != nil {
		return false
	}

	// RCPT TO - this is where we test if the email exists
	err = client.Rcpt(email)
	return err == nil
}

func isDisposableEmail(domain string) bool {
	disposableDomains := []string{
		"10minutemail.com", "guerrillamail.com", "mailinator.com",
		"tempmail.org", "throwaway.email", "temp-mail.org",
	}

	domain = strings.ToLower(domain)
	for _, disposable := range disposableDomains {
		if domain == disposable {
			return true
		}
	}
	return false
}