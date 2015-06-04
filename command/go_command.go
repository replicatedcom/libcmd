package command

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

	"github.com/awslabs/aws-sdk-go/aws"
	"github.com/awslabs/aws-sdk-go/aws/credentials"
	"github.com/awslabs/aws-sdk-go/service/ec2"
	"github.com/awslabs/aws-sdk-go/service/s3"
	"github.com/awslabs/aws-sdk-go/service/sqs"
	"github.com/fsouza/go-dockerclient"
)

type goCommandFunc func(c *goCmd, args ...string) ([]string, error)

const (
	AWSServiceEC2 = "ec2"
	AWSServiceS3  = "s3"
	AWSServiceSQS = "sqs"
)

var (
	randCharset = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ_-0123456789")

	goCommands map[string]goCommandFunc = map[string]goCommandFunc{
		"cert":             certCommand,
		"random":           randomCommand,
		"echo":             echoCommand,
		"publicip":         publicIpCommand,
		"github_app_auth":  githubAppAuthCommand,
		"aws_auth":         awsAuthCommand,
		"resolve_host":     resolveHostCommand,
		"tcp_port_accept":  tcpPortAccept,
		"http_status_code": httpStatusCode,
	}
)

type goCmd struct {
	fn           goCommandFunc
	config       CmdConfig
	dockerClient *docker.Client
}

func (c *goCmd) Run(args ...string) ([]string, error) {
	return c.fn(c, args...)
}

func NewGoCmd(op string, config CmdConfig, dockerClient *docker.Client) (*goCmd, error) {
	fn, exists := goCommands[op]
	if !exists {
		return nil, ErrCommandNotFound
	}
	return &goCmd{fn, config, dockerClient}, nil
}

func certCommand(c *goCmd, args ...string) ([]string, error) {
	cmd, err := NewContainerCmd("cert", c.config, c.dockerClient)
	if err != nil {
		return nil, err
	}
	result, err := cmd.Run(args...)
	if err != nil {
		return nil, err
	}
	results := strings.SplitAfter(result[0], "-----END RSA PRIVATE KEY-----")
	for i, result := range results {
		result := strings.TrimSpace(result)
		results[i] = base64.StdEncoding.EncodeToString([]byte(result))
	}
	return results, nil
}

func randomCommand(c *goCmd, args ...string) ([]string, error) {
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

func echoCommand(c *goCmd, args ...string) ([]string, error) {
	result := strings.Join(args, " ")
	return []string{result}, nil
}

func publicIpCommand(c *goCmd, args ...string) ([]string, error) {
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

func githubAppAuthCommand(c *goCmd, args ...string) ([]string, error) {
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

func awsAuthCommand(c *goCmd, args ...string) ([]string, error) {
	// Should be:
	// 0: aws_access_key_id
	// 1: aws_secret_access_key
	// 2: aws_service
	if len(args) < 3 {
		return nil, fmt.Errorf("Missing required args")
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
		return []string{"false", "Access denied."}, nil
	} else if err != nil {
		return nil, err
	}

	return []string{"true", "Access granted."}, nil
}

func resolveHostCommand(c *goCmd, args ...string) ([]string, error) {
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

func tcpPortAccept(c *goCmd, args ...string) ([]string, error) {
	_, err := net.Dial("tcp", fmt.Sprintf("%s:%s", args[0], args[1]))
	if err != nil {
		return []string{strconv.FormatBool(false)}, nil
	}

	return []string{strconv.FormatBool(true)}, nil
}

func httpStatusCode(c *goCmd, args ...string) ([]string, error) {
	req, err := http.NewRequest("GET", args[0], nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	expectedResponse, err := strconv.Atoi(args[1])
	if err != nil {
		return nil, err
	}

	result := expectedResponse == resp.StatusCode

	return []string{strconv.FormatBool(result)}, nil
}
