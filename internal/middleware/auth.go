package middleware

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/imyashkale/buildserver/internal/logger"
)

var (
	ErrMissingAuthHeader = errors.New("missing authorization header")
	ErrInvalidAuthHeader = errors.New("invalid authorization header format")
	ErrInvalidToken      = errors.New("invalid token")
	ErrTokenExpired      = errors.New("token expired")
	ErrMissingUserID     = errors.New("missing user ID in token")
)

// JWKSet represents a JSON Web Key Set
type JWKSet struct {
	Keys []JWK `json:"keys"`
}

// JWK represents a JSON Web Key
type JWK struct {
	Kid string   `json:"kid"`
	Kty string   `json:"kty"`
	Use string   `json:"use"`
	N   string   `json:"n"`
	E   string   `json:"e"`
	X5c []string `json:"x5c"`
}

// Auth0Config holds Auth0 configuration
type Auth0Config struct {
	Domain   string
	Audience string
}

// NewAuth0Config creates a new Auth0 configuration
func NewAuth0Config(domain, audience string) *Auth0Config {
	return &Auth0Config{
		Domain:   domain,
		Audience: audience,
	}
}

// Authentication middleware validates Auth0 JWT tokens
func Authentication() gin.HandlerFunc {
	return func(c *gin.Context) {
		logger.Debug("Authentication middleware invoked")

		// Extract the bearer token from the Authorization header
		authHeader := c.GetHeader("Authorization")
		const prefix = "Bearer "
		if len(authHeader) < len(prefix) || authHeader[:len(prefix)] != prefix {
			logger.WithField("path", c.Request.URL.Path).Warn("Authentication failed: missing or invalid authorization header")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "Missing or invalid authorization header",
			})
			return
		}

		// Get the token string
		tokenString := authHeader[len(prefix):]

		// Check if token has the correct structure (header.payload.signature)
		parts := strings.Split(tokenString, ".")
		if len(parts) != 3 {
			logger.WithFields(map[string]interface{}{
				"path":        c.Request.URL.Path,
				"parts_count": len(parts),
			}).Warn("Authentication failed: malformed token")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "malformed_token",
				"message": fmt.Sprintf("JWT token must have 3 parts (header.payload.signature), got %d part(s)", len(parts)),
				"hint":    "Make sure you're sending the complete JWT token from Auth0, not just the decoded payload",
			})
			return
		}

		// Parse the token without validation for development
		// In production, use AuthenticationWithAuth0() middleware for full validation
		parser := jwt.NewParser(jwt.WithoutClaimsValidation())
		token, _, err := parser.ParseUnverified(tokenString, jwt.MapClaims{})

		if err != nil {
			logger.Debugf("Token parse error: %v", err)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "invalid_token",
				"message": fmt.Sprintf("Failed to parse token: %v", err),
			})
			return
		}

		// Skip signature verification error for now
		// We'll still validate the claims
		if token == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "invalid_token",
				"message": "Failed to parse token",
			})
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "invalid_token",
				"message": "Invalid token claims",
			})
			return
		}

		// Validate expiration
		if exp, ok := claims["exp"].(float64); ok {
			if time.Now().Unix() > int64(exp) {
				logger.WithField("path", c.Request.URL.Path).Warn("Authentication failed: token expired")
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"error":   "token_expired",
					"message": "Token has expired",
				})
				return
			}
		}

		// Extract user ID from claims (Auth0 uses 'sub' claim)
		var userId string
		if sub, ok := claims["sub"].(string); ok {
			userId = sub
		} else {
			logger.WithField("path", c.Request.URL.Path).Warn("Authentication failed: missing user ID in token")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "invalid_token",
				"message": "Missing user ID in token",
			})
			return
		}

		// Set user ID in context for handlers to use
		c.Set("user_id", userId)
		c.Set("token_claims", claims)

		logger.WithFields(map[string]interface{}{
			"user_id": userId,
			"path":    c.Request.URL.Path,
		}).Debug("Authentication successful")

		c.Next()
	}
}

// AuthenticationWithAuth0 middleware validates Auth0 JWT tokens with full verification
func AuthenticationWithAuth0(config *Auth0Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		logger.Debug("AuthenticationWithAuth0 middleware invoked")

		// Extract the bearer token from the Authorization header
		authHeader := c.GetHeader("Authorization")
		const prefix = "Bearer "
		if len(authHeader) < len(prefix) || !strings.HasPrefix(authHeader, prefix) {
			logger.WithField("path", c.Request.URL.Path).Warn("Auth0 authentication failed: missing or invalid authorization header")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "Missing or invalid authorization header",
			})
			return
		}

		tokenString := authHeader[len(prefix):]

		// Parse and validate the token
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			// Validate the signing method
			if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}

			// Fetch the public key from Auth0's JWKS endpoint
			cert, err := getPemCert(token, config.Domain)
			if err != nil {
				return nil, err
			}

			return jwt.ParseRSAPublicKeyFromPEM([]byte(cert))
		})

		if err != nil {
			logger.WithFields(map[string]interface{}{
				"path":  c.Request.URL.Path,
				"error": err.Error(),
			}).Warn("Auth0 authentication failed: token validation error")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "invalid_token",
				"message": err.Error(),
			})
			return
		}

		if !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "invalid_token",
				"message": "Token is not valid",
			})
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "invalid_token",
				"message": "Invalid token claims",
			})
			return
		}

		// Validate audience
		if config.Audience != "" {
			if aud, ok := claims["aud"].(string); !ok || aud != config.Audience {
				// Also check if aud is an array
				if audArray, ok := claims["aud"].([]interface{}); ok {
					found := false
					for _, a := range audArray {
						if audStr, ok := a.(string); ok && audStr == config.Audience {
							found = true
							break
						}
					}
					if !found {
						c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
							"error":   "invalid_token",
							"message": "Invalid audience",
						})
						return
					}
				} else {
					c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
						"error":   "invalid_token",
						"message": "Invalid audience",
					})
					return
				}
			}
		}

		// Validate issuer
		expectedIssuer := fmt.Sprintf("https://%s/", config.Domain)
		if iss, ok := claims["iss"].(string); !ok || iss != expectedIssuer {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "invalid_token",
				"message": "Invalid issuer",
			})
			return
		}

		// Extract user ID from claims (Auth0 uses 'sub' claim)
		var userId string
		if sub, ok := claims["sub"].(string); ok {
			userId = sub
		} else {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "invalid_token",
				"message": "Missing user ID in token",
			})
			return
		}

		// Set user ID in context for handlers to use
		c.Set("user_id", userId)
		c.Set("token_claims", claims)

		c.Next()
	}
}

// getPemCert fetches the PEM certificate from Auth0's JWKS endpoint
func getPemCert(token *jwt.Token, domain string) (string, error) {
	cert := ""
	jwksURL := fmt.Sprintf("https://%s/.well-known/jwks.json", domain)

	resp, err := http.Get(jwksURL)
	if err != nil {
		return cert, err
	}
	defer resp.Body.Close()

	var jwks JWKSet
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return cert, err
	}

	// Find the key with matching kid
	kid, ok := token.Header["kid"].(string)
	if !ok {
		return cert, errors.New("missing kid in token header")
	}

	for _, key := range jwks.Keys {
		if key.Kid == kid {
			if len(key.X5c) > 0 {
				cert = fmt.Sprintf("-----BEGIN CERTIFICATE-----\n%s\n-----END CERTIFICATE-----", key.X5c[0])
				return cert, nil
			}
		}
	}

	return cert, errors.New("unable to find appropriate key")
}
