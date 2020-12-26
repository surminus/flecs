package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/aws/aws-sdk-go/service/sts"
)

// BuildStep represents the docker step, that builds and pushes Docker
// images. For simplicity we're just going to wrap Docker commands so it
// just needs to be installed.
type BuildStep struct {
	Build string `yaml:"build"`
}

// Build specifies details of the image to build
type Build struct {
	Dockerfile string     `yaml:"dockerfile"`
	Repository string     `yaml:"repository"`
	Path       string     `yaml:"path"`
	Args       DockerArgs `yaml:"args"`
}

// DockerArgs is a defined list of different arguments to pass to Docker
type DockerArgs struct{}

// Run runs the Build step stage
func (b BuildStep) Run(c Client, cfg Config) (err error) {
	clients, err := c.InitClients()
	if err != nil {
		return err
	}

	build, ok := cfg.Builds[b.Build]
	if !ok {
		return fmt.Errorf("cannot find build configured called %s", b.Build)
	}

	repository := build.Repository
	if repository == "" {
		repository = cfg.ProjectName
	}

	gci, err := clients.STS.GetCallerIdentity(&sts.GetCallerIdentityInput{})
	if err != nil {
		return err
	}

	registryURI := fmt.Sprintf("%s.dkr.ecr.%s.amazonaws.com", aws.StringValue(gci.Account), cfg.Options.ECRRegion)
	imageName := fmt.Sprintf("%s/%s", registryURI, repository)
	imageNameWithTag := fmt.Sprintf("%s:%s", imageName, cfg.Tag)

	// Build image
	err = buildImage(imageNameWithTag, build.Dockerfile, build.Path)
	if err != nil {
		return err
	}

	// Check ECR repository exists and create if it doesn't exist
	arn, err := b.createRepository(clients, repository)
	if err != nil {
		return err
	}
	Log.Infof("Using repository: %s", arn)

	// Authenticate with Docker
	Log.Info("Authenticating...")
	err = b.loginToECR(clients, registryURI)
	if err != nil {
		return err
	}

	// Push image to ECR
	err = pushImage(imageNameWithTag)
	if err != nil {
		return err
	}

	return err
}

func buildImage(imageName, dockerfile, path string) (err error) {
	buildArgs := []string{
		"build",
		"--tag",
		imageName,
	}

	if path == "" {
		path = "."
	}

	if dockerfile != "" {
		buildArgs = append(buildArgs, []string{"--file", dockerfile}...)
	}

	buildArgs = append(buildArgs, path)

	err = runDockerCommand(buildArgs)
	return err
}

func pushImage(imageName string) (err error) {
	pushArgs := []string{
		"push",
		imageName,
	}

	err = runDockerCommand(pushArgs)
	return err
}

func (b BuildStep) loginToECR(clients Clients, registry string) (err error) {
	result, err := clients.ECR.GetAuthorizationToken(&ecr.GetAuthorizationTokenInput{})
	if err != nil {
		return err
	}

	if len(result.AuthorizationData) < 1 {
		return fmt.Errorf("Unable to get authorization data")
	}

	base64token := aws.StringValue(result.AuthorizationData[0].AuthorizationToken)
	decodedToken, err := base64.StdEncoding.DecodeString(base64token)
	if err != nil {
		return err
	}

	token := strings.Split(string(decodedToken), ":")[1]

	args := []string{
		"login",
		"--username",
		"AWS",
		"--password-stdin",
		registry,
	}

	path, err := exec.LookPath("docker")
	if err != nil {
		return err
	}

	command := exec.Cmd{
		Path:   path,
		Args:   append([]string{path}, args...),
		Stderr: os.Stderr,
	}

	stdin, err := command.StdinPipe()
	if err != nil {
		return err
	}

	go func() {
		defer stdin.Close()
		_, err = io.WriteString(stdin, token)
		if err != nil {
			return
		}
	}()

	err = command.Run()
	return err
}

func runDockerCommand(args []string) (err error) {
	path, err := exec.LookPath("docker")
	if err != nil {
		return err
	}

	command := exec.Cmd{
		Path:   path,
		Args:   append([]string{path}, args...),
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}

	err = command.Run()
	if err != nil {
		return err
	}

	return err
}

func (b BuildStep) createRepository(clients Clients, name string) (arn string, err error) {
	arn, err = b.getRepositoryARN(clients, name)
	if err != nil {
		return arn, err
	}

	if arn == "" {
		Log.Infof("Cannot find repository. Creating repository %s", name)

		input := ecr.CreateRepositoryInput{
			RepositoryName: aws.String(name),
		}

		create, err := clients.ECR.CreateRepository(&input)
		if err != nil {
			return arn, err
		}

		arn = aws.StringValue(create.Repository.RepositoryArn)

		// Wait for a bit to ensure it's ready
		time.Sleep(10 * time.Second)
	}

	return arn, err
}

func (b BuildStep) getRepositoryARN(clients Clients, name string) (arn string, err error) {
	resp, err := clients.ECR.DescribeRepositories(&ecr.DescribeRepositoriesInput{
		RepositoryNames: aws.StringSlice([]string{name}),
	})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			if aerr.Code() != ecr.ErrCodeRepositoryNotFoundException {
				return arn, nil
			}
		} else {
			return arn, err
		}
	}

	if len(resp.Repositories) < 1 {
		return arn, fmt.Errorf("cannot find repository %s", name)
	}

	arn = aws.StringValue(resp.Repositories[0].RepositoryArn)
	return arn, err
}
