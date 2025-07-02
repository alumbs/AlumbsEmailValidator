package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/smtp"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

type EmailResult struct {
	Address     string `json:"address"`
	Username    string `json:"username"`
	Domain      string `json:"domain"`
	HostExists  bool   `json:"hostExists"`
	Deliverable bool   `json:"deliverable"`
	FullInbox   bool   `json:"fullInbox"`
	CatchAll    bool   `json:"catchAll"`
	Disposable  bool   `json:"disposable"`
	Gravatar    bool   `json:"gravatar"`
}

func main() {
	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())

	// Routes matching Trumail API
	e.GET("/v1/json/:email", validateEmailJSON)
	e.GET("/v1/xml/:email", validateEmailXML)
	e.GET("/v1/health", healthCheck)
	e.GET("/", healthCheck)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Starting Trumail-compatible email validation service on port %s", port)
	log.Printf("Available endpoints:")
	log.Printf("  GET /v1/json/:email")
	log.Printf("  GET /v1/xml/:email")
	log.Printf("  GET /v1/health")

	e.Logger.Fatal(e.Start(":" + port))
}

func validateEmailJSON(c echo.Context) error {
	email := c.Param("email")
	result := validateEmail(email)
	return c.JSON(http.StatusOK, result)
}

func validateEmailXML(c echo.Context) error {
	email := c.Param("email")
	result := validateEmail(email)
	return c.XML(http.StatusOK, result)
}

func healthCheck(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{
		"status":  "healthy",
		"service": "trumail-compatible",
		"version": "1.0",
	})
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