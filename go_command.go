package libcmd

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"strconv"
	"strings"
)

type goCommandFunc func(args ...string) ([]string, error)

var (
	randCharset = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ_-0123456789")

	goCommands map[string]goCommandFunc = map[string]goCommandFunc{
		"cert":            certCommand,
		"random":          randomCommand,
		"echo":            echoCommand,
		"publicip":        publicIpCommand,
		"github_app_auth": githubAppAuthCommand,
		"resolve_host":    resolveHostCommand,
	}
)

func certCommand(args ...string) ([]string, error) {
	cmd, err := newContainerCmd("cert")
	if err != nil {
		return nil, err
	}
	result, err := cmd.Run(args...)
	if err != nil {
		return nil, err
	}
	results := strings.SplitAfter(result, "-----END RSA PRIVATE KEY-----")
	for i, result := range results {
		result := strings.TrimSpace(result)
		results[i] = base64.StdEncoding.EncodeToString([]byte(result))
	}
	return results, nil
}

func randomCommand(args ...string) ([]string, error) {
	length := 16
	if len(args) > 0 {
		var err error
		length, err = strconv.Atoi(args[0])
		if err != nil {
			return nil, err
		}
	}
	str := randSeq(length)
	return []string{str}, nil
}

func randSeq(length int) string {
	b := make([]rune, length)
	for i := range b {
		b[i] = randCharset[rand.Intn(len(randCharset))]
	}
	return string(b)
}

func echoCommand(args ...string) ([]string, error) {
	result := strings.Join(args, " ")
	return []string{result}, nil
}

func publicIpCommand(args ...string) ([]string, error) {
	urls := []string{
		"http://ipecho.net/plain",
		"http://ip.appspot.com",
		"http://whatismyip.akamai.com",
	}

	done := make(chan string, len(urls))
	errs := make(chan bool, len(urls))
	for _, url := range urls {
		go func(url string) {
			resp, err := http.Get(url)
			if err != nil {
				errs <- true
				return
			}

			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				errs <- true
				return
			}

			ipStr := strings.TrimSpace(string(body))
			if ip := net.ParseIP(ipStr); ip != nil {
				done <- ipStr
			} else {
				errs <- true
			}
		}(url)
	}

	errCount := 0
	for {
		select {
		case result := <-done:
			return []string{result}, nil
		case <-errs:
			errCount = errCount + 1
			if errCount == len(urls) {
				return nil, errors.New("Error contacting publicip servers")
			}
		}
	}
}

func githubAppAuthCommand(args ...string) ([]string, error) {
	// Should be:
	// 0: github_type: "github_type_public" or "github_type_enterprise"
	// 1: github_enterprise_host: "github.replicated.com"
	// 2: github_enterprise_protocol: "github_enterprise_protocol_http" or "github_enterprise_protocol_https"
	// 3: github_client_id
	// 4: github_client_secret
	if len(args) < 5 {
		return nil, fmt.Errorf("Missing required args")
	}
	githubType := args[0]
	githubEnterpriseHost := args[1]
	githubEnterpriseProtocol := args[2]
	githubClientId := args[3]
	githubClientSecret := args[4]

	var protocol, endpoint string
	switch githubType {
	case "github_type_public":
		protocol = "https"
		endpoint = "api.github.com"
	case "github_type_enterprise":
		protocol = strings.Split(githubEnterpriseProtocol, "_")[3]
		cleanedHost := strings.Split(githubEnterpriseHost, "/")[0]
		endpoint = fmt.Sprintf("%s/api/v3", cleanedHost)
	default:
		return nil, fmt.Errorf("Unknown github type: %s", githubType)
	}

	testUrl := fmt.Sprintf("%s://%s/applications/%s/tokens/notatoken", protocol, endpoint, githubClientId)
	req, err := http.NewRequest("GET", testUrl, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(githubClientId, githubClientSecret)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		// Yes, 404 means it's working.
		return []string{"true", "Access granted."}, nil
	}

	return []string{"false", "Access denied."}, nil
}

func resolveHostCommand(args ...string) ([]string, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("Missing a hostname to resolve")
	}
	hostname := args[0]

	a, err := net.LookupHost(hostname)
	if err != nil {
		if strings.HasSuffix(err.Error(), "no such host") {
			return []string{"false", "Hostname could not be resolved."}, nil
		}

		return nil, err
	}

	if len(a) > 0 {
		return []string{"true", "Hostname was resolved successfully."}, nil
	}

	return []string{"false", "Hostname could not be resolved."}, nil
}
