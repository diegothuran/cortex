/*
Copyright 2019 Cortex Labs, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/cortexlabs/cortex/pkg/consts"
	"github.com/cortexlabs/cortex/pkg/lib/aws"
	"github.com/cortexlabs/cortex/pkg/lib/clusterconfig"
	cr "github.com/cortexlabs/cortex/pkg/lib/configreader"
	"github.com/cortexlabs/cortex/pkg/lib/errors"
	"github.com/cortexlabs/cortex/pkg/lib/exit"
	"github.com/cortexlabs/cortex/pkg/lib/hash"
	"github.com/cortexlabs/cortex/pkg/lib/k8s"
	"github.com/cortexlabs/cortex/pkg/lib/telemetry"
)

const _cluster_config_path = "/configs/cluster/cluster.yaml"

var (
	Cluster *clusterconfig.InternalConfig
	AWS     *aws.Client
	K8s     *k8s.Client
)

func Init() error {
	var err error

	Cluster = &clusterconfig.InternalConfig{
		APIVersion:        consts.CortexVersion,
		OperatorInCluster: strings.ToLower(os.Getenv("CORTEX_OPERATOR_IN_CLUSTER")) != "false",
	}

	clusterConfigPath := os.Getenv("CORTEX_CLUSTER_CONFIG_PATH")
	if clusterConfigPath == "" {
		clusterConfigPath = _cluster_config_path
	}

	errs := cr.ParseYAMLFile(Cluster, clusterconfig.Validation, clusterConfigPath)
	if errors.HasError(errs) {
		return errors.FirstError(errs...)
	}

	AWS, err = aws.New(*Cluster.Region, *Cluster.Bucket, true)
	if err != nil {
		exit.Error(err)
	}

	Cluster.ID = hash.String(Cluster.ClusterName + *Cluster.Region + AWS.HashedAccountID)

	err = telemetry.Init(telemetry.Config{
		Enabled:     Cluster.Telemetry,
		UserID:      AWS.HashedAccountID,
		Properties:  map[string]interface{}{"clusterID": Cluster.ID},
		Environment: "operator",
		LogErrors:   true,
		BackoffMode: telemetry.BackoffAnyMessages,
	})
	if err != nil {
		fmt.Println(err.Error())
	}

	Cluster.InstanceMetadata = aws.InstanceMetadatas[*Cluster.Region][*Cluster.InstanceType]

	if K8s, err = k8s.New("default", Cluster.OperatorInCluster); err != nil {
		return err
	}

	return nil
}
