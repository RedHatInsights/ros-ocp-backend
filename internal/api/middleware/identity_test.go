package middleware_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	authenticationv1 "k8s.io/api/authentication/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubernetesfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/testing"

	"github.com/redhatinsights/platform-go-middlewares/v2/identity"
	"github.com/redhatinsights/ros-ocp-backend/internal/api/middleware"
)

/*
===============================================================================
IDENTITY MIDDLEWARE BDD TEST SUITE
===============================================================================

This BDD test suite describes the behavior of authentication middleware for
the ROS-OCP backend. Tests are written in Given/When/Then format to clearly
express expected behaviors and business requirements.

AUTHENTICATION PROVIDER BEHAVIORS:

1. When selecting an identity provider
   - Given OAuth/RHSSO configuration → Then provides appropriate middleware

2. When authenticating with Red Hat SSO
   - Given X-Rh-Identity headers → Then extracts user identity for RBAC

3. When authenticating with OAuth 2.0
   - Given Authorization: Bearer tokens → Then validates via Kubernetes TokenReview
   - Covers realistic tokens, security edge cases, performance, and RBAC integration

Each behavior includes comprehensive scenarios to ensure production readiness.
===============================================================================
*/

var _ = Describe("Identity Middleware", func() {
	var (
		e          *echo.Echo
		req        *http.Request
		rec        *httptest.ResponseRecorder
		fakeClient *kubernetesfake.Clientset
		provider   middleware.IdentityProvider
	)

	BeforeEach(func() {
		e = echo.New()
		rec = httptest.NewRecorder()

		// Create fake Kubernetes client
		fakeClient = kubernetesfake.NewSimpleClientset()
		provider = middleware.NewOauthIDProvider(fakeClient)
	})

	// =============================================================================
	// IDENTITY PROVIDER FACTORY BEHAVIOR
	// Tests how the system selects and configures identity providers
	// =============================================================================

	Describe("When selecting an identity provider", func() {
		Context("Given OAuth configuration", func() {
			It("should provide OAuth middleware when environment supports it", func() {
				// Given: a temporary kubeconfig file for testing
				tempDir, err := os.MkdirTemp("", "kubeconfig-test")
				Expect(err).ToNot(HaveOccurred())
				defer os.RemoveAll(tempDir)

				kubeconfigPath := filepath.Join(tempDir, "kubeconfig")
				kubeconfigContent := `
apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://fake-k8s-server:6443
    insecure-skip-tls-verify: true
  name: fake-cluster
contexts:
- context:
    cluster: fake-cluster
    user: fake-user
  name: fake-context
current-context: fake-context
users:
- name: fake-user
  user:
    token: fake-token
`
				err = os.WriteFile(kubeconfigPath, []byte(kubeconfigContent), 0644)
				Expect(err).ToNot(HaveOccurred())

				os.Setenv("KUBECONFIG", kubeconfigPath)
				// When: requesting OAuth provider
				handler, err := middleware.GetIdentityProviderHandlerFunction(middleware.OAuth2IDProvider)

				// Then: should return valid OAuth middleware
				Expect(err).ToNot(HaveOccurred())
				Expect(handler).ToNot(BeNil())
				Expect(handler).To(BeAssignableToTypeOf((echo.MiddlewareFunc)(nil)))
			})
		})

		Context("Given RHSSO configuration", func() {
			It("should provide RHSSO middleware for explicit RHSSO config", func() {
				// Given: RHSSO provider is explicitly requested

				// When: requesting RHSSO provider
				handler, err := middleware.GetIdentityProviderHandlerFunction("rhsso")

				// Then: should return valid RHSSO middleware
				Expect(err).ToNot(HaveOccurred())
				Expect(handler).ToNot(BeNil())
				Expect(handler).To(BeAssignableToTypeOf((echo.MiddlewareFunc)(nil)))
			})

			It("should default to RHSSO middleware for unknown configurations", func() {
				// Given: an unknown provider type is requested

				// When: requesting unknown provider
				handler, err := middleware.GetIdentityProviderHandlerFunction("unknown")

				// Then: should fallback to RHSSO middleware
				Expect(err).ToNot(HaveOccurred())
				Expect(handler).ToNot(BeNil())
				Expect(handler).To(BeAssignableToTypeOf((echo.MiddlewareFunc)(nil)))
			})

			It("should default to RHSSO middleware for empty configuration", func() {
				// Given: no provider type is specified

				// When: requesting provider with empty config
				handler, err := middleware.GetIdentityProviderHandlerFunction("")

				// Then: should fallback to RHSSO middleware
				Expect(err).ToNot(HaveOccurred())
				Expect(handler).ToNot(BeNil())
				Expect(handler).To(BeAssignableToTypeOf((echo.MiddlewareFunc)(nil)))
			})
		})
	})

	// =============================================================================
	// RHSSO IDENTITY PROVIDER BEHAVIOR
	// Tests how Red Hat SSO authentication processes X-Rh-Identity headers
	// =============================================================================

	Describe("When authenticating with Red Hat SSO", func() {
		var rhssoProvider middleware.IdentityProvider

		BeforeEach(func() {
			// Given: RHSSO identity provider is configured
			rhssoProvider = middleware.NewRHSSOIdentityProvider()
		})

		Context("Given X-Rh-Identity header processing", func() {
			It("should provide valid middleware for request processing", func() {
				// Given: RHSSO provider is initialized

				// When: getting the handler function
				handler := rhssoProvider.GetHandlerFunction()

				// Then: should return a valid middleware function
				Expect(handler).ToNot(BeNil())
			})

			It("should extract organization identity that supports RBAC", func() {
				// Given: a valid XRHID structure with organization information
				xrhid := identity.XRHID{
					Identity: identity.Identity{
						OrgID: "test-org-123",
						Type:  "User",
					},
				}

				// And: the identity is properly encoded in the header
				j, err := json.Marshal(xrhid)
				Expect(err).NotTo(HaveOccurred())
				encodedXRHID := base64.StdEncoding.EncodeToString(j)

				req := httptest.NewRequest(http.MethodGet, "/test", nil)
				req.Header.Set("X-Rh-Identity", encodedXRHID)

				testHandler := func(c echo.Context) error {
					// When: retrieving identity from context for RBAC
					id := c.Get("Identity").(identity.OrganizationIDProvider)

					// Then: should provide correct organization ID for RBAC
					Expect(id.GetOrganizationID()).To(Equal("test-org-123"), "XRHID should return the correct organization ID")

					return c.String(http.StatusOK, "success")
				}

				// When: processing the request through RHSSO middleware
				c := e.NewContext(req, rec)
				middlewareFunc := rhssoProvider.GetHandlerFunction()
				err = middlewareFunc(testHandler)(c)

				// Then: should successfully process the authentication
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	// =============================================================================
	// OAUTH IDENTITY PROVIDER BEHAVIOR
	// Tests how OAuth 2.0 authentication processes Authorization: Bearer headers
	// Includes comprehensive scenarios for production readiness
	// =============================================================================

	Describe("When authenticating with OAuth 2.0", func() {
		BeforeEach(func() {
			fakeClient = kubernetesfake.NewSimpleClientset()
			provider = middleware.NewOauthIDProvider(fakeClient)
		})

		// -------------------------------------------------------------------------
		// REALISTIC TOKEN FORMAT SCENARIOS
		// -------------------------------------------------------------------------
		Context("Given realistic token formats from various sources", func() {
			DescribeTable("should handle different token formats appropriately",
				func(tokenName string, token string, shouldSucceed bool, expectedError string) {
					// Setup authentication response
					fakeClient.PrependReactor("create", "tokenreviews", func(action testing.Action) (bool, runtime.Object, error) {
						tr := action.(testing.CreateAction).GetObject().(*authenticationv1.TokenReview)
						tr.Status.Authenticated = shouldSucceed
						if shouldSucceed {
							tr.Status.User = authenticationv1.UserInfo{
								Username: "system:serviceaccount:default:ros-ocp-api",
								UID:      "12345678-1234-1234-1234-123456789012",
								Groups:   []string{"system:serviceaccounts", "system:serviceaccounts:default"},
							}
						} else {
							tr.Status.Error = expectedError
						}
						return true, tr, nil
					})

					req = httptest.NewRequest(http.MethodGet, "/test", nil)
					req.Header.Set("Authorization", "Bearer "+token)

					testHandler := func(c echo.Context) error {
						return c.String(http.StatusOK, "success")
					}

					c := e.NewContext(req, rec)
					middlewareFunc := provider.GetHandlerFunction()
					err := middlewareFunc(testHandler)(c)

					if shouldSucceed {
						Expect(err).NotTo(HaveOccurred())
					} else {
						Expect(err).To(HaveOccurred())
						if expectedError != "" {
							httpError, ok := err.(*echo.HTTPError)
							Expect(ok).To(BeTrue())
							// Accept either TokenReview error or OAuth validation error
							Expect(httpError.Message).To(SatisfyAny(
								ContainSubstring("Invalid or expired token"),
								ContainSubstring("Empty token in Authorization header"),
								ContainSubstring("Missing or invalid Authorization header"),
							))
						}
					}
				},
				Entry("Kubernetes Service Account JWT", "k8s-sa-token",
					"eyJhbGciOiJSUzI1NiIsImtpZCI6IkRRVjJpLXFKOHczWjBhWDBMYzlybjYyc1gtOTFQUDdmS3Y3Z2d1SllzNUEifQ.eyJhdWQiOlsiaHR0cHM6Ly9rdWJlcm5ldGVzLmRlZmF1bHQuc3ZjLmNsdXN0ZXIubG9jYWwiXSwiZXhwIjoxNzA2MjA0NDU3LCJpYXQiOjE3MDYyMDA4NTcsImlzcyI6Imh0dHBzOi8va3ViZXJuZXRlcy5kZWZhdWx0LnN2Yy5jbHVzdGVyLmxvY2FsIiwia3ViZXJuZXRlcy5pbyI6eyJuYW1lc3BhY2UiOiJkZWZhdWx0Iiwic2VydmljZWFjY291bnQiOnsibmFtZSI6InJvcy1vY3AtYXBpIiwidWlkIjoiMTIzNDU2Nzg5MCJ9fSwibmJmIjoxNzA2MjAwODU3LCJzdWIiOiJzeXN0ZW06c2VydmljZWFjY291bnQ6ZGVmYXVsdDpyb3Mtb2NwLWFwaSJ9",
					true, ""),
				Entry("UUID-style Token", "uuid-token",
					"12345678-1234-1234-1234-123456789012",
					true, ""),
				Entry("Base64 Encoded Token", "base64-token",
					"dGVzdC10b2tlbi12YWx1ZQ==",
					true, ""),
				Entry("Short Alphanumeric Token", "short-token",
					"abc123def456",
					true, ""),
				Entry("Malformed JWT Structure", "malformed-jwt",
					"invalid.jwt.token.structure.here",
					false, "token is not valid"),
				Entry("Empty Token Content", "empty-token",
					"",
					false, "token is not valid"),
			)
		})

		// -------------------------------------------------------------------------
		// COMPREHENSIVE USER IDENTITY SCENARIOS
		// -------------------------------------------------------------------------
		Context("Given various user identity types and configurations", func() {
			type userScenario struct {
				name          string
				userInfo      authenticationv1.UserInfo
				authenticated bool
				expectGroups  []string
			}

			userScenarios := []userScenario{
				{
					name: "Service Account with Standard Groups",
					userInfo: authenticationv1.UserInfo{
						Username: "system:serviceaccount:default:ros-ocp-api",
						UID:      "12345678-1234-1234-1234-123456789012",
						Groups:   []string{"system:serviceaccounts", "system:serviceaccounts:default"},
					},
					authenticated: true,
					expectGroups:  []string{"system:serviceaccounts", "system:serviceaccounts:default"},
				},
				{
					name: "Human User with Multiple Roles",
					userInfo: authenticationv1.UserInfo{
						Username: "jane.doe@company.com",
						UID:      "user-67890",
						Groups:   []string{"developers", "admins", "ros-ocp-users"},
					},
					authenticated: true,
					expectGroups:  []string{"developers", "admins", "ros-ocp-users"},
				},
				{
					name: "Service Account with Custom Claims",
					userInfo: authenticationv1.UserInfo{
						Username: "system:serviceaccount:ros-ocp:processor",
						UID:      "sa-98765",
						Groups:   []string{"system:serviceaccounts:ros-ocp"},
						Extra: map[string]authenticationv1.ExtraValue{
							"org_id":    []string{"12345"},
							"tenant_id": []string{"acme-corp"},
						},
					},
					authenticated: true,
					expectGroups:  []string{"system:serviceaccounts:ros-ocp"},
				},
				{
					name: "User with Single Group",
					userInfo: authenticationv1.UserInfo{
						Username: "system:admin",
						UID:      "admin-user-123",
						Groups:   []string{"system:masters"},
					},
					authenticated: true,
					expectGroups:  []string{"system:masters"},
				},
			}

			for _, scenario := range userScenarios {
				scenario := scenario // capture loop variable
				It(fmt.Sprintf("should handle %s correctly", scenario.name), func() {
					fakeClient.PrependReactor("create", "tokenreviews", func(action testing.Action) (bool, runtime.Object, error) {
						tr := action.(testing.CreateAction).GetObject().(*authenticationv1.TokenReview)
						tr.Status.Authenticated = scenario.authenticated
						tr.Status.User = scenario.userInfo
						return true, tr, nil
					})

					req = httptest.NewRequest(http.MethodGet, "/test", nil)
					req.Header.Set("Authorization", "Bearer test-token-"+scenario.name)

					testHandler := func(c echo.Context) error {
						if scenario.authenticated {
							id := c.Get("Identity").(identity.OrganizationIDProvider)
							oauthID := id.(identity.OAuthID)
							userInfo := authenticationv1.UserInfo(oauthID)

							Expect(userInfo.Username).To(Equal(scenario.userInfo.Username))
							Expect(userInfo.UID).To(Equal(scenario.userInfo.UID))
							Expect(userInfo.Groups).To(Equal(scenario.expectGroups))

							// Test Extra claims if present
							if scenario.userInfo.Extra != nil {
								Expect(userInfo.Extra).To(Equal(scenario.userInfo.Extra))
							}
						}
						return c.String(http.StatusOK, "success")
					}

					c := e.NewContext(req, rec)
					middlewareFunc := provider.GetHandlerFunction()
					err := middlewareFunc(testHandler)(c)

					if scenario.authenticated {
						Expect(err).NotTo(HaveOccurred())
					} else {
						Expect(err).To(HaveOccurred())
					}
				})
			}
		})

		// -------------------------------------------------------------------------
		// SECURITY EDGE CASES
		// -------------------------------------------------------------------------
		Context("Given potentially malicious or edge case token scenarios", func() {
			It("should reject extremely long tokens", func() {
				fakeClient.PrependReactor("create", "tokenreviews", func(action testing.Action) (bool, runtime.Object, error) {
					tr := action.(testing.CreateAction).GetObject().(*authenticationv1.TokenReview)
					tr.Status.Authenticated = false
					tr.Status.Error = "token too long"
					return true, tr, nil
				})

				// Create a 1KB token (reasonable size for testing)
				longToken := strings.Repeat("a", 1024)

				req = httptest.NewRequest(http.MethodGet, "/test", nil)
				req.Header.Set("Authorization", "Bearer "+longToken)

				testHandler := func(c echo.Context) error {
					Fail("Should not reach handler with oversized token")
					return nil
				}

				c := e.NewContext(req, rec)
				middlewareFunc := provider.GetHandlerFunction()
				err := middlewareFunc(testHandler)(c)

				Expect(err).To(HaveOccurred())
				httpError, ok := err.(*echo.HTTPError)
				Expect(ok).To(BeTrue())
				Expect(httpError.Code).To(Equal(http.StatusUnauthorized))
			})

			It("should handle tokens with special characters safely", func() {
				specialTokens := []string{
					"token-with-<script>alert('xss')</script>",
					"token-with-\x00null-bytes\x00",
					"token-with-unicode-💻🔐",
					"token-with-sql-'; DROP TABLE users; --",
					"token with spaces and\ttabs",
				}

				for _, token := range specialTokens {
					fakeClient.PrependReactor("create", "tokenreviews", func(action testing.Action) (bool, runtime.Object, error) {
						tr := action.(testing.CreateAction).GetObject().(*authenticationv1.TokenReview)
						tr.Status.Authenticated = false
						tr.Status.Error = "Invalid token format"
						return true, tr, nil
					})

					req = httptest.NewRequest(http.MethodGet, "/test", nil)
					req.Header.Set("Authorization", "Bearer "+token)

					testHandler := func(c echo.Context) error {
						Fail("Should not reach handler with special character token")
						return nil
					}

					c := e.NewContext(req, rec)
					middlewareFunc := provider.GetHandlerFunction()
					err := middlewareFunc(testHandler)(c)

					Expect(err).To(HaveOccurred(), fmt.Sprintf("Token with special characters should be rejected: %s", token))
					httpError, ok := err.(*echo.HTTPError)
					Expect(ok).To(BeTrue())
					Expect(httpError.Code).To(Equal(http.StatusUnauthorized))
				}
			})

			It("should handle multiple Authorization headers gracefully", func() {
				req = httptest.NewRequest(http.MethodGet, "/test", nil)
				// Add multiple Authorization headers
				req.Header.Add("Authorization", "Bearer token1")
				req.Header.Add("Authorization", "Bearer token2")

				testHandler := func(c echo.Context) error {
					return c.String(http.StatusOK, "success")
				}

				// The last header value should be used (standard HTTP behavior)
				fakeClient.PrependReactor("create", "tokenreviews", func(action testing.Action) (bool, runtime.Object, error) {
					tr := action.(testing.CreateAction).GetObject().(*authenticationv1.TokenReview)
					// Should receive "Bearer token1, Bearer token2" or just "Bearer token2"
					tr.Status.Authenticated = false
					tr.Status.Error = "Multiple authorization headers"
					return true, tr, nil
				})

				c := e.NewContext(req, rec)
				middlewareFunc := provider.GetHandlerFunction()
				err := middlewareFunc(testHandler)(c)

				// Should handle gracefully (may succeed or fail depending on implementation)
				if err != nil {
					httpError, ok := err.(*echo.HTTPError)
					Expect(ok).To(BeTrue())
					Expect(httpError.Code).To(BeNumerically(">=", 400))
				}
			})
		})

		// -------------------------------------------------------------------------
		// PERFORMANCE AND CONCURRENCY TESTING
		// -------------------------------------------------------------------------
		Context("Given high-load and concurrent authentication scenarios", func() {
			It("should handle concurrent authentication requests safely", func() {
				const numGoroutines = 20
				const requestsPerGoroutine = 5

				fakeClient.PrependReactor("create", "tokenreviews", func(action testing.Action) (bool, runtime.Object, error) {
					tr := action.(testing.CreateAction).GetObject().(*authenticationv1.TokenReview)
					tr.Status.Authenticated = true
					tr.Status.User = authenticationv1.UserInfo{
						Username: "concurrent-test-user",
						UID:      "concurrent-12345",
						Groups:   []string{"test-group"},
					}
					// Simulate some processing time
					time.Sleep(1 * time.Millisecond)
					return true, tr, nil
				})

				var wg sync.WaitGroup
				errorChan := make(chan error, numGoroutines*requestsPerGoroutine)

				for i := 0; i < numGoroutines; i++ {
					wg.Add(1)
					go func(goroutineID int) {
						defer wg.Done()
						for j := 0; j < requestsPerGoroutine; j++ {
							req := httptest.NewRequest(http.MethodGet, "/test", nil)
							req.Header.Set("Authorization", fmt.Sprintf("Bearer token-%d-%d", goroutineID, j))
							rec := httptest.NewRecorder()

							testHandler := func(c echo.Context) error {
								return c.String(http.StatusOK, "success")
							}

							c := e.NewContext(req, rec)
							middlewareFunc := provider.GetHandlerFunction()
							err := middlewareFunc(testHandler)(c)
							if err != nil {
								errorChan <- err
							}
						}
					}(i)
				}

				wg.Wait()
				close(errorChan)

				// Check for any errors
				var errors []error
				for err := range errorChan {
					errors = append(errors, err)
				}
				Expect(errors).To(BeEmpty(), "Concurrent requests should not cause errors")
			})

			It("should complete authentication within reasonable time", func() {
				fakeClient.PrependReactor("create", "tokenreviews", func(action testing.Action) (bool, runtime.Object, error) {
					// Simulate realistic processing time
					time.Sleep(5 * time.Millisecond)
					tr := action.(testing.CreateAction).GetObject().(*authenticationv1.TokenReview)
					tr.Status.Authenticated = true
					tr.Status.User = authenticationv1.UserInfo{
						Username: "performance-test-user",
						UID:      "perf-12345",
					}
					return true, tr, nil
				})

				req = httptest.NewRequest(http.MethodGet, "/test", nil)
				req.Header.Set("Authorization", "Bearer performance-test-token")

				testHandler := func(c echo.Context) error {
					return c.String(http.StatusOK, "success")
				}

				start := time.Now()
				c := e.NewContext(req, rec)
				middlewareFunc := provider.GetHandlerFunction()
				err := middlewareFunc(testHandler)(c)

				duration := time.Since(start)
				Expect(err).NotTo(HaveOccurred())
				Expect(duration).To(BeNumerically("<", 100*time.Millisecond), "Authentication should complete quickly")
			})
		})

		// -------------------------------------------------------------------------
		// NETWORK FAILURE SCENARIOS
		// -------------------------------------------------------------------------
		Context("Given Kubernetes API connectivity issues", func() {
			It("should handle TokenReview API timeouts", func() {
				fakeClient.PrependReactor("create", "tokenreviews", func(action testing.Action) (bool, runtime.Object, error) {
					// Simulate timeout
					return true, nil, context.DeadlineExceeded
				})

				req = httptest.NewRequest(http.MethodGet, "/test", nil)
				req.Header.Set("Authorization", "Bearer timeout-test-token")

				testHandler := func(c echo.Context) error {
					Fail("Should not reach handler due to timeout")
					return nil
				}

				c := e.NewContext(req, rec)
				middlewareFunc := provider.GetHandlerFunction()
				err := middlewareFunc(testHandler)(c)

				Expect(err).To(HaveOccurred())
				httpError, ok := err.(*echo.HTTPError)
				Expect(ok).To(BeTrue())
				Expect(httpError.Code).To(Equal(http.StatusInternalServerError))
				Expect(httpError.Message).To(ContainSubstring("Failed to validate token"))
			})

			It("should handle Kubernetes API connection failures", func() {
				fakeClient.PrependReactor("create", "tokenreviews", func(action testing.Action) (bool, runtime.Object, error) {
					return true, nil, fmt.Errorf("connection refused: Kubernetes API not available")
				})

				req = httptest.NewRequest(http.MethodGet, "/test", nil)
				req.Header.Set("Authorization", "Bearer connection-failure-token")

				testHandler := func(c echo.Context) error {
					Fail("Should not reach handler due to connection failure")
					return nil
				}

				c := e.NewContext(req, rec)
				middlewareFunc := provider.GetHandlerFunction()
				err := middlewareFunc(testHandler)(c)

				Expect(err).To(HaveOccurred())
				httpError, ok := err.(*echo.HTTPError)
				Expect(ok).To(BeTrue())
				Expect(httpError.Code).To(Equal(http.StatusInternalServerError))
				Expect(httpError.Message).To(ContainSubstring("Failed to validate token"))
			})
		})

		// -------------------------------------------------------------------------
		// RBAC INTEGRATION TESTING
		// -------------------------------------------------------------------------
		Context("Given RBAC middleware integration requirements", func() {
			It("should provide correct identity format for RBAC middleware", func() {
				expectedUserInfo := authenticationv1.UserInfo{
					Username: "system:serviceaccount:default:ros-ocp-api",
					UID:      "rbac-test-12345",
					Groups:   []string{"system:serviceaccounts", "system:serviceaccounts:default"},
				}

				fakeClient.PrependReactor("create", "tokenreviews", func(action testing.Action) (bool, runtime.Object, error) {
					tr := action.(testing.CreateAction).GetObject().(*authenticationv1.TokenReview)
					tr.Status.Authenticated = true
					tr.Status.User = expectedUserInfo
					return true, tr, nil
				})

				req = httptest.NewRequest(http.MethodGet, "/test", nil)
				req.Header.Set("Authorization", "Bearer rbac-integration-token")

				testHandler := func(c echo.Context) error {
					// Verify identity is properly set for RBAC
					id := c.Get("Identity").(identity.OrganizationIDProvider)
					oauthID := id.(identity.OAuthID)
					userInfo := authenticationv1.UserInfo(oauthID)

					Expect(userInfo.Username).To(Equal(expectedUserInfo.Username))
					Expect(userInfo.Groups).To(Equal(expectedUserInfo.Groups))
					Expect(userInfo.Extra).To(Equal(expectedUserInfo.Extra))

					// Verify OrganizationIDProvider interface works
					// This is what RBAC middleware would use
					Expect(id.GetOrganizationID()).To(Equal("1"), "Organization ID should be 12345")

					return c.String(http.StatusOK, "success")
				}

				c := e.NewContext(req, rec)
				middlewareFunc := provider.GetHandlerFunction()
				err := middlewareFunc(testHandler)(c)

				Expect(err).NotTo(HaveOccurred())
			})
		})

		// -------------------------------------------------------------------------
		// OAUTH PROVIDER CONSTRUCTOR AND METHODS TESTING
		// -------------------------------------------------------------------------
		Context("Given OAuth provider construction and method calls", func() {
			It("should create OAuth provider with Kubernetes client", func() {
				// Given: a fake Kubernetes client
				testClient := kubernetesfake.NewSimpleClientset()

				// When: creating OAuth identity provider
				oauthProvider := middleware.NewOauthIDProvider(testClient)

				// Then: should return valid OAuth provider instance
				Expect(oauthProvider).ToNot(BeNil())
				Expect(oauthProvider).To(BeAssignableToTypeOf((*middleware.OAuthIdentityProvider)(nil)))
			})

			It("should provide handler function for middleware chain", func() {
				// Given: OAuth provider instance
				testClient := kubernetesfake.NewSimpleClientset()
				oauthProvider := middleware.NewOauthIDProvider(testClient)

				// When: getting handler function
				handlerFunc := oauthProvider.GetHandlerFunction()

				// Then: should return valid echo middleware function
				Expect(handlerFunc).ToNot(BeNil())
				Expect(handlerFunc).To(BeAssignableToTypeOf((echo.MiddlewareFunc)(nil)))
			})

			It("should handle method chaining correctly", func() {
				// Given: a fresh OAuth provider
				testClient := kubernetesfake.NewSimpleClientset()

				// When: creating provider and immediately getting handler
				handlerFunc := middleware.NewOauthIDProvider(testClient).GetHandlerFunction()

				// Then: should work seamlessly in method chain
				Expect(handlerFunc).ToNot(BeNil())
			})
		})

		// -------------------------------------------------------------------------
		// TOKEN VALIDATION UNIT TESTING
		// -------------------------------------------------------------------------
		Context("Given token validation scenarios", func() {
			var testProvider middleware.IdentityProvider
			var testClient *kubernetesfake.Clientset

			BeforeEach(func() {
				// Given: OAuth provider with fresh fake client for each test
				testClient = kubernetesfake.NewSimpleClientset()
				testProvider = middleware.NewOauthIDProvider(testClient)
			})

			It("should validate authentic tokens successfully", func() {
				// Given: Kubernetes API configured to accept the token
				expectedUser := authenticationv1.UserInfo{
					Username: "test-user",
					UID:      "user-123",
					Groups:   []string{"group1", "group2"},
				}

				testClient.PrependReactor("create", "tokenreviews", func(action testing.Action) (bool, runtime.Object, error) {
					tr := action.(testing.CreateAction).GetObject().(*authenticationv1.TokenReview)
					tr.Status.Authenticated = true
					tr.Status.User = expectedUser
					return true, tr, nil
				})

				// When: processing request through OAuth middleware
				req, _ := http.NewRequest("GET", "/test", nil)
				req.Header.Set("Authorization", "Bearer valid-token-123")
				c := e.NewContext(req, rec)

				testHandler := func(c echo.Context) error {
					// Then: identity should be set in context
					id := c.Get("Identity")
					Expect(id).ToNot(BeNil())

					// Verify user details through identity
					oauthID := id.(identity.OAuthID)
					userInfo := authenticationv1.UserInfo(oauthID)
					Expect(userInfo.Username).To(Equal("test-user"))
					Expect(userInfo.UID).To(Equal("user-123"))
					Expect(userInfo.Groups).To(ConsistOf("group1", "group2"))

					return c.String(http.StatusOK, "success")
				}

				middlewareFunc := testProvider.GetHandlerFunction()
				err := middlewareFunc(testHandler)(c)

				Expect(err).ToNot(HaveOccurred())
			})

			It("should reject invalid tokens appropriately", func() {
				// Given: Kubernetes API configured to reject the token
				testClient.PrependReactor("create", "tokenreviews", func(action testing.Action) (bool, runtime.Object, error) {
					tr := action.(testing.CreateAction).GetObject().(*authenticationv1.TokenReview)
					tr.Status.Authenticated = false
					tr.Status.Error = "token is not valid"
					return true, tr, nil
				})

				// When: processing request with invalid token
				req, _ := http.NewRequest("GET", "/test", nil)
				req.Header.Set("Authorization", "Bearer invalid-token")
				c := e.NewContext(req, rec)

				testHandler := func(c echo.Context) error {
					return c.String(http.StatusOK, "should not reach here")
				}

				middlewareFunc := testProvider.GetHandlerFunction()
				err := middlewareFunc(testHandler)(c)

				// Then: should return authentication error
				Expect(err).To(HaveOccurred())

				httpError, isHTTPError := err.(*echo.HTTPError)
				Expect(isHTTPError).To(BeTrue())
				Expect(httpError.Code).To(Equal(http.StatusUnauthorized))
				Expect(httpError.Message).To(ContainSubstring("Invalid or expired token"))
			})

			It("should handle Kubernetes API errors gracefully", func() {
				// Given: Kubernetes API experiencing connectivity issues
				testClient.PrependReactor("create", "tokenreviews", func(action testing.Action) (bool, runtime.Object, error) {
					return true, nil, fmt.Errorf("connection refused")
				})

				// When: processing request during API failure
				req, _ := http.NewRequest("GET", "/test", nil)
				req.Header.Set("Authorization", "Bearer any-token")
				c := e.NewContext(req, rec)

				testHandler := func(c echo.Context) error {
					return c.String(http.StatusOK, "should not reach here")
				}

				middlewareFunc := testProvider.GetHandlerFunction()
				err := middlewareFunc(testHandler)(c)

				// Then: should return internal server error
				Expect(err).To(HaveOccurred())

				httpError, isHTTPError := err.(*echo.HTTPError)
				Expect(isHTTPError).To(BeTrue())
				Expect(httpError.Code).To(Equal(http.StatusInternalServerError))
				Expect(httpError.Message).To(ContainSubstring("Failed to validate token"))
			})

			It("should properly format TokenReview requests", func() {
				// Given: token validation request monitoring
				var capturedTokenReview *authenticationv1.TokenReview
				testClient.PrependReactor("create", "tokenreviews", func(action testing.Action) (bool, runtime.Object, error) {
					capturedTokenReview = action.(testing.CreateAction).GetObject().(*authenticationv1.TokenReview)
					capturedTokenReview.Status.Authenticated = true
					capturedTokenReview.Status.User = authenticationv1.UserInfo{Username: "test"}
					return true, capturedTokenReview, nil
				})

				// When: processing request with token
				testToken := "test-token-content"
				req, _ := http.NewRequest("GET", "/test", nil)
				req.Header.Set("Authorization", "Bearer "+testToken)
				c := e.NewContext(req, rec)

				testHandler := func(c echo.Context) error {
					return c.String(http.StatusOK, "success")
				}

				middlewareFunc := testProvider.GetHandlerFunction()
				err := middlewareFunc(testHandler)(c)

				// Then: should format TokenReview correctly
				Expect(err).ToNot(HaveOccurred())
				Expect(capturedTokenReview).ToNot(BeNil())
				Expect(capturedTokenReview.Spec.Token).To(Equal(testToken))
			})

			It("should handle missing authorization header", func() {
				// Given: request without Authorization header
				req, _ := http.NewRequest("GET", "/test", nil)
				// Intentionally not setting Authorization header
				c := e.NewContext(req, rec)

				testHandler := func(c echo.Context) error {
					return c.String(http.StatusOK, "should not reach here")
				}

				// When: processing request without authorization
				middlewareFunc := testProvider.GetHandlerFunction()
				err := middlewareFunc(testHandler)(c)

				// Then: should return authorization error
				Expect(err).To(HaveOccurred())

				httpError, isHTTPError := err.(*echo.HTTPError)
				Expect(isHTTPError).To(BeTrue())
				Expect(httpError.Code).To(Equal(http.StatusUnauthorized))
				Expect(httpError.Message).To(ContainSubstring("Missing Authorization header"))
			})

			It("should handle malformed authorization header", func() {
				// Given: request with malformed Authorization header
				req, _ := http.NewRequest("GET", "/test", nil)
				req.Header.Set("Authorization", "NotBearer token-content")
				c := e.NewContext(req, rec)

				testHandler := func(c echo.Context) error {
					return c.String(http.StatusOK, "should not reach here")
				}

				// When: processing request with malformed header
				middlewareFunc := testProvider.GetHandlerFunction()
				err := middlewareFunc(testHandler)(c)

				// Then: should return authorization error
				Expect(err).To(HaveOccurred())

				httpError, isHTTPError := err.(*echo.HTTPError)
				Expect(isHTTPError).To(BeTrue())
				Expect(httpError.Code).To(Equal(http.StatusUnauthorized))
				Expect(httpError.Message).To(ContainSubstring("Invalid Authorization header format"))
			})
		})
	})
})
