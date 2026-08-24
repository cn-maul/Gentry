package web

import (
	"crypto/subtle"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

const authTokenEnv = "ALTERBOT_AUTH_TOKEN"

var blockedURLSchemes = map[string]struct{}{
	"http":  {},
	"https": {},
}

func requireAuth() gin.HandlerFunc {
	expected := strings.TrimSpace(os.Getenv(authTokenEnv))
	return func(c *gin.Context) {
		if expected == "" {
			c.Next()
			return
		}

		provided := strings.TrimSpace(c.GetHeader("Authorization"))
		if strings.HasPrefix(strings.ToLower(provided), "bearer ") {
			provided = strings.TrimSpace(provided[7:])
		}
		if subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, NewErrorResponse(401, "unauthorized"))
			return
		}
		c.Next()
	}
}

func validateOutboundURL(raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return fmt.Errorf("invalid url")
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("url must include scheme and host")
	}
	if _, ok := blockedURLSchemes[strings.ToLower(parsed.Scheme)]; !ok {
		return fmt.Errorf("unsupported url scheme")
	}

	hostname := parsed.Hostname()
	if hostname == "" {
		return fmt.Errorf("invalid host")
	}
	if isBlockedHostname(hostname) {
		return fmt.Errorf("host is not allowed")
	}
	return nil
}

func isBlockedHostname(host string) bool {
	lower := strings.ToLower(strings.TrimSpace(host))
	if lower == "localhost" {
		return true
	}
	if addr, err := netip.ParseAddr(lower); err == nil {
		return isBlockedIP(addr)
	}
	ips, err := net.LookupIP(lower)
	if err != nil {
		return true
	}
	for _, ip := range ips {
		addr, ok := netip.AddrFromSlice(ip)
		if !ok {
			return true
		}
		if isBlockedIP(addr.Unmap()) {
			return true
		}
	}
	return false
}

func isBlockedIP(addr netip.Addr) bool {
	return addr.IsLoopback() ||
		addr.IsPrivate() ||
		addr.IsLinkLocalUnicast() ||
		addr.IsLinkLocalMulticast() ||
		addr.IsMulticast() ||
		addr.IsUnspecified()
}

func maskSecret(secret string) string {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return ""
	}
	if len(secret) <= 6 {
		return "******"
	}
	return secret[:3] + "****" + secret[len(secret)-3:]
}

func maskSensitiveConfig(service string, config map[string]interface{}) map[string]interface{} {
	masked := make(map[string]interface{}, len(config))
	for key, value := range config {
		masked[key] = value
	}
	switch service {
	case "pushplus":
		if token, ok := masked["token"].(string); ok {
			masked["token"] = maskSecret(token)
		}
	case "serverchan":
		if sendkey, ok := masked["sendkey"].(string); ok {
			masked["sendkey"] = maskSecret(sendkey)
		}
	case "webhook":
		if webhookURL, ok := masked["url"].(string); ok {
			masked["url"] = maskSecret(webhookURL)
		}
	case "bark":
		if key, ok := masked["key"].(string); ok {
			masked["key"] = maskSecret(key)
		}
	}
	return masked
}
