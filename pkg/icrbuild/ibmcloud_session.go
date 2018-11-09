// ------------------------------------------------------------------------------
// Copyright IBM Corp. 2018
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
// ------------------------------------------------------------------------------

// Package icrbuild ...
package icrbuild

import (
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"net"
	"net/http"
	"time"

	"fmt"
	"io/ioutil"
	"net/url"
	"os"

	"github.com/sirupsen/logrus"

	ibmcloud "github.com/IBM-Cloud/bluemix-go"
	"github.com/IBM-Cloud/bluemix-go/api/container/registryv1"
	"github.com/IBM-Cloud/bluemix-go/api/iam/iamv1"
	"github.com/IBM-Cloud/bluemix-go/endpoints"
	"github.com/IBM-Cloud/bluemix-go/session"
	"github.com/pkg/errors"

	"strings"
)

const na = "n/a"

// IBMRegistrySession structure
type IBMRegistrySession struct {
	Registry          string
	Builds            registryv1.Builds
	BuildTargetHeader registryv1.BuildTargetHeader
}

type configJSON struct {
	Region          string `json:"Region"`
	IAMToken        string `json:"IAMToken"`
	IAMRefreshToken string `json:"IAMRefreshToken"`
	Account         struct {
		GUID string `json:"GUID"`
	} `json:"Account"`
	SSLDisabled bool `json:"SSLDisabled"`
}

// Same structs used by knative credentials
type entry struct {
	Secret   string `json:"-"`
	Username string `json:"username"`
	Password string `json:"password"`
	Auth     string `json:"auth"`
	Email    string `json:"email"`
}

type dockerConfig struct {
	Entries map[string]entry `json:"auths"`
}

// NewHTTPClient ...
func NewHTTPClient(config *ibmcloud.Config) *http.Client {
	return &http.Client{
		Transport: makeTransport(config),
		Timeout:   config.HTTPTimeout,
	}
}

func makeTransport(config *ibmcloud.Config) http.RoundTripper {
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		Dial: (&net.Dialer{
			Timeout:   50 * time.Second,
			KeepAlive: 30 * time.Second,
		}).Dial,
		TLSHandshakeTimeout: 20 * time.Second,
		DisableCompression:  true,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: config.SSLDisable,
		},
	}
}

// NewRegistryClient Authenticates with IBM Cloud using provided API Key
// Fixes the image name if the registry name isn't part of it
func NewRegistryClient(imageName string) (*IBMRegistrySession, string, error) {
	var (
		c = &ibmcloud.Config{
			Region:        "us-south",
			BluemixAPIKey: "",
		}

		authSession       *session.Session
		endpointcp        *string
		account, endpoint string
		iamAPI            iamv1.IAMServiceAPI
		registryAPI       registryv1.RegistryServiceAPI
		userInfo          *iamv1.UserInfo
		err               error
		url               url.URL
	)

	c.HTTPClient = NewHTTPClient(c)
	endpointcp = getRegistryEndpoint(imageName)

	if endpointcp == nil {
		var registry string

		registry, err = endpoints.NewEndpointLocator(c.Region).ContainerRegistryEndpoint()
		if err != nil {
			return nil, imageName, errors.Wrap(err, "Unsupported IBM Cloud default region")
		}
		endpoint = registry
		imageName, err = addRegistry(registry, imageName)
		if err != nil {
			return nil, imageName, err
		}
		url, err := url.Parse(endpoint)
		if err != nil {
			return nil, imageName, errors.Wrap(err, "Error Parsing registry endpoint")
		}
		endpointcp = &url.Host
	} else {
		endpoint = fmt.Sprintf("https://%s", *endpointcp)
	}

	_, err = configFromDocker(c, *endpointcp)
	if err != nil {
		logrus.Errorf("Error Fetching Docker Config: ", err)
	}
	if c.BluemixAPIKey == "" {
		logrus.Warnf("Bluemix not set, trying to use a pre-authenticated CLI Session...")
		account, err = configFromJSON(c)
		if err != nil {
			return nil, imageName, errors.Wrap(err, "IBM Cloud configuration error.")
		}
	}
	authSession, err = session.New(c)
	if err != nil {
		return nil, imageName, errors.Wrap(err, "IBM Cloud configuration error.")
	}
	iamAPI, err = iamv1.New(authSession)
	if err != nil {
		return nil, imageName, errors.Wrap(err, "IBM Cloud auth error.")
	}

	if account == "" {
		userInfo, err = iamAPI.Identity().UserInfo()
		if err != nil {
			return nil, imageName, errors.Wrap(err, "IBM Cloud fetching user account error.")
		}
		account = userInfo.Account.Bss
	}

	c.Endpoint = &endpoint
	registryAPI, err = registryv1.New(authSession)
	if err != nil {
		return nil, imageName, errors.Wrap(err, "IBM Cloud auth error.")
	}

	return &IBMRegistrySession{
		BuildTargetHeader: registryv1.BuildTargetHeader{
			AccountID: account,
		},
		Builds: registryAPI.Builds(),
	}, imageName, nil
}

