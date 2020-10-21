package tilgin

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	"golang.org/x/net/publicsuffix"
)

var (
	hmacRegex  = regexp.MustCompile(`__pass\.value,\s+"(\w+?)"`)
	spaceRegex = regexp.MustCompile(`\s+`)
)

type Restarter struct {
	logger     *zap.SugaredLogger
	username   string
	password   string
	routerHost string
}

func NewRestarter(logger *zap.SugaredLogger, username, password, routerHost string) *Restarter {
	return &Restarter{
		logger:     logger,
		username:   username,
		password:   password,
		routerHost: routerHost,
	}
}

func (s *Restarter) Restart() error {
	s.logger.Info("Starting restart, fetching secrets")

	hmacSecret, err := s.fetchHMACSecret()
	if err != nil {
		s.logger.Errorw("Failed to fetch hmac secret",
			"error", err)
		return errors.Wrap(err, "failed to fetch hmac secret")
	}

	// Auth
	s.logger.Info("Authenticating")

	jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	if err != nil {
		s.logger.Errorw("Failed to initialise cookie jar",
			"error", err)
		return errors.Wrap(err, "failed to initialise cookie jar")
	}
	client := &http.Client{
		Jar: jar,
	}

	loginData := url.Values{
		"__formtok": []string{""},
		"__auth":    []string{"login"},
		"__user":    []string{s.username},
		"__hash":    []string{s.passwordHash(hmacSecret)},
	}
	loginRequest, err := http.NewRequest(http.MethodPost, s.routerHost, strings.NewReader(loginData.Encode()))
	if err != nil {
		s.logger.Errorw("Failed to create login request",
			"error", err)
		return errors.Wrap(err, "failed to create login request")
	}

	_, err = client.Do(loginRequest)
	if err != nil {
		s.logger.Errorw("Failed to auth",
			"error", err)
		return errors.Wrap(err, "failed to auth")
	}

	s.logger.Info("Restarting")

	restartData := url.Values{
		"__formtok": []string{""},
		"__form":    []string{"restart"},
	}
	restartRequest, err := http.NewRequest(http.MethodPost, s.routerHost, strings.NewReader(restartData.Encode()))
	if err != nil {
		s.logger.Errorw("Failed to create restart request",
			"error", err)
		return errors.Wrap(err, "failed to create restart request")
	}

	_, err = client.Do(restartRequest)
	if err != nil {
		s.logger.Errorw("Failed to restart",
			"error", err)
		return errors.Wrap(err, "failed to restart")
	}

	return nil
}

func (s *Restarter) fetchHMACSecret() ([]byte, error) {
	s.logger.Info("Fetching HMAC secret")
	indexResp, err := http.Get(s.routerHost)
	if err != nil {
		s.logger.Errorw("Failed to get index page",
			"error", err)
		return nil, errors.Wrap(err, "failed to get index page")
	}
	body, err := ioutil.ReadAll(indexResp.Body)
	if err != nil {
		s.logger.Errorw("Failed to read body",
			"error", err)
		return nil, errors.Wrap(err, "failed to read body")
	}
	submatches := hmacRegex.FindStringSubmatch(string(body))
	if len(submatches) != 2 {
		s.logger.Error("Failed to extract hmac secret")
		return nil, errors.New("failed to extract hmac secret")
	}
	return []byte(submatches[1]), nil
}

func (s *Restarter) passwordHash(hmacSecret []byte) string {
	mac := hmac.New(sha1.New, hmacSecret)
	mac.Write([]byte(s.username + s.password))
	expectedMAC := mac.Sum(nil)
	hexString := hex.EncodeToString(expectedMAC)
	return hexString
}

func trimAndCleanString(s string) string {
	return strings.TrimSpace(spaceRegex.ReplaceAllString(s, " "))
}
