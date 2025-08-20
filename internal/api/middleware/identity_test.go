package middleware_test

import (
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"

	"github.com/labstack/echo/v4"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	authenticationv1 "k8s.io/api/authentication/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubernetesfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/testing"

	"github.com/redhatinsights/ros-ocp-backend/internal/api/middleware"
)

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

	Context("OAuth Identity Provider", func() {
		Describe("Bearer Token Authentication", func() {
			Context("when token authentication succeeds", func() {
				var (
					userInfo = authenticationv1.UserInfo{
						Username: "testuser",
						UID:      "12345",
						Groups:   []string{"group1"},
					}
				)
				BeforeEach(func() {
					// Set up reactor to simulate successful authentication
					fakeClient.PrependReactor("create", "tokenreviews", func(action testing.Action) (bool, runtime.Object, error) {
						tr := action.(testing.CreateAction).GetObject().(*authenticationv1.TokenReview)
						// Simulate successful authentication
						tr.Status.Authenticated = true
						tr.Status.User = userInfo
						return true, tr, nil
					})

					// OAuth is now just a string, so directly encode the token
					token := "valid-test-token"
					encodedOAuth := base64.StdEncoding.EncodeToString([]byte(token))

					req = httptest.NewRequest(http.MethodGet, "/test", nil)
					req.Header.Set("Bearer Token", encodedOAuth)
				})

				It("should authenticate successfully and call next handler", func() {
					handlerCalled := false
					testHandler := func(c echo.Context) error {
						handlerCalled = true
						// Verify identity is set in context
						identity := c.Get("Identity")
						Expect(identity).To(Equal(userInfo))
						return c.String(http.StatusOK, "success")
					}

					c := e.NewContext(req, rec)
					middlewareFunc := provider.GetHandlerFunction()
					err := middlewareFunc(testHandler)(c)

					Expect(err).NotTo(HaveOccurred())
					Expect(handlerCalled).To(BeTrue())
				})
			})

			Context("when token authentication fails", func() {
				BeforeEach(func() {
					// Set up reactor to simulate failed authentication
					fakeClient.PrependReactor("create", "tokenreviews", func(action testing.Action) (bool, runtime.Object, error) {
						tr := action.(testing.CreateAction).GetObject().(*authenticationv1.TokenReview)
						// Simulate failed authentication
						tr.Status.Authenticated = false
						tr.Status.Error = "token is not valid"
						return true, tr, nil
					})

					token := "invalid-test-token"
					encodedOAuth := base64.StdEncoding.EncodeToString([]byte(token))

					req = httptest.NewRequest(http.MethodGet, "/test", nil)
					req.Header.Set("Bearer Token", encodedOAuth)
				})

				It("should return 401 Unauthorized from TokenReview validation", func() {
					testHandler := func(c echo.Context) error {
						Fail("Should not reach handler due to invalid token from TokenReview")
						return nil
					}

					c := e.NewContext(req, rec)
					middlewareFunc := provider.GetHandlerFunction()
					err := middlewareFunc(testHandler)(c)

					Expect(err).To(HaveOccurred())
					httpError, ok := err.(*echo.HTTPError)
					Expect(ok).To(BeTrue())
					Expect(httpError.Code).To(Equal(http.StatusUnauthorized))
					Expect(httpError.Message).To(ContainSubstring("Invalid or expired token"))
				})
			})

			Context("when TokenReview API returns an error", func() {
				BeforeEach(func() {
					// Set up reactor to simulate TokenReview API error
					fakeClient.PrependReactor("create", "tokenreviews", func(action testing.Action) (bool, runtime.Object, error) {
						return true, nil, errors.New("Internal server error")
					})

					token := "test-token-api-error"
					encodedOAuth := base64.StdEncoding.EncodeToString([]byte(token))

					req = httptest.NewRequest(http.MethodGet, "/test", nil)
					req.Header.Set("Bearer Token", encodedOAuth)
				})

				It("should return 500 Internal Server Error", func() {
					testHandler := func(c echo.Context) error {
						Fail("Should not reach handler due to TokenReview API error")
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

			Context("when testing malformed requests", func() {
				It("should return 401 for missing Bearer Token header", func() {
					req = httptest.NewRequest(http.MethodGet, "/test", nil)
					// No Bearer Token header set

					testHandler := func(c echo.Context) error {
						return c.String(http.StatusOK, "success")
					}

					c := e.NewContext(req, rec)
					middlewareFunc := provider.GetHandlerFunction()
					err := middlewareFunc(testHandler)(c)

					Expect(err).To(HaveOccurred())
					httpError, ok := err.(*echo.HTTPError)
					Expect(ok).To(BeTrue())
					Expect(httpError.Code).To(Equal(http.StatusUnauthorized))
					Expect(httpError.Message).To(ContainSubstring("Missing Bearer Token"))
				})

				It("should return 401 for malformed base64 token", func() {
					req = httptest.NewRequest(http.MethodGet, "/test", nil)
					req.Header.Set("Bearer Token", "invalid-base64-!@#")

					testHandler := func(c echo.Context) error {
						return c.String(http.StatusOK, "success")
					}

					c := e.NewContext(req, rec)
					middlewareFunc := provider.GetHandlerFunction()
					err := middlewareFunc(testHandler)(c)

					Expect(err).To(HaveOccurred())
					httpError, ok := err.(*echo.HTTPError)
					Expect(ok).To(BeTrue())
					Expect(httpError.Code).To(Equal(http.StatusUnauthorized))
					Expect(httpError.Message).To(ContainSubstring("Unable to decode"))
				})
			})

			Context("when testing with empty token after decode", func() {
				BeforeEach(func() {
					// Empty string encoded in base64
					encodedOAuth := base64.StdEncoding.EncodeToString([]byte(""))

					req = httptest.NewRequest(http.MethodGet, "/test", nil)
					req.Header.Set("Bearer Token", encodedOAuth)
				})

				It("should return 401 for empty token", func() {
					testHandler := func(c echo.Context) error {
						Fail("Should not reach handler due to empty token")
						return nil
					}

					c := e.NewContext(req, rec)
					middlewareFunc := provider.GetHandlerFunction()
					err := middlewareFunc(testHandler)(c)

					Expect(err).To(HaveOccurred())
					httpError, ok := err.(*echo.HTTPError)
					Expect(ok).To(BeTrue())
					Expect(httpError.Code).To(Equal(http.StatusUnauthorized))
					Expect(httpError.Message).To(ContainSubstring("Missing Bearer Token"))
				})
			})
		})
	})

	Context("Identity Provider Factory", func() {
		Describe("GetIdentityProviderHandlerFunction", func() {
			It("should return RHSSO provider for rhsso config", func() {
				handler, err := middleware.GetIdentityProviderHandlerFunction("rhsso")
				Expect(err).ToNot(HaveOccurred())
				Expect(handler).ToNot(BeNil())
			})

			It("should return RHSSO provider for default/unknown config", func() {
				handler, err := middleware.GetIdentityProviderHandlerFunction("unknown")
				Expect(err).ToNot(HaveOccurred())
				Expect(handler).ToNot(BeNil())
			})

			It("should return RHSSO provider for empty config", func() {
				handler, err := middleware.GetIdentityProviderHandlerFunction("")
				Expect(err).ToNot(HaveOccurred())
				Expect(handler).ToNot(BeNil())
			})
		})
	})

	Context("RHSSO Identity Provider", func() {
		var rhssoProvider middleware.IdentityProvider

		BeforeEach(func() {
			rhssoProvider = middleware.NewRHSSOIdentityProvider()
		})

		Describe("X-Rh-Identity header processing", func() {
			It("should process valid X-Rh-Identity header", func() {
				// This is a basic test - the actual RHSSO functionality would need more complex testing
				handler := rhssoProvider.GetHandlerFunction()
				Expect(handler).ToNot(BeNil())
			})
		})
	})
})
