package middleware

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/justaboyhai-wq/fmind/internal/types"
	"github.com/sirupsen/logrus"
)

func TestSanitizeBody(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "camelCase apiKey",
			in:   `{"modelName":"gpt-5.2","apiKey":"sk-secret-123","provider":"azure_openai"}`,
			want: `{"modelName":"gpt-5.2","apiKey":"***","provider":"azure_openai"}`,
		},
		{
			name: "snake_case api_key",
			in:   `{"api_key":"sk-secret-123"}`,
			want: `{"api_key":"***"}`,
		},
		{
			name: "PascalCase APIKey",
			in:   `{"APIKey":"sk-secret-123"}`,
			want: `{"APIKey":"***"}`,
		},
		{
			name: "object storage credentials in camelCase",
			in:   `{"secretAccessKey":"abc","accessKeyId":"id","accessKey":"key"}`,
			want: `{"secretAccessKey":"***","accessKeyId":"***","accessKey":"***"}`,
		},
		{
			name: "object storage credentials in snake_case",
			in:   `{"secret_access_key":"abc","access_key_id":"id","access_key":"key"}`,
			want: `{"secret_access_key":"***","access_key_id":"***","access_key":"***"}`,
		},
		{
			name: "refreshToken / accessToken camelCase",
			in:   `{"refreshToken":"rt","accessToken":"at"}`,
			want: `{"refreshToken":"***","accessToken":"***"}`,
		},
		{
			name: "password and token preserved as masked",
			in:   `{"password":"p","token":"t"}`,
			want: `{"password":"***","token":"***"}`,
		},
		{
			name: "snake_case new_password and old_password",
			in:   `{"email":"alice@example.com","new_password":"FreshPass9","old_password":"OldPass9"}`,
			want: `{"email":"alice@example.com","new_password":"***","old_password":"***"}`,
		},
		{
			name: "extra whitespace around colon",
			in:   `{"apiKey"  :   "leak"}`,
			want: `{"apiKey":"***"}`,
		},
		{
			name: "authorization variants and credentials",
			in:   `{"authorization":"Bearer token","proxyAuthorization":"Basic token","credentials":"value"}`,
			want: `{"authorization":"***","proxyAuthorization":"***","credentials":"***"}`,
		},
		{
			name: "agent binding response secrets in all naming styles",
			in:   `{"connector_secret":"fmind_plaintext","ConnectorSecret":"fmind_pascal","bindingToken":"signed.jwt.value","BINDING_TOKEN":"signed.jwt.upper"}`,
			want: `{"connector_secret":"***","ConnectorSecret":"***","bindingToken":"***","BINDING_TOKEN":"***"}`,
		},
		{
			name: "non sensitive fields untouched",
			in:   `{"baseUrl":"https://example.com","modelName":"gpt"}`,
			want: `{"baseUrl":"https://example.com","modelName":"gpt"}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := sanitizeBody(tc.in)
			if got != tc.want {
				t.Errorf("sanitizeBody(%q)\n got: %s\nwant: %s", tc.in, got, tc.want)
			}
		})
	}
}

func TestSanitizeBindingEndpointsExcludeBodiesFromLogs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []string{
		"/internal/v1/agent-bindings/introspect",
		"/api/v1/agent-bindings",
		"/api/v1/agent-bindings/binding-1/rotate-key",
	}

	for _, path := range tests {
		t.Run(path, func(t *testing.T) {
			var captured bytes.Buffer
			testLogger := logrus.New()
			testLogger.SetOutput(&captured)
			testLogger.SetFormatter(&logrus.JSONFormatter{})
			entry := logrus.NewEntry(testLogger)

			router := gin.New()
			router.ContextWithFallback = true
			router.Use(func(c *gin.Context) {
				c.Request = c.Request.WithContext(context.WithValue(
					c.Request.Context(), types.LoggerContextKey, entry,
				))
				c.Next()
			})
			router.Use(Logger())
			router.POST(path, func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{
					"connector_secret": "fmind_response_plaintext",
					"binding_token":    "signed.response.jwt",
				})
			})
			request := httptest.NewRequest(http.MethodPost, path,
				strings.NewReader(`{"payload":"fmind_request_plaintext","jwt":"signed.request.jwt"}`))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)

			logOutput := captured.String()
			if !strings.Contains(logOutput, path) || !strings.Contains(logOutput, http.MethodPost) {
				t.Fatalf("expected request metadata in captured log, got: %s", logOutput)
			}
			for _, forbidden := range []string{
				"request_body", "response_body",
				"fmind_request_plaintext", "signed.request.jwt",
				"fmind_response_plaintext", "signed.response.jwt",
			} {
				if strings.Contains(logOutput, forbidden) {
					t.Fatalf("sensitive endpoint log contains %q: %s", forbidden, logOutput)
				}
			}
		})
	}
}