func getRegistryEndpoint(imageName string) *string {
	var segments []string
	var endpoint string

	segments = strings.Split(imageName, "/")
	if len(segments) > 0 && len(imageName) > 0 {
		if !strings.Contains(segments[0], ".") {
			return nil
		}
	}
	endpoint = segments[0]
	return &endpoint
}

// addRegistry for adding the default registry if no registry is in image
func addRegistry(endpoint string, imageName string) (string, error) {
	var (
		registryURL *url.URL
		segments    []string
		err         error
	)

	registryURL, err = url.Parse(endpoint)
	if err != nil {
		return "", errors.Wrap(err, "Bad registry URL for IBM Cloud default region")
	}
	segments = strings.Split(imageName, "/")
	if len(segments) > 0 && len(imageName) > 0 {
		if strings.Contains(segments[0], ".") {
			return imageName, nil
		}
		tempName := imageName
		if strings.HasPrefix(imageName, "/") {
			tempName = imageName[1:]
		}
		return fmt.Sprintf("%s/%s", registryURL.Hostname(), tempName), nil
	}
	return imageName, nil
}

// If the authenticated with IBM Cloud using CLI
func configFromJSON(icconfig *ibmcloud.Config) (accountID string, err error) {
	var (
		config    *configJSON
		jsonFile  *os.File
		byteValue []byte
	)

	config = new(configJSON)

	jsonFile, err = os.Open(os.Getenv("HOME") + "/.bluemix/config.json")
	defer func() {
		cerr := jsonFile.Close()
		if err == nil {
			err = cerr
		}
	}()
	if err == nil {
		byteValue, err = ioutil.ReadAll(jsonFile)
		if err == nil {
			err = json.Unmarshal(byteValue, config)
			if err == nil {
				icconfig.Region = config.Region
				icconfig.IAMAccessToken = config.IAMToken
				icconfig.IAMRefreshToken = config.IAMRefreshToken
				icconfig.SSLDisable = config.SSLDisabled
				icconfig.BluemixAPIKey = na
				icconfig.IBMID = na
				icconfig.IBMIDPassword = na
				accountID = config.Account.GUID
			}
		}
	}
	return accountID, err
}

// If the authenticated with hav
func configFromDocker(icconfig *ibmcloud.Config, endpoint string) (accountID string, err error) {
	var (
		config             *dockerConfig
		jsonFile           *os.File
		byteValue, decoded []byte
		authToken          []string
	)

	config = new(dockerConfig)

	jsonFile, err = os.Open(os.Getenv("HOME") + "/.docker/config.json")
	defer func() {
		cerr := jsonFile.Close()
		if err == nil {
			err = cerr
		}
	}()
	if err == nil {
		byteValue, err = ioutil.ReadAll(jsonFile)
		if err == nil {
			err = json.Unmarshal(byteValue, config)
			if err == nil {
				if entry, ok := config.Entries[endpoint]; ok {
					//Only API keys stored as passwords are supported
					if entry.Password != "" {
						icconfig.BluemixAPIKey = entry.Password
					} else if entry.Auth != "" {
						decoded, err = base64.StdEncoding.DecodeString(entry.Auth)
						if err == nil {
							authToken = strings.Split(string(decoded), ":")
							if len(authToken) > 1 {
								icconfig.BluemixAPIKey = authToken[1]
							}
						}
					} else {
						err = errors.Errorf("Found docker config but unable to find API Key!", endpoint)
					}
				} else {
					err = errors.Errorf("Registry %s not found in docker creds!", endpoint)
				}
			}
		}
	}
	return "", err
}
