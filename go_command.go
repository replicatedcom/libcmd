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
	// 0: access type: http or https
	// 1: endpoint, e.g.: api.github.com, or github.replicated.com/api/v3
	// 2: key
	// 3: secret
	if len(args) < 4 {
		return nil, fmt.Errorf("Missing required args")
	}

	endpoint := strings.TrimSuffix(args[1], "/")
	testUrl := fmt.Sprintf("%v://%v/applications/%v/tokens/notatoken", args[0], endpoint, args[2])
	req, err := http.NewRequest("GET", testUrl, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(args[2], args[3])
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		// Yes, 404 means it's working.
		return []string{"access_granted", "404"}, nil
	}

	return []string{"access_denied", fmt.Sprintf("%v", resp.StatusCode)}, nil
}
