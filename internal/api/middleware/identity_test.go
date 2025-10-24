package middleware_test

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"github.com/labstack/echo/v4"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

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
   - Given RHSSO configuration → Then provides appropriate middleware

2. When authenticating with Red Hat SSO
   - Given X-Rh-Identity headers → Then extracts user identity for RBAC

Each behavior includes comprehensive scenarios to ensure production readiness.
===============================================================================
*/

var _ = Describe("Identity Middleware", func() {
	var (
		e   *echo.Echo
		rec *httptest.ResponseRecorder
	)

	BeforeEach(func() {
		e = echo.New()
		rec = httptest.NewRecorder()
	})

	// =============================================================================
	// IDENTITY PROVIDER FACTORY BEHAVIOR
	// Tests how the system selects and configures identity providers
	// =============================================================================

	Describe("When requesting the identity provider", func() {
		Context("Given RHSSO identity provider", func() {
			It("should provide RHSSO middleware", func() {
				// Given: RHSSO is the only identity provider

				// When: requesting identity provider middleware
				handler, err := middleware.GetIdentityProviderHandlerFunction()

				// Then: should return valid RHSSO middleware
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
})
