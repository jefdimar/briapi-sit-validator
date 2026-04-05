package gdrive

import (
	"bytes"
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
	tokenURL  = "https://oauth2.googleapis.com/token"
	uploadURL = "https://www.googleapis.com/upload/drive/v3/files"
	boundary  = "foo_bar_baz"
)

// Client holds the OAuth2 credentials and HTTP client for Drive uploads.
type Client struct {
	clientID     string
	clientSecret string
	refreshToken string
	folderID     string
	httpClient   *http.Client
}

// NewClientFromEnv reads Google OAuth2 credentials from environment variables.
// Returns (nil, false) if any of the required three env vars are missing.
func NewClientFromEnv() (*Client, bool) {
	clientID := os.Getenv("GOOGLE_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
	refreshToken := os.Getenv("GOOGLE_REFRESH_TOKEN")

	if clientID == "" || clientSecret == "" || refreshToken == "" {
		return nil, false
	}

	return &Client{
		clientID:     clientID,
		clientSecret: clientSecret,
		refreshToken: refreshToken,
		folderID:     os.Getenv("GOOGLE_DRIVE_FOLDER_ID"),
		httpClient:   &http.Client{},
	}, true
}

// UploadExcel uploads the given Excel bytes to Google Drive and returns the
// view URL (https://drive.google.com/file/d/{id}/view).
func (c *Client) UploadExcel(ctx context.Context, filename string, data []byte) (string, error) {
	accessToken, err := c.getAccessToken(ctx)
	if err != nil {
		return "", fmt.Errorf("gdrive: get access token: %w", err)
	}

	fileID, err := c.uploadFile(ctx, accessToken, filename, data)
	if err != nil {
		return "", fmt.Errorf("gdrive: upload file: %w", err)
	}

	return fmt.Sprintf("https://drive.google.com/file/d/%s/view", fileID), nil
}

// getAccessToken exchanges the stored refresh token for a short-lived access token.
func (c *Client) getAccessToken(ctx context.Context) (string, error) {
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("client_id", c.clientID)
	form.Set("client_secret", c.clientSecret)
	form.Set("refresh_token", c.refreshToken)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token endpoint returned %d: %s", resp.StatusCode, body)
	}

	var result struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parse token response: %w", err)
	}
	if result.AccessToken == "" {
		return "", fmt.Errorf("token response missing access_token")
	}

	return result.AccessToken, nil
}

// uploadFile performs a multipart/related upload to the Drive API and returns
// the new file's ID.
func (c *Client) uploadFile(ctx context.Context, accessToken, filename string, data []byte) (string, error) {
	// Build the metadata JSON for the first part.
	type fileMetadata struct {
		Name    string   `json:"name"`
		Parents []string `json:"parents,omitempty"`
	}
	meta := fileMetadata{Name: filename}
	if c.folderID != "" {
		meta.Parents = []string{c.folderID}
	}
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return "", err
	}

	// Manually build a multipart/related body.
	var buf bytes.Buffer

	// -- metadata part --
	buf.WriteString("--" + boundary + "\r\n")
	buf.WriteString("Content-Type: application/json; charset=UTF-8\r\n")
	buf.WriteString("\r\n")
	buf.Write(metaJSON)
	buf.WriteString("\r\n")

	// -- file data part --
	buf.WriteString("--" + boundary + "\r\n")
	buf.WriteString("Content-Type: application/vnd.openxmlformats-officedocument.spreadsheetml.sheet\r\n")
	buf.WriteString("\r\n")
	buf.Write(data)
	buf.WriteString("\r\n")

	// -- closing boundary --
	buf.WriteString("--" + boundary + "--")

	uploadEndpoint := uploadURL + "?uploadType=multipart&fields=id"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadEndpoint, &buf)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "multipart/related; boundary="+boundary)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("upload returned %d: %s", resp.StatusCode, respBody)
	}

	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parse upload response: %w", err)
	}
	if result.ID == "" {
		return "", fmt.Errorf("upload response missing id")
	}

	return result.ID, nil
}
