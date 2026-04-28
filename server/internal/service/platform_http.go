package service

import (
	"net/http"
	"strings"
)

const (
	platformAndroid = "android"
	platformIOS     = "ios"
)

// platformFromUserAgent classifies an HTTP request as iOS vs Android based on
// the User-Agent header. Unity's UnityWebRequest does not set a UA on iOS, so
// CFNetwork's default ("<bundle>/<build> CFNetwork/x Darwin/x") is what arrives;
// on Android Unity sets "UnityPlayer/... (UnityWebRequest/...)". Any other UA
// (or none) is treated as Android, matching model.DefaultPlatform.
func platformFromUserAgent(r *http.Request) string {
	ua := r.Header.Get("User-Agent")
	if strings.Contains(ua, "Darwin/") || strings.Contains(ua, "CFNetwork/") {
		return platformIOS
	}
	return platformAndroid
}
