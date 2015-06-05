package command

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/awslabs/aws-sdk-go/aws"
	"github.com/awslabs/aws-sdk-go/aws/credentials"
	"github.com/awslabs/aws-sdk-go/service/ec2"
	"github.com/awslabs/aws-sdk-go/service/s3"
	"github.com/awslabs/aws-sdk-go/service/sqs"
	"github.com/fsouza/go-dockerclient"
)

type goCommandFunc func(c *GoCmd, args ...string) ([]string, error)

const (
	AWSServiceEC2 = "ec2"
	AWSServiceS3  = "s3"
	AWSServiceSQS = "sqs"
)

var (
	randCharset = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ_-0123456789")

	goCommands = map[string]goCommandFunc{
		"cert":             certCommand,
		"random":           randomCommand,
		"echo":             echoCommand,
		"publicip":         publicIPCommand,
		"github_app_auth":  githubAppAuthCommand,
		"aws_auth":         awsAuthCommand,
		"resolve_host":     resolveHostCommand,
		"tcp_port_accept":  tcpPortAccept,
		"http_status_code": httpStatusCode,
	}
)

type GoCmd struct {
	Fn           goCommandFunc
	config       CmdConfig
	dockerClient *docker.Client
}

func (c *GoCmd) Run(args ...string) ([]string, error) {
	return c.Fn(c, args...)
}

func NewGoCmd(op string, config CmdConfig, dockerClient *docker.Client) (*GoCmd, error) {
	fn, exists := goCommands[op]
	if !exists {
		return nil, ErrCommandNotFound
	}
	return &GoCmd{fn, config, dockerClient}, nil
}

func certCommand(c *GoCmd, args ...string) ([]string, error) {
	cmd, err := NewContainerCmd("cert", c.config, c.dockerClient)
	if err != nil {
		return nil, err
	}
	result, err := cmd.Run(args...)
	if err != nil {
		return result, err
	}
	results := strings.SplitAfter(result[0], "-----END RSA PRIVATE KEY-----")
	for i, result := range results {
		result := strings.TrimSpace(result)
		results[i] = base64.StdEncoding.EncodeToString([]byte(result))
	}
	return results, nil
}

func randomCommand(c *GoCmd, args ...string) ([]string, error) {
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

func echoCommand(c *GoCmd, args ...string) ([]string, error) {
	result := strings.Join(args, " ")
	return []string{result}, nil
}

func publicIPCommand(c *GoCmd, args ...string) ([]string, error) {
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
			errCount++
			if errCount == len(urls) {
				return nil, errors.New("Error contacting publicip servers.")
			}
		}
	}
}

func githubAppAuthCommand(c *GoCmd, args ...string) ([]string, error) {
	// Should be:
	// 0: github_type: "github_type_public" or "github_type_enterprise"
	// 1: github_enterprise_host: "github.replicated.com"
	// 2: github_enterprise_protocol: "github_enterprise_protocol_http" or "github_enterprise_protocol_https"
	// 3: github_client_id
	// 4: github_client_secret
	if len(args) < 5 {
		return nil, ErrMissingArgs
	}
	githubType := args[0]
	githubEnterpriseHost := args[1]
	githubEnterpriseProtocol := args[2]
	githubClientID := args[3]
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

	testURL := fmt.Sprintf("%s://%s/applications/%s/tokens/notatoken", protocol, endpoint, githubClientID)
	req, err := http.NewRequest("GET", testURL, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(githubClientID, githubClientSecret)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		// Yes, 404 means it's working.
		return []string{"true"}, nil
	}

	errMsg := "Github app authentication failed."

	if body, err := ioutil.ReadAll(resp.Body); err == nil {
		var data map[string]interface{}
		if err := json.Unmarshal(body, &data); err == nil {
			if msg, ok := data["message"]; ok {
				errMsg = fmt.Sprintf("Github app authentication failed: %s", msg)
			}
		}
	}

	return []string{"false"}, ErrCommandResponse{errMsg}
}

func awsAuthCommand(c *GoCmd, args ...string) ([]string, error) {
	// Should be:
	// 0: aws_access_key_id
	// 1: aws_secret_access_key
	// 2: aws_service
	if len(args) < 3 {
		return nil, ErrMissingArgs
	}
	awsAccessKeyID := args[0]
	awsSecretAccessKey := args[1]
	awsService := args[2]

	creds := credentials.NewStaticCredentials(awsAccessKeyID, awsSecretAccessKey, "")
	config := &aws.Config{
		Region:      "us-east-1",
		Credentials: creds,
	}

	var err error

	switch awsService {
	case AWSServiceEC2:
		svc := ec2.New(config)
		_, err = svc.DescribeRegions(nil)

	case AWSServiceS3:
		svc := s3.New(config)
		_, err = svc.ListBuckets(nil)

	case AWSServiceSQS:
		svc := sqs.New(config)
		_, err = svc.ListQueues(nil)

	default:
		return nil, errors.New("AWS service must be one of \"ec2\", \"s3\" or \"sqs\"")
	}

	if awserr := aws.Error(err); awserr != nil {
		errMsg := fmt.Sprintf("AWS authentication failed: %v", awserr)
		return []string{"false"}, ErrCommandResponse{errMsg}
	} else if err != nil {
		return nil, err
	}

	return []string{"true"}, nil
}

func resolveHostCommand(c *GoCmd, args ...string) ([]string, error) {
	if len(args) < 1 {
		return nil, ErrMissingArgs
	}

	hostname := args[0]

	addrs, err := net.LookupHost(hostname)
	if err != nil {
		errMsg := fmt.Sprintf("Error contacting host: %v", err)
		return nil, ErrCommandResponse{errMsg}
	}

	if len(addrs) > 0 {
		return addrs, nil
	}

	return nil, ErrCommandResponse{"Error contacting host"}
}

func tcpPortAccept(c *GoCmd, args ...string) ([]string, error) {
	if len(args) < 2 {
		return nil, ErrMissingArgs
	}

	if _, err := net.Dial("tcp", fmt.Sprintf("%s:%s", args[0], args[1])); err != nil {
		return []string{"false"}, ErrCommandResponse{err.Error()}
	}
	return []string{"true"}, nil
}

func httpStatusCode(c *GoCmd, args ...string) ([]string, error) {
	if len(args) < 1 {
		return nil, ErrMissingArgs
	}

	req, err := http.NewRequest("GET", args[0], nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	actualStatus := strconv.Itoa(resp.StatusCode)

	// TODO: i would like to deprecate the expected status version of this command
	if len(args) >= 2 {
		expectedStatus := args[1]
		if _, err := strconv.Atoi(expectedStatus); err != nil {
			return nil, err
		}

		if actualStatus != expectedStatus {
			errMsg := fmt.Sprintf("HTTP status code %s", actualStatus)
			return []string{"false"}, ErrCommandResponse{errMsg}
		}

		return []string{"true"}, nil
	}

	return []string{actualStatus}, nil
}
