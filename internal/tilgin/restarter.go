package tilgin

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	"golang.org/x/net/publicsuffix"
)

var (
	hmacRegex    = regexp.MustCompile(`__pass\.value,\s+"(\w+?)"`)
	formTokRegex = regexp.MustCompile(`<input type=hidden name="__formtok" value="(\w+?)">`)
)

type Restarter struct {
	logger     *zap.SugaredLogger
	username   string
	password   string
	routerHost string

	httpClient *http.Client
	hmacSecret []byte
	formToken  string
}

func NewRestarter(logger *zap.SugaredLogger, username, password, routerHost string) *Restarter {
	return &Restarter{
		logger:     logger,
		username:   username,
		password:   password,
		routerHost: routerHost,
	}
}

func (r *Restarter) Restart() error {
	r.logger.Info("Starting restart, fetching secrets")

	jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	if err != nil {
		r.logger.Errorw("Failed to initialise cookie jar",
			"error", err)
		return errors.Wrap(err, "failed to initialise cookie jar")
	}
	r.httpClient = &http.Client{
		Jar: jar,
	}

	err = r.fetchHMACSecret()
	if err != nil {
		r.logger.Errorw("Failed to fetch hmac secret",
			"error", err)
		return errors.Wrap(err, "failed to fetch hmac secret")
	}

	err = r.login()
	if err != nil {
		r.logger.Errorw("Failed to login",
			"error", err)
		return errors.Wrap(err, "failed to login")
	}

	err = r.restart()
	if err != nil {
		r.logger.Errorw("Failed to restart",
			"error", err)
		return errors.Wrap(err, "failed to restart")
	}

	err = r.checkHost()
	if err != nil {
		r.logger.Errorw("Failed to check if router came back up",
			"error", err)
		return errors.Wrap(err, "failed to check if router came back up")
	}

	return nil
}

func (r *Restarter) fetchHMACSecret() error {
	r.logger.Info("Fetching HMAC secret")
	indexResp, err := r.httpClient.Get(r.routerHost)
	if err != nil {
		r.logger.Errorw("Failed to get index page",
			"error", err)
		return errors.Wrap(err, "failed to get index page")
	}
	body, err := ioutil.ReadAll(indexResp.Body)
	if err != nil {
		r.logger.Errorw("Failed to read body",
			"error", err)
		return errors.Wrap(err, "failed to read body")
	}
	submatches := hmacRegex.FindStringSubmatch(string(body))
	if len(submatches) != 2 {
		r.logger.Error("Failed to extract hmac secret")
		return errors.New("failed to extract hmac secret")
	}
	r.logger.Debug("HMAC secret:", submatches[1])
	r.hmacSecret = []byte(submatches[1])
	return nil
}

func (r *Restarter) login() error {
	r.logger.Info("Authenticating")

	loginData := url.Values{
		"__formtok": []string{""},
		"__auth":    []string{"login"},
		"__user":    []string{r.username},
		"__hash":    []string{r.passwordHash(r.hmacSecret)},
	}
	loginRequest, err := http.NewRequest(http.MethodPost, r.routerHost, strings.NewReader(loginData.Encode()))
	if err != nil {
		r.logger.Errorw("Failed to create login request",
			"error", err)
		return errors.Wrap(err, "failed to create login request")
	}
	loginRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	loginResponse, err := r.httpClient.Do(loginRequest)
	if err != nil {
		r.logger.Errorw("Failed to auth",
			"error", err)
		return errors.Wrap(err, "failed to auth")
	}

	loginBody, err := ioutil.ReadAll(loginResponse.Body)
	if err != nil {
		r.logger.Errorw("Failed to read auth body",
			"error", err)
		return errors.Wrap(err, "failed to read auth body")
	}

	if loginResponse.StatusCode < 200 || loginResponse.StatusCode > 299 {
		r.logger.Error("Auth failed:", string(loginBody))
		return errors.New("failed to auth")
	}

	submatches := formTokRegex.FindStringSubmatch(string(loginBody))
	if len(submatches) < 2 {
		r.logger.Error("Failed to extract CSRF form token")
		return errors.New("failed to extract CSRF form token")
	}
	r.logger.Debug("CSRF form token:", submatches[1])
	r.formToken = submatches[1]

	return nil
}

func (r *Restarter) restart() error {
	r.logger.Info("Restarting")

	restartData := url.Values{
		"__formtok": []string{r.formToken},
		"__form":    []string{"restart"},
	}
	restartRequest, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/tools/restart", r.routerHost), strings.NewReader(restartData.Encode()))
	if err != nil {
		r.logger.Errorw("Failed to create restart request",
			"error", err)
		return errors.Wrap(err, "failed to create restart request")
	}
	restartRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	restartResponse, err := r.httpClient.Do(restartRequest)
	if err != nil {
		r.logger.Errorw("Failed to restart",
			"error", err)
		return errors.Wrap(err, "failed to restart")
	}
	if restartResponse.StatusCode < 200 || restartResponse.StatusCode > 299 {
		body, err := ioutil.ReadAll(restartResponse.Body)
		if err != nil {
			r.logger.Errorw("Failed to read restart failure",
				"error", err)
			return errors.Wrap(err, "failed to read restart failure")
		}
		r.logger.Error("Restart failed:", string(body))
		return errors.New("failed to restart")
	}

	return nil
}

func (r *Restarter) checkHost() error {
	timeout := time.After(5 * time.Minute)
	client := &http.Client{
		Timeout: 5 * time.Second,
	}
	tick := time.Tick(time.Second)
	lastSeenDown := false
	firstSeenDownTime := time.Time{}
	for {
		select {
		case <-timeout:
			return errors.New("timed out waiting for host to come back up")
		case <-tick:
			_, err := client.Get(r.routerHost)
			if err != nil {
				if !lastSeenDown {
					firstSeenDownTime = time.Now()
				}
				lastSeenDown = true
				r.logger.Info("Failed to reach router host: ", err)
				continue
			}
			if lastSeenDown {
				r.logger.Infof("Router has come back up after %s", time.Since(firstSeenDownTime))
				return nil
			}
			r.logger.Info("Router is reachable")
		}
	}
}

func (r *Restarter) passwordHash(hmacSecret []byte) string {
	mac := hmac.New(sha1.New, hmacSecret)
	mac.Write([]byte(r.username + r.password))
	expectedMAC := mac.Sum(nil)
	hexString := hex.EncodeToString(expectedMAC)
	return hexString
}
