package main

import (
    "github.com/aws/aws-sdk-go/aws"
    "github.com/aws/aws-sdk-go/aws/session"
    "github.com/aws/aws-sdk-go/service/ec2"
	"encoding/base64"
    "fmt"
    "log"
	"time"
	"flag"
)

func main() {
    // add support to specify RAY_VERSION and RAY_COMMIT and PYTHON_VERSION
    RAY_VERSION := flag.String("ray_version", "", "The version of Ray to install")
    RAY_COMMIT := flag.String("ray_commit", "", "The commit of Ray to install")
    PYTHON_VERSION := flag.String("python_version", "", "The version of Python to install")
    ENDPOINT := flag.String("endpoint", "", "The endpoint to send the status to")
    flag.Parse()
    if *RAY_VERSION == "" || *RAY_COMMIT == "" || *PYTHON_VERSION == "" || *ENDPOINT == "" {
        log.Fatal("All flags are required")
    }

    sess, err := session.NewSession(&aws.Config{
        Region: aws.String("us-west-2")},
    )
	userDataScript := fmt.Sprintf(`#!/bin/bash
    mkdir -p ~/miniconda3
    wget https://repo.anaconda.com/miniconda/Miniconda3-latest-Linux-x86_64.sh -O ~/miniconda3/miniconda.sh
    bash ~/miniconda3/miniconda.sh -b -u -p ~/miniconda3
    rm ~/miniconda3/miniconda.sh
    source ~/miniconda3/bin/activate
	git clone https://github.com/ray-project/ray.git
    cd ray
    export RAY_VERSION="%s"
    export RAY_COMMIT="%s"
    export PYTHON_VERSION="%s"
    conda create -n ray python=${PYTHON_VERSION} -y
    conda activate ray
    pip install \
    --index-url https://test.pypi.org/simple/ \
    --extra-index-url https://pypi.org/simple \
    "ray[cpp]==$RAY_VERSION"

    cd release/util
    python sanity_check.py --ray_version="${RAY_VERSION}" --ray_commit="${RAY_COMMIT}" > sanity_check.log

    curl -X POST -d @sanity_check.log "%s"
`, *RAY_VERSION, *RAY_COMMIT, *PYTHON_VERSION, *ENDPOINT)
    encodedScript := base64.StdEncoding.EncodeToString([]byte(userDataScript))

    svc := ec2.New(sess)
    runResult, err := svc.RunInstances(&ec2.RunInstancesInput{
        ImageId:      aws.String("ami-04dd23e62ed049936"), // ubuntu 24.04
        InstanceType: aws.String("t3.xlarge"),
        MinCount:     aws.Int64(1),
        MaxCount:     aws.Int64(1),
		UserData:     aws.String(encodedScript),
		TagSpecifications: []*ec2.TagSpecification{
			{
				ResourceType: aws.String("instance"),
				Tags: []*ec2.Tag{
					{Key: aws.String("Name"), Value: aws.String("Kevin-wheel-verification")},
				},
			},
		},
    })

    if err != nil {
        fmt.Println("Could not create instance", err)
        return
    }
	instanceID := *runResult.Instances[0].InstanceId
    fmt.Println("Created instance", instanceID)
}
