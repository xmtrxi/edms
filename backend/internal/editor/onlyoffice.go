package editor

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"path"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Format describes how ONLYOFFICE handles a file type.
type Format struct {
	// DocumentType is word, cell, slide or pdf.
	DocumentType string
	Editable     bool
}

var formats = map[string]Format{
	"docx": {"word", true}, "docm": {"word", true}, "dotx": {"word", true}, "odt": {"word", true}, "ott": {"word", true},
	"rtf": {"word", true}, "txt": {"word", true}, "doc": {"word", false}, "dot": {"word", false}, "epub": {"word", false},
	"fb2": {"word", false}, "html": {"word", false}, "htm": {"word", false}, "mht": {"word", false}, "xps": {"word", false},
	"djvu": {"word", false},
	"xlsx": {"cell", true}, "xlsm": {"cell", true}, "xltx": {"cell", true}, "ods": {"cell", true}, "ots": {"cell", true},
	"csv": {"cell", true}, "xls": {"cell", false}, "xlt": {"cell", false},
	"pptx": {"slide", true}, "pptm": {"slide", true}, "potx": {"slide", true}, "odp": {"slide", true}, "otp": {"slide", true},
	"ppt": {"slide", false}, "pps": {"slide", false}, "ppsx": {"slide", false},
	"pdf": {"pdf", false},
}

// FormatOf returns how ONLYOFFICE handles a file name, and whether it can open
// it at all.
func FormatOf(name string) (string, Format, bool) {
	ext := strings.TrimPrefix(strings.ToLower(path.Ext(name)), ".")
	format, ok := formats[ext]
	return ext, format, ok
}

// Key identifies one revision to ONLYOFFICE. Everyone editing the same
// revision shares a session; a new version starts a new one.
func Key(documentID, versionID, checksum string) string {
	sum := sha256.Sum256([]byte(documentID + "|" + versionID + "|" + checksum))
	return hex.EncodeToString(sum[:])[:32]
}

// Sign returns an HS256 JWT of claims.
func Sign(secret string, claims map[string]any) (string, error) {
	return jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims(claims)).SignedString([]byte(secret))
}

// Verify checks an HS256 JWT and returns its claims. ONLYOFFICE's own tokens
// don't always carry an expiry; when one does, it is enforced.
func Verify(secret, token string) (map[string]any, error) {
	parsed, err := jwt.Parse(token, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(secret), nil
	}, jwt.WithLeeway(time.Minute))
	if err != nil || !parsed.Valid {
		return nil, errors.New("invalid token")
	}
	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}

// Scoped tokens authorise ONLYOFFICE to fetch one file or save one document.
type Scoped struct {
	Purpose    string `json:"purpose"`
	DocumentID string `json:"doc"`
	OrgID      string `json:"org,omitempty"`
	Path       string `json:"path,omitempty"`
	Name       string `json:"name,omitempty"`
}

// SignScoped issues a token for one purpose that expires after ttl.
func SignScoped(secret string, scope Scoped, ttl time.Duration) (string, error) {
	return Sign(secret, map[string]any{
		"purpose": scope.Purpose, "doc": scope.DocumentID, "org": scope.OrgID, "path": scope.Path, "name": scope.Name,
		"exp": time.Now().Add(ttl).Unix(),
	})
}

// VerifyScoped checks a scoped token and that it was issued for purpose. It
// requires an expiry, unlike tokens that come from ONLYOFFICE.
func VerifyScoped(secret, token, purpose string) (*Scoped, error) {
	parsed, err := jwt.Parse(token, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(secret), nil
	}, jwt.WithExpirationRequired())
	if err != nil || !parsed.Valid {
		return nil, errors.New("invalid or expired link")
	}
	raw, _ := json.Marshal(parsed.Claims)
	var scope Scoped
	if err := json.Unmarshal(raw, &scope); err != nil || scope.Purpose != purpose || scope.DocumentID == "" {
		return nil, errors.New("invalid link")
	}
	return &scope, nil
}

// Callback is what ONLYOFFICE posts when an editing session changes state.
type Callback struct {
	Key      string   `json:"key"`
	Status   int      `json:"status"`
	URL      string   `json:"url"`
	FileType string   `json:"filetype"`
	Users    []string `json:"users"`
}

// Callback statuses.
const (
	StatusEditing        = 1
	StatusReadyToSave    = 2
	StatusSaveError      = 3
	StatusClosedNoChange = 4
	StatusForceSave      = 6
	StatusForceSaveError = 7
)

// ParseCallback verifies a callback. ONLYOFFICE signs the body in a "token"
// field, or in the Authorization header wrapped as {"payload": body}.
func ParseCallback(secret string, body []byte, authorization string) (*Callback, error) {
	var envelope struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(body, &envelope)
	var claims map[string]any
	var err error
	switch {
	case envelope.Token != "":
		claims, err = Verify(secret, envelope.Token)
	case strings.HasPrefix(authorization, "Bearer "):
		claims, err = Verify(secret, strings.TrimPrefix(authorization, "Bearer "))
		if payload, ok := claims["payload"].(map[string]any); err == nil && ok {
			claims = payload
		}
	default:
		return nil, errors.New("the callback is not signed")
	}
	if err != nil {
		return nil, err
	}
	raw, _ := json.Marshal(claims)
	var callback Callback
	if err := json.Unmarshal(raw, &callback); err != nil {
		return nil, err
	}
	return &callback, nil
}
