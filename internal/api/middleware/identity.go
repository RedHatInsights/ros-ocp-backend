package middleware

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"

	"github.com/labstack/echo/v4"
	"github.com/redhatinsights/platform-go-middlewares/identity"
	authv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	// ID Provider config values
	rhSSOIDProvider = "rhsso"
	oAuthIDProvider = "oauth"

	// ID Provider header values
	RHSSOIdentityHeader = "X-Rh-Identity"
	OAuthIdentityHeader = "Bearer Token"
)

type IdentityProvider interface {
	GetHandlerFunction() echo.MiddlewareFunc
}

type OAuthIdentityProvider struct {
	client kubernetes.Interface
}

func NewOauthIDProvider(kubeClient kubernetes.Interface) IdentityProvider {
	return &OAuthIdentityProvider{client: kubeClient}
}

func (o *OAuthIdentityProvider) GetHandlerFunction() echo.MiddlewareFunc {
	return o.oauthIdentityHandlerFunction
}

func (o *OAuthIdentityProvider) oauthIdentityHandlerFunction(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		decodedIdentity, err := decodeIdentity(c, OAuthIdentityHeader)
		if err != nil {
			return err
		}
		token := string(decodedIdentity)
		if token == "" {
			return echo.NewHTTPError(http.StatusUnauthorized, fmt.Sprintf("Missing %s", OAuthIdentityHeader))
		}

		userInfo, err := o.validateToken(string(token))
		if err != nil {
			return err
		}
		if userInfo == nil {
			return echo.NewHTTPError(http.StatusUnauthorized, "User information is missing from TokenReview API")
		}

		c.Set("Identity", identity.OAuthID(*userInfo))
		return next(c)
	}
}

func (o *OAuthIdentityProvider) validateToken(token string) (*authv1.UserInfo, error) {
	// Validate the bearer token using Kubernetes TokenReview API
	tokenReview := &authv1.TokenReview{
		Spec: authv1.TokenReviewSpec{
			Token: token,
		},
	}

	result, err := o.client.AuthenticationV1().TokenReviews().Create(
		context.TODO(),
		tokenReview,
		metav1.CreateOptions{},
	)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to validate token: %v", err))
	}

	if !result.Status.Authenticated {
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "Invalid or expired token")
	}
	return &result.Status.User, nil
}

func GetIdentityProviderHandlerFunction(idProvider string) (echo.MiddlewareFunc, error) {
	var hf IdentityProvider
	var err error

	switch idProvider {
	case oAuthIDProvider:
		hf, err = newOauthIdentityProvider()
		if err != nil {
			return nil, err
		}
	case rhSSOIDProvider:
		fallthrough
	default:
		hf = NewRHSSOIdentityProvider()
	}
	return hf.GetHandlerFunction(), nil
}

func newOauthIdentityProvider() (IdentityProvider, error) {
	var kubeClient *kubernetes.Clientset
	var err error
	if os.Getenv("KUBECONFIG") != "" {
		cfg, err := clientcmd.BuildConfigFromFlags("", os.Getenv("KUBECONFIG"))
		if err != nil {
			return nil, err
		}
		kubeClient, err = kubernetes.NewForConfig(cfg)
		if err != nil {
			return nil, err
		}
	} else {
		kubeClient, err = createInClusterClient()
		if err != nil {
			return nil, err
		}
	}
	return NewOauthIDProvider(kubeClient), nil
}

type RHSSOIdentityProvider struct {
}

func NewRHSSOIdentityProvider() IdentityProvider {
	return &RHSSOIdentityProvider{}
}

func (r *RHSSOIdentityProvider) GetHandlerFunction() echo.MiddlewareFunc {
	return r.rhSSOIdentityHandlerFunction
}

func (r *RHSSOIdentityProvider) rhSSOIdentityHandlerFunction(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		decodedIdentity, err := decodeIdentity(c, RHSSOIdentityHeader)
		if err != nil {
			return err
		}
		id, err := identity.NewXRHIDFromHeader(decodedIdentity)
		if err != nil {
			return echo.NewHTTPError(http.StatusUnauthorized, fmt.Sprintf("Unable to marshal %s into struct", RHSSOIdentityHeader))
		}
		c.Set("Identity", id)
		return next(c)
	}
}

func decodeIdentity(c echo.Context, header string) ([]byte, error) {
	encodedIdentity := c.Request().Header.Get(header)
	decodedIdentity, err := base64.StdEncoding.DecodeString(encodedIdentity)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusUnauthorized, fmt.Sprintf("Unable to decode %s", header))
	}
	return decodedIdentity, nil
}

func createInClusterClient() (*kubernetes.Clientset, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("unable to load in-cluster config: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("unable to create Kubernetes client: %v", err)
	}

	return clientset, nil
}
