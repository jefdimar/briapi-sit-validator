// get-drive-token is a one-time helper that walks you through the Google
// OAuth2 consent flow and prints the refresh token you need for the
// GOOGLE_REFRESH_TOKEN environment variable.
//
// Prerequisites:
//   - A Google Cloud project with the Drive API enabled.
//   - An OAuth2 "Desktop app" client ID + secret from the Cloud Console.
//
// Usage:
//
//	go run scripts/get-drive-token/main.go
//
// The script automatically reads GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET
// from a .env file in the current directory (or from shell environment
// variables if you prefer to export them manually).
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

const (
	authURL     = "https://accounts.google.com/o/oauth2/v2/auth"
	tokenURL    = "https://oauth2.googleapis.com/token"
	redirectURI = "http://localhost:8085/callback"
	scope       = "https://www.googleapis.com/auth/drive.file"
	listenAddr  = ":8085"
)

// loadDotEnv loads key=value pairs from a .env file into the process
// environment.  Existing env vars are never overridden.
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return // .env is optional
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		if len(val) >= 2 {
			if (val[0] == '"' && val[len(val)-1] == '"') ||
				(val[0] == '\'' && val[len(val)-1] == '\'') {
				val = val[1 : len(val)-1]
			}
		}
		if os.Getenv(key) == "" {
			os.Setenv(key, val)
		}
	}
}

func main() {
	loadDotEnv(".env")

	clientID := os.Getenv("GOOGLE_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")

	if clientID == "" || clientSecret == "" {
		fmt.Fprintln(os.Stderr, "Error: GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET must be set.")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Add them to your .env file (see .env.example) and re-run:")
		fmt.Fprintln(os.Stderr, "  go run scripts/get-drive-token/main.go")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Or pass them inline:")
		fmt.Fprintln(os.Stderr, "  GOOGLE_CLIENT_ID=<id> GOOGLE_CLIENT_SECRET=<secret> \\")
		fmt.Fprintln(os.Stderr, "    go run scripts/get-drive-token/main.go")
		os.Exit(1)
	}

	// Build the consent URL.
	params := url.Values{}
	params.Set("response_type", "code")
	params.Set("client_id", clientID)
	params.Set("redirect_uri", redirectURI)
	params.Set("scope", scope)
	params.Set("access_type", "offline")
	params.Set("prompt", "consent") // force refresh_token to be returned every time

	consentURL := authURL + "?" + params.Encode()

	fmt.Println("─────────────────────────────────────────────────────────")
	fmt.Println("  BRIAPI SIT Validator — Google Drive first-time setup")
	fmt.Println("─────────────────────────────────────────────────────────")
	fmt.Println()
	fmt.Println("1. Open the following URL in your browser:")
	fmt.Println()
	fmt.Println(" ", consentURL)
	fmt.Println()
	fmt.Println("2. Sign in with the Google account that owns the Drive")
	fmt.Println("   where reports should be saved.")
	fmt.Println()
	fmt.Println("3. Click 'Allow' on the consent screen.")
	fmt.Println()
	fmt.Println("Waiting for the OAuth2 callback on http://localhost:8085 ...")
	fmt.Println()

	// codeCh receives the authorization code from the callback handler.
	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	srv := &http.Server{Addr: listenAddr, Handler: mux}

	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		errParam := r.URL.Query().Get("error")

		if errParam != "" {
			fmt.Fprintf(w, "<h2>Authorization denied: %s</h2><p>You may close this tab.</p>", errParam)
			errCh <- fmt.Errorf("authorization denied: %s", errParam)
			return
		}
		if code == "" {
			fmt.Fprintf(w, "<h2>Missing code parameter.</h2><p>You may close this tab.</p>")
			errCh <- fmt.Errorf("callback received no code parameter")
			return
		}

		fmt.Fprintf(w, "<h2>Authorization successful!</h2><p>You may close this tab and return to the terminal.</p>")
		codeCh <- code
	})

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("local server error: %w", err)
		}
	}()

	// Wait for either a code or an error.
	var authCode string
	select {
	case authCode = <-codeCh:
	case err := <-errCh:
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Shut down the local server now that we have the code.
	_ = srv.Shutdown(context.Background())

	// Exchange the authorization code for tokens.
	fmt.Println("Authorization code received. Exchanging for tokens ...")

	form := url.Values{}
	form.Set("code", authCode)
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("redirect_uri", redirectURI)
	form.Set("grant_type", "authorization_code")

	resp, err := http.Post(tokenURL, "application/x-www-form-urlencoded", strings.NewReader(form.Encode())) //nolint:noctx
	if err != nil {
		fmt.Fprintf(os.Stderr, "Token exchange request failed: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "Token endpoint returned %d:\n%s\n", resp.StatusCode, body)
		os.Exit(1)
	}

	var result struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type"`
		Scope        string `json:"scope"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse token response: %v\n%s\n", err, body)
		os.Exit(1)
	}

	if result.RefreshToken == "" {
		fmt.Fprintln(os.Stderr, "No refresh_token in response.")
		fmt.Fprintln(os.Stderr, "This can happen if the app was already authorized without 'prompt=consent'.")
		fmt.Fprintln(os.Stderr, "Revoke access at https://myaccount.google.com/permissions and re-run this script.")
		os.Exit(1)
	}

	fmt.Println()
	fmt.Println("─────────────────────────────────────────────────────────")
	fmt.Println("  Success! Add the following to your .env file:")
	fmt.Println("─────────────────────────────────────────────────────────")
	fmt.Println()
	fmt.Printf("GOOGLE_CLIENT_ID=%s\n", clientID)
	fmt.Printf("GOOGLE_CLIENT_SECRET=%s\n", clientSecret)
	fmt.Printf("GOOGLE_REFRESH_TOKEN=%s\n", result.RefreshToken)
	fmt.Println()
	fmt.Println("─────────────────────────────────────────────────────────")
}
