// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package llama3

import (
	"time"

	kaitov1beta1 "github.com/kaito-project/kaito/api/v1beta1"
	"github.com/kaito-project/kaito/pkg/model"
	"github.com/kaito-project/kaito/pkg/utils/plugin"
	"github.com/kaito-project/kaito/pkg/workspace/inference"
)

func init() {
	plugin.KaitoModelRegister.Register(&plugin.Registration{
		Name:     "llama-3.3-70b-instruct",
		Instance: &llama3A,
	})
}

var (
	llama3RunParams = map[string]string{
		"model": "/workspace/weights",
	}
)

var llama3A llama3

type llama3 struct{}

func (*llama3) GetInferenceParameters() *model.PresetParam {
	return &model.PresetParam{
		ModelFamilyName:           "LLaMa3",
		ImageAccessMode:           string(kaitov1beta1.ModelImageAccessModeDownload),
		DiskStorageRequirement:    "152Gi",
		GPUCountRequirement:       "2",
		TotalGPUMemoryRequirement: "152Gi",
		PerGPUMemoryRequirement:   "40Gi",
		RuntimeParam: model.RuntimeParam{
			VLLM: model.VLLMParam{
				BaseCommand:    inference.DefaultVLLMCommand,
				ModelName:      "llama-3.3-70b-instruct",
				ModelRunParams: llama3RunParams,
			},
		},
		ReadinessTimeout: time.Duration(30) * time.Minute,
		WorldSize:        2,
	}
}

func (*llama3) GetTuningParameters() *model.PresetParam {
	return nil // Currently doesn't support fine-tuning
}
func (*llama3) GetDownloadParameters() *model.DownloadParam {
	return &model.DownloadParam{
		RepoId:          "meta-llama/Llama-3.3-70B-Instruct",
		Timeout:         time.Duration(30 * time.Minute),
		PVCBoundTimeout: time.Duration(5 * time.Minute),
	}
}
func (*llama3) SupportDistributedInference() bool {
	return false
}
func (*llama3) SupportTuning() bool {
	return false
}
func (*llama3) SupportDownload() bool {
	return true
}
